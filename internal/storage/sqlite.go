package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/fless-lab/TextDock/internal/message"

	_ "modernc.org/sqlite"
)

type SQLite struct{ db *sql.DB }

func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// One connection keeps PRAGMAs and in-memory databases consistent. The
	// local workload doesn't need a pool; WAL permits external read tooling.
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, recipient TEXT NOT NULL, run_id TEXT NOT NULL,
			body TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS messages_created ON messages(created_at DESC);
		CREATE INDEX IF NOT EXISTS messages_run ON messages(run_id, recipient, created_at DESC);
		INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return &SQLite{db: db}, nil
}

// Fixed-width UTC timestamps preserve chronological ordering in SQLite TEXT.
const timestamp = "2006-01-02T15:04:05.000000000Z"

func (s *SQLite) Save(ctx context.Context, m message.Message) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO messages VALUES (?, ?, ?, ?, ?, ?)`,
		m.ID, m.To, m.RunID, m.Body, m.CreatedAt.UTC().Format(timestamp), string(data))
	return err
}

func (s *SQLite) List(ctx context.Context, f message.Filter) ([]message.Message, error) {
	if f.Limit < 1 || f.Limit > 200 {
		f.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM messages
		WHERE (? = '' OR instr(lower(body), lower(?)) > 0 OR instr(recipient, ?) > 0)
		AND (? = '' OR recipient = ?) AND (? = '' OR run_id = ?)
		AND created_at >= ? ORDER BY created_at DESC, id DESC LIMIT ?`,
		f.Query, f.Query, f.Query, f.To, f.To, f.RunID, f.RunID,
		f.Since.UTC().Format(timestamp), f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]message.Message, 0)
	for rows.Next() {
		var data string
		var m message.Message
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) Delete(ctx context.Context, id string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

func (s *SQLite) Close() error { return s.db.Close() }

var _ message.Repository = (*SQLite)(nil)
