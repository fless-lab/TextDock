package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/fless-lab/TextDock/internal/audit"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
)

func (s *SQLite) migrateWorkspace() error {
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
	if version == 2 {
		_, err := tx.Exec(`CREATE TABLE projects(id TEXT PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE inboxes(id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id), name TEXT NOT NULL);
		INSERT INTO projects VALUES ('default', 'Local project');
		INSERT INTO inboxes VALUES ('local', 'default', 'Inbox');
		ALTER TABLE messages ADD COLUMN inbox TEXT NOT NULL DEFAULT 'local' REFERENCES inboxes(id);
		ALTER TABLE messages ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE pairings ADD COLUMN inbox TEXT NOT NULL DEFAULT 'local';
		ALTER TABLE devices ADD COLUMN inbox TEXT NOT NULL DEFAULT 'local';
		UPDATE messages SET payload = json_set(payload, '$.inbox', 'local', '$.favorite', json('false'), '$.tags', json('[]'));
		CREATE INDEX messages_inbox_created ON messages(inbox, created_at DESC, id DESC);
		CREATE INDEX messages_inbox_run ON messages(inbox, run_id, recipient, created_at DESC, id DESC);
		INSERT INTO schema_migrations VALUES (3);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) ListProjects(ctx context.Context) ([]workspace.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM projects ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]workspace.Project, 0)
	for rows.Next() {
		var p workspace.Project
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *SQLite) ListInboxes(ctx context.Context) ([]workspace.Inbox, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name FROM inboxes ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]workspace.Inbox, 0)
	for rows.Next() {
		var p workspace.Inbox
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *SQLite) CreateProject(ctx context.Context, p workspace.Project, in workspace.Inbox) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects VALUES (?, ?)`, p.ID, p.Name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO inboxes VALUES (?, ?, ?)`, in.ID, p.ID, in.Name); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, audit.Event{Action: "project.created", ProjectID: p.ID, ResourceType: "project", ResourceID: p.ID, Count: 1}); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, audit.Event{Action: "inbox.created", InboxID: in.ID, ResourceType: "inbox", ResourceID: in.ID, Count: 1}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) CreateInbox(ctx context.Context, in workspace.Inbox) error {
	_, err := s.auditedExec(ctx, audit.Event{Action: "inbox.created", InboxID: in.ID, ResourceType: "inbox", ResourceID: in.ID}, `INSERT INTO inboxes VALUES (?, ?, ?)`, in.ID, in.ProjectID, in.Name)
	return err
}
func (s *SQLite) InboxExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM inboxes WHERE id = ?)`, id).Scan(&exists)
	return exists, err
}

func (s *SQLite) Update(ctx context.Context, id string, patch message.Update) (message.Message, error) {
	var m message.Message
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	var data string
	if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id = ?`, id).Scan(&data); err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return m, err
	}
	if patch.Favorite != nil {
		m.Favorite = *patch.Favorite
	}
	if patch.Tags != nil {
		m.Tags = *patch.Tags
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE messages SET payload = ?, favorite = ? WHERE id = ?`, string(payload), m.Favorite, id); err != nil {
		return m, err
	}
	if err := insertAudit(ctx, tx, audit.Event{Action: "message.updated", InboxID: m.Inbox, ResourceType: "message", ResourceID: m.ID, Count: 1}); err != nil {
		return m, err
	}
	return m, tx.Commit()
}

func (s *SQLite) Purge(ctx context.Context, inbox string) (int64, error) {
	return s.auditedExec(ctx, audit.Event{Action: "messages.purged", InboxID: inbox, ResourceType: "inbox", ResourceID: inbox}, `DELETE FROM messages WHERE inbox=?`, inbox)
}

func (s *SQLite) Prune(ctx context.Context, before time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT inbox,count(*) FROM messages WHERE created_at<? GROUP BY inbox`, before.UTC().Format(timestamp))
	if err != nil {
		return 0, err
	}
	events := []audit.Event{}
	var count int64
	for rows.Next() {
		e := audit.Event{Action: "messages.pruned", ResourceType: "inbox"}
		if err := rows.Scan(&e.InboxID, &e.Count); err != nil {
			rows.Close()
			return 0, err
		}
		e.ResourceID = e.InboxID
		count += e.Count
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE created_at<?`, before.UTC().Format(timestamp)); err != nil {
		return 0, err
	}
	for _, e := range events {
		if err := insertAudit(ctx, tx, e); err != nil {
			return 0, err
		}
	}
	return count, tx.Commit()
}

func (s *SQLite) Backup(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}

var _ workspace.Repository = (*SQLite)(nil)

func ValidateSnapshot(ctx context.Context, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	u := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return err
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("snapshot integrity: %s", integrity)
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version < 1 || version > schemaVersion {
		return errors.New("snapshot schema is not supported by this version")
	}
	return nil
}
