package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sync/atomic"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/simulation"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db          *sql.DB
	keeper      *sql.DB
	pushEnabled atomic.Bool
}

const schemaVersion = 10

func Open(path string) (*SQLite, error) {
	// database/sql can discard a connection after a cancelled transaction.
	// Keep a named memory database alive independently of the request pool.
	parameters := url.Values{"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}}
	var target url.URL
	target.Scheme = "file"
	if path == ":memory:" {
		target.Opaque = "textdock-" + rand.Text()
		parameters.Set("mode", "memory")
		parameters.Set("cache", "shared")
	} else {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		target.Path = absolute
	}
	target.RawQuery = parameters.Encode()
	db, err := sql.Open("sqlite", target.String())
	if err != nil {
		return nil, err
	}
	// One connection keeps PRAGMAs and in-memory databases consistent. The
	// local workload doesn't need a pool; WAL permits external read tooling.
	db.SetMaxOpenConns(1)
	var keeper *sql.DB
	if path == ":memory:" {
		keeper, err = sql.Open("sqlite", target.String())
		if err != nil {
			db.Close()
			return nil, err
		}
		keeper.SetMaxOpenConns(1)
		if err = keeper.Ping(); err != nil {
			keeper.Close()
			db.Close()
			return nil, err
		}
	}
	cleanup := func() {
		db.Close()
		if keeper != nil {
			keeper.Close()
		}
	}
	_, err = db.Exec(`PRAGMA foreign_keys=OFF; PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY, recipient TEXT NOT NULL, run_id TEXT NOT NULL,
			body TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS messages_created ON messages(created_at DESC);
		CREATE INDEX IF NOT EXISTS messages_run ON messages(run_id, recipient, created_at DESC);
		INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);`)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	s := &SQLite{db: db, keeper: keeper}
	if err := s.migrateConnect(); err != nil {
		cleanup()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	if err := s.migrateWorkspace(); err != nil {
		cleanup()
		return nil, fmt.Errorf("migrate workspaces: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateSimulation(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateRelay(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateDeviceLab(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateGatewayHealth(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migratePush(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateKeys(); err != nil {
		cleanup()
		return nil, err
	}
	if err := s.migrateUsers(); err != nil {
		cleanup()
		return nil, err
	}
	return s, nil
}

// Fixed-width UTC timestamps preserve chronological ordering in SQLite TEXT.
const timestamp = "2006-01-02T15:04:05.000000000Z"

func (s *SQLite) Save(ctx context.Context, m message.Message) error {
	return s.Schedule(ctx, m, []simulation.Job{})
}

func (s *SQLite) List(ctx context.Context, f message.Filter) ([]message.Message, error) {
	if f.Limit < 1 || f.Limit > 201 {
		f.Limit = 100
	}
	if f.Inbox == "" {
		f.Inbox = "local"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM messages
		WHERE inbox = ? AND (? = '' OR instr(lower(body), lower(?)) > 0 OR instr(recipient, ?) > 0)
		AND (? = '' OR recipient = ?) AND (? = '' OR run_id = ?)
		AND created_at >= ? AND (? = '' OR created_at < ? OR (created_at = ? AND id < ?))
		AND (? = 0 OR favorite = 1) AND (? = 0 OR json_extract(payload, '$.analysis.otp') IS NOT NULL)
		AND (? = '' OR EXISTS(SELECT 1 FROM json_each(payload, '$.tags') WHERE value = ?))
		AND (? = '' OR json_extract(payload, '$.status') = ?)
		ORDER BY created_at DESC, id DESC LIMIT ?`,
		f.Inbox, f.Query, f.Query, f.Query, f.To, f.To, f.RunID, f.RunID,
		f.Since.UTC().Format(timestamp), f.BeforeID, f.Before.UTC().Format(timestamp), f.Before.UTC().Format(timestamp), f.BeforeID,
		f.Favorite, f.OTPOnly, f.Tag, f.Tag, f.Status, f.Status, f.Limit)
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

func (s *SQLite) Get(ctx context.Context, id string) (message.Message, error) {
	var data string
	var m message.Message
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id = ?`, id).Scan(&data)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal([]byte(data), &m)
	return m, err
}

func (s *SQLite) Close() error {
	err := s.db.Close()
	if s.keeper != nil {
		err = errors.Join(err, s.keeper.Close())
	}
	return err
}

var _ message.Repository = (*SQLite)(nil)
