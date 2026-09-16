package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/fless-lab/TextDock/internal/access"
	"github.com/fless-lab/TextDock/internal/audit"
	"time"
)

func (s *SQLite) migrateKeys() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, project_id TEXT NOT NULL REFERENCES projects(id),
 inbox_id TEXT REFERENCES inboxes(id), permissions TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
 created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, revoked_at INTEGER);
 INSERT OR IGNORE INTO schema_migrations VALUES(9);`)
	return err
}
func (s *SQLite) CreateAPIKey(ctx context.Context, k access.Key, hash string) error {
	if err := k.Validate(time.Now()); err != nil {
		return err
	}
	permissions, err := json.Marshal(k.Permissions)
	if err != nil {
		return err
	}
	n, err := s.auditedExec(ctx, audit.Event{Action: "api_key.created", ProjectID: k.ProjectID, InboxID: k.InboxID, ResourceType: "api_key", ResourceID: k.ID, Permissions: k.Permissions}, `INSERT INTO api_keys(id,name,project_id,inbox_id,permissions,token_hash,created_at,expires_at)
 SELECT ?,?,?,nullif(?,''),?,?,?,? WHERE EXISTS(SELECT 1 FROM projects WHERE id=?)
 AND (?='' OR EXISTS(SELECT 1 FROM inboxes WHERE id=? AND project_id=?))`, k.ID, k.Name, k.ProjectID, k.InboxID, string(permissions), hash, k.CreatedAt.UnixMilli(), k.ExpiresAt.UnixMilli(), k.ProjectID, k.InboxID, k.InboxID, k.ProjectID)
	if err != nil {
		return err
	}
	if err == nil && n == 0 {
		return access.ErrScope
	}
	return err
}

const keyColumns = `id,name,project_id,coalesce(inbox_id,''),permissions,created_at,expires_at,revoked_at`

func scanKey(row interface{ Scan(...any) error }) (access.Key, error) {
	var k access.Key
	var p string
	var created, expires int64
	var revoked sql.NullInt64
	err := row.Scan(&k.ID, &k.Name, &k.ProjectID, &k.InboxID, &p, &created, &expires, &revoked)
	if err != nil {
		return k, err
	}
	k.CreatedAt = time.UnixMilli(created).UTC()
	k.ExpiresAt = time.UnixMilli(expires).UTC()
	if revoked.Valid {
		v := time.UnixMilli(revoked.Int64).UTC()
		k.RevokedAt = &v
	}
	return k, json.Unmarshal([]byte(p), &k.Permissions)
}
func (s *SQLite) ListAPIKeys(ctx context.Context) ([]access.Key, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+keyColumns+` FROM api_keys ORDER BY created_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []access.Key{}
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *SQLite) APIKeyByHash(ctx context.Context, hash string) (access.Key, error) {
	k, err := scanKey(s.db.QueryRowContext(ctx, `SELECT `+keyColumns+` FROM api_keys WHERE token_hash=? AND revoked_at IS NULL AND expires_at>?`, hash, time.Now().UnixMilli()))
	if errors.Is(err, sql.ErrNoRows) {
		err = access.ErrInvalid
	}
	return k, err
}
func (s *SQLite) RevokeAPIKey(ctx context.Context, id string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	k, err := scanKey(tx.QueryRowContext(ctx, `SELECT `+keyColumns+` FROM api_keys WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if k.RevokedAt != nil {
		return true, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET revoked_at=? WHERE id=?`, time.Now().UnixMilli(), id); err != nil {
		return false, err
	}
	if err := insertAudit(ctx, tx, audit.Event{Action: "api_key.revoked", ProjectID: k.ProjectID, InboxID: k.InboxID, ResourceType: "api_key", ResourceID: k.ID, Count: 1}); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (s *SQLite) KeyInboxAllowed(ctx context.Context, k access.Key, inbox string) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inboxes WHERE id=? AND project_id=? AND (?='' OR id=?))`, inbox, k.ProjectID, k.InboxID, k.InboxID).Scan(&ok)
	return ok, err
}
