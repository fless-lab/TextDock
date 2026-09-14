package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
)

func (s *SQLite) migrateConnect() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version > schemaVersion {
		return errors.New("database schema is newer than this TextDock version")
	}
	if version == 1 {
		_, err = tx.Exec(`CREATE TABLE pairings (
			code_hash TEXT PRIMARY KEY, recipient TEXT NOT NULL, run_id TEXT NOT NULL, expires_at INTEGER NOT NULL
		);
		CREATE TABLE devices (
			id TEXT PRIMARY KEY, token_hash TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
			recipient TEXT NOT NULL, run_id TEXT NOT NULL, created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO schema_migrations(version) VALUES (2);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) CreatePair(ctx context.Context, hash string, scope connect.Scope, expires time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM pairings WHERE expires_at <= ?`, time.Now().UnixMilli()); err != nil {
		return err
	}
	if scope.Inbox == "" {
		scope.Inbox = "local"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO pairings(code_hash, recipient, run_id, expires_at, inbox) VALUES (?, ?, ?, ?, ?)`, hash, scope.To, scope.RunID, expires.UnixMilli(), scope.Inbox); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) ClaimPair(ctx context.Context, codeHash, tokenHash string, d connect.Device) (connect.Device, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return d, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `DELETE FROM pairings WHERE code_hash = ? AND expires_at > ? RETURNING recipient, run_id, inbox`, codeHash, time.Now().UnixMilli()).Scan(&d.Scope.To, &d.Scope.RunID, &d.Scope.Inbox)
	if errors.Is(err, sql.ErrNoRows) {
		return d, connect.ErrInvalid
	}
	if err != nil {
		return d, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO devices(id, token_hash, name, recipient, run_id, created_at, expires_at, inbox) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, tokenHash, d.Name, d.Scope.To, d.Scope.RunID, d.CreatedAt.UnixMilli(), d.ExpiresAt.UnixMilli(), d.Scope.Inbox)
	if err != nil {
		return d, err
	}
	return d, tx.Commit()
}

func scanDevice(row interface{ Scan(...any) error }) (connect.Device, error) {
	var d connect.Device
	var created, expires int64
	err := row.Scan(&d.ID, &d.Name, &d.Scope.To, &d.Scope.RunID, &created, &expires, &d.Revoked, &d.Scope.Inbox)
	d.CreatedAt, d.ExpiresAt = time.UnixMilli(created).UTC(), time.UnixMilli(expires).UTC()
	return d, err
}

const deviceColumns = `id, name, recipient, run_id, created_at, expires_at, revoked, inbox`

func (s *SQLite) DeviceByHash(ctx context.Context, hash string) (connect.Device, error) {
	d, err := scanDevice(s.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM devices WHERE token_hash = ? AND revoked = 0 AND expires_at > ?`, hash, time.Now().UnixMilli()))
	if errors.Is(err, sql.ErrNoRows) {
		err = connect.ErrInvalid
	}
	return d, err
}

func (s *SQLite) ListDevices(ctx context.Context) ([]connect.Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+deviceColumns+` FROM devices ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := make([]connect.Device, 0)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *SQLite) RevokeDevice(ctx context.Context, id string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE devices SET revoked = 1 WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

var _ connect.Repository = (*SQLite)(nil)
