package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/fless-lab/TextDock/internal/access"
	"time"
)

func (s *SQLite) migrateUsers() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,username TEXT NOT NULL UNIQUE,name TEXT NOT NULL,password_hash TEXT NOT NULL,disabled INTEGER NOT NULL DEFAULT 0,created_at INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS memberships(project_id TEXT NOT NULL REFERENCES projects(id),user_id TEXT NOT NULL REFERENCES users(id),role TEXT NOT NULL CHECK(role IN ('viewer','member','admin')),PRIMARY KEY(project_id,user_id));
 CREATE TABLE IF NOT EXISTS user_sessions(id TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES users(id),token_hash TEXT NOT NULL UNIQUE,created_at INTEGER NOT NULL,expires_at INTEGER NOT NULL,revoked_at INTEGER);
 CREATE INDEX IF NOT EXISTS user_sessions_user ON user_sessions(user_id);
 INSERT OR IGNORE INTO schema_migrations VALUES(10);`)
	return err
}
func (s *SQLite) CreateUser(ctx context.Context, u access.User, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO users(id,username,name,password_hash,created_at) VALUES(?,?,?,?,?) ON CONFLICT(username) DO NOTHING`, u.ID, u.Username, u.Name, passwordHash, u.CreatedAt.UnixMilli())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return access.ErrUsername
	}
	return err
}
func (s *SQLite) ListUsers(ctx context.Context) ([]access.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,username,name,disabled,created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []access.User{}
	for rows.Next() {
		var u access.User
		var at int64
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Disabled, &at); err != nil {
			return nil, err
		}
		u.CreatedAt = time.UnixMilli(at).UTC()
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *SQLite) UserCredential(ctx context.Context, username string) (access.User, string, error) {
	var u access.User
	var hash string
	var at int64
	err := s.db.QueryRowContext(ctx, `SELECT id,username,name,disabled,created_at,password_hash FROM users WHERE username=?`, username).Scan(&u.ID, &u.Username, &u.Name, &u.Disabled, &at, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		err = access.ErrLogin
	}
	u.CreatedAt = time.UnixMilli(at).UTC()
	return u, hash, err
}
func (s *SQLite) CreateUserSession(ctx context.Context, session access.UserSession, hash, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO user_sessions(id,user_id,token_hash,created_at,expires_at) SELECT ?,?,?,?,? WHERE EXISTS(SELECT 1 FROM users WHERE id=? AND password_hash=? AND disabled=0)`, session.ID, session.User.ID, hash, session.CreatedAt.UnixMilli(), session.ExpiresAt.UnixMilli(), session.User.ID, passwordHash)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return access.ErrLogin
	}
	return err
}
func (s *SQLite) UserSessionByHash(ctx context.Context, hash string) (access.UserSession, error) {
	var out access.UserSession
	var created, expires, userCreated int64
	err := s.db.QueryRowContext(ctx, `SELECT s.id,s.created_at,s.expires_at,u.id,u.username,u.name,u.created_at FROM user_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.revoked_at IS NULL AND s.expires_at>? AND u.disabled=0`, hash, time.Now().UnixMilli()).Scan(&out.ID, &created, &expires, &out.User.ID, &out.User.Username, &out.User.Name, &userCreated)
	if errors.Is(err, sql.ErrNoRows) {
		return out, access.ErrSession
	}
	if err != nil {
		return out, err
	}
	out.CreatedAt = time.UnixMilli(created).UTC()
	out.ExpiresAt = time.UnixMilli(expires).UTC()
	out.User.CreatedAt = time.UnixMilli(userCreated).UTC()
	rows, err := s.db.QueryContext(ctx, `SELECT project_id,role FROM memberships WHERE user_id=? ORDER BY project_id`, out.User.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Memberships = []access.Membership{}
	for rows.Next() {
		m := access.Membership{UserID: out.User.ID, Username: out.User.Username, Name: out.User.Name}
		if err := rows.Scan(&m.ProjectID, &m.Role); err != nil {
			return out, err
		}
		out.Memberships = append(out.Memberships, m)
	}
	return out, rows.Err()
}
func (s *SQLite) UserSessions(ctx context.Context, user string) ([]access.SessionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,created_at,expires_at FROM user_sessions WHERE user_id=? AND revoked_at IS NULL AND expires_at>? ORDER BY created_at DESC`, user, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []access.SessionSummary{}
	for rows.Next() {
		var v access.SessionSummary
		var a, b int64
		if err := rows.Scan(&v.ID, &a, &b); err != nil {
			return nil, err
		}
		v.CreatedAt = time.UnixMilli(a).UTC()
		v.ExpiresAt = time.UnixMilli(b).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *SQLite) RevokeUserSession(ctx context.Context, user, id string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked_at=coalesce(revoked_at,?) WHERE id=? AND user_id=?`, time.Now().UnixMilli(), id, user)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

// Mutations and session revocation commit together. Returned IDs allow the HTTP
// process to close live streams immediately after commit.
func revokeSessions(ctx context.Context, tx *sql.Tx, user string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM user_sessions WHERE user_id=? AND revoked_at IS NULL`, user)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE user_sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, time.Now().UnixMilli(), user)
	return ids, err
}
func (s *SQLite) changeUser(ctx context.Context, id, statement string, args ...any) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, access.ErrUser
	}
	ids, err := revokeSessions(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}
func (s *SQLite) SetUserDisabled(ctx context.Context, id string, disabled bool) ([]string, error) {
	return s.changeUser(ctx, id, `UPDATE users SET disabled=? WHERE id=?`, disabled, id)
}
func (s *SQLite) SetUserPassword(ctx context.Context, id, hash, expected string) ([]string, error) {
	return s.changeUser(ctx, id, `UPDATE users SET password_hash=? WHERE id=? AND (?='' OR password_hash=?)`, hash, id, expected, expected)
}
func (s *SQLite) ProjectMembers(ctx context.Context, project string) ([]access.Membership, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.project_id,u.id,u.username,u.name,m.role FROM memberships m JOIN users u ON u.id=m.user_id WHERE project_id=? ORDER BY u.username`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []access.Membership{}
	for rows.Next() {
		var m access.Membership
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Username, &m.Name, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *SQLite) SetMembership(ctx context.Context, project, username, role string) ([]string, error) {
	if len(access.RolePermissions(role)) == 0 {
		return nil, access.ErrUser
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE username=? AND disabled=0 AND EXISTS(SELECT 1 FROM projects WHERE id=?)`, username, project).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, access.ErrUser
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memberships VALUES(?,?,?) ON CONFLICT(project_id,user_id) DO UPDATE SET role=excluded.role`, project, id, role)
	if err != nil {
		return nil, err
	}
	ids, err := revokeSessions(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}
func (s *SQLite) RemoveMembership(ctx context.Context, project, user string) ([]string, error) {
	return s.changeUser(ctx, user, `DELETE FROM memberships WHERE project_id=? AND user_id=?`, project, user)
}
