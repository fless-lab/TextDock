package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fless-lab/TextDock/internal/devicelab"
	"github.com/fless-lab/TextDock/internal/message"
)

func (s *SQLite) migrateDeviceLab() error {
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
		return errors.New("database schema is newer than this version")
	}
	if version == 5 {
		_, err := tx.Exec(`CREATE TABLE lab_injections(id TEXT PRIMARY KEY, message_id TEXT UNIQUE NOT NULL REFERENCES messages(id) ON DELETE CASCADE, inbox TEXT NOT NULL, serial TEXT NOT NULL, status TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', output TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, lease_until INTEGER NOT NULL);
		CREATE INDEX lab_history ON lab_injections(inbox,created_at DESC);
		CREATE TABLE lab_keys(inbox TEXT NOT NULL, key TEXT NOT NULL, fingerprint BLOB NOT NULL, message_id TEXT REFERENCES messages(id) ON DELETE SET NULL, PRIMARY KEY(inbox,key));
		INSERT INTO schema_migrations VALUES (6);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

const injectionColumns = `id,message_id,inbox,serial,status,detail,output,created_at,updated_at`

func scanInjection(row interface{ Scan(...any) error }) (devicelab.Injection, error) {
	var item devicelab.Injection
	var created, updated int64
	err := row.Scan(&item.ID, &item.MessageID, &item.Inbox, &item.Serial, &item.Status, &item.Detail, &item.Output, &created, &updated)
	item.CreatedAt, item.UpdatedAt = time.UnixMilli(created).UTC(), time.UnixMilli(updated).UTC()
	return item, err
}
func (s *SQLite) ReserveInjection(ctx context.Context, m message.Message, serial string) (devicelab.Result, error) {
	return s.reserveInjection(ctx, m, serial, true)
}
func (s *SQLite) LookupInjection(ctx context.Context, m message.Message, serial string) (devicelab.Result, error) {
	return s.reserveInjection(ctx, m, serial, false)
}
func (s *SQLite) reserveInjection(ctx context.Context, m message.Message, serial string, create bool) (devicelab.Result, error) {
	result := devicelab.Result{Message: m}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	canonical, _ := json.Marshal([]string{serial, m.To, m.From, m.Body, m.RunID})
	hash := sha256.Sum256(canonical)
	if m.IdempotencyKey != "" {
		var previous []byte
		var id sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT fingerprint,message_id FROM lab_keys WHERE inbox=? AND key=?`, m.Inbox, m.IdempotencyKey).Scan(&previous, &id)
		if err == nil {
			if !id.Valid || string(previous) != string(hash[:]) {
				return result, devicelab.ErrConflict
			}
			var payload string
			if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id=?`, id.String).Scan(&payload); err != nil {
				return result, err
			}
			if err := json.Unmarshal([]byte(payload), &result.Message); err != nil {
				return result, err
			}
			result.Injection, err = scanInjection(tx.QueryRowContext(ctx, `SELECT `+injectionColumns+` FROM lab_injections WHERE message_id=?`, id.String))
			result.Replayed = true
			return result, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
	}
	if !create {
		return result, nil
	}
	if _, err := insertMessage(ctx, tx, m); err != nil {
		return result, err
	}
	now := time.Now().UTC()
	id := "inject_" + rand.Text()
	if _, err := tx.ExecContext(ctx, `INSERT INTO lab_injections(id,message_id,inbox,serial,status,created_at,updated_at,lease_until) VALUES(?,?,?,?,?,?,?,?)`, id, m.ID, m.Inbox, serial, "injecting", now.UnixMilli(), now.UnixMilli(), now.Add(time.Minute).UnixMilli()); err != nil {
		return result, err
	}
	if m.IdempotencyKey != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO lab_keys VALUES(?,?,?,?)`, m.Inbox, m.IdempotencyKey, hash[:], m.ID); err != nil {
			return result, err
		}
	}
	result.Injection = devicelab.Injection{ID: id, MessageID: m.ID, Inbox: m.Inbox, Serial: serial, Status: "injecting", CreatedAt: now, UpdatedAt: now}
	return result, tx.Commit()
}
func (s *SQLite) FinishInjection(ctx context.Context, id, state, detail, output string) (devicelab.Result, error) {
	var result devicelab.Result
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE lab_injections SET status=?,detail=?,output=?,updated_at=? WHERE message_id=? AND status IN ('injecting','unknown')`, state, detail, output, time.Now().UnixMilli(), id); err != nil {
		return result, err
	}
	item, err := scanInjection(tx.QueryRowContext(ctx, `SELECT `+injectionColumns+` FROM lab_injections WHERE message_id=?`, id))
	if err != nil {
		return result, err
	}
	result.Injection = item
	result.Message, err = updateRelayMessage(ctx, tx, id, item.Status, item.Detail)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
func (s *SQLite) InjectionHistory(ctx context.Context, inbox string, limit int) ([]devicelab.Injection, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+injectionColumns+` FROM lab_injections WHERE inbox=? ORDER BY created_at DESC,id DESC LIMIT ?`, inbox, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]devicelab.Injection, 0)
	for rows.Next() {
		item, err := scanInjection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *SQLite) RecoverInjections(ctx context.Context, now time.Time) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT message_id FROM lab_injections WHERE status='injecting' AND lease_until<=?`, now.UnixMilli())
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, id := range ids {
		_, err := s.FinishInjection(ctx, id, "unknown", "Injection was interrupted; automatic reinjection is disabled", "")
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

var _ devicelab.Repository = (*SQLite)(nil)
