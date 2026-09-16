package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/fless-lab/TextDock/internal/audit"
	"time"
)

func (s *SQLite) migrateAudit() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS audit_events(
 id INTEGER PRIMARY KEY AUTOINCREMENT,at TEXT NOT NULL,actor_kind TEXT NOT NULL,actor_id TEXT NOT NULL,
 action TEXT NOT NULL,outcome TEXT NOT NULL,project_id TEXT NOT NULL,inbox_id TEXT NOT NULL,
 resource_type TEXT NOT NULL,resource_id TEXT NOT NULL,count INTEGER NOT NULL,
 role_before TEXT NOT NULL,role_after TEXT NOT NULL,permissions TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS audit_project ON audit_events(project_id,id DESC);
 CREATE INDEX IF NOT EXISTS audit_inbox ON audit_events(inbox_id,id DESC);
 INSERT OR IGNORE INTO schema_migrations VALUES(11);`)
	return err
}

// All mutation callers use the transaction that changes the resource. Audit
// failures roll back that change; no foreign keys allow later SMS deletion to
// erase its history. Inbox/project association is resolved inside the transaction.
func insertAudit(ctx context.Context, tx *sql.Tx, e audit.Event) error {
	e.Actor = audit.ContextActor(ctx)
	if e.Outcome == "" {
		e.Outcome = "committed"
	}
	e.At = time.Now().UTC()
	if err := e.Validate(); err != nil {
		return err
	}
	permissions := e.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	data, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(at,actor_kind,actor_id,action,outcome,project_id,inbox_id,resource_type,resource_id,count,role_before,role_after,permissions)
 VALUES(?,?,?,?,?,CASE WHEN ?<>'' THEN (SELECT project_id FROM inboxes WHERE id=?) ELSE ? END,?,?,?,?,?,?,?)`, e.At.Format(timestamp), e.Actor.Kind, e.Actor.ID, e.Action, e.Outcome, e.InboxID, e.InboxID, e.ProjectID, e.InboxID, e.ResourceType, e.ResourceID, e.Count, e.RoleBefore, e.RoleAfter, string(data))
	return err
}
func (s *SQLite) auditedExec(ctx context.Context, e audit.Event, statement string, args ...any) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		e.Count = n
		if err := insertAudit(ctx, tx, e); err != nil {
			return 0, err
		}
	}
	return n, tx.Commit()
}
func (s *SQLite) AppendAudit(ctx context.Context, e audit.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertAudit(ctx, tx, e); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) ListAudit(ctx context.Context, f audit.Filter) ([]audit.Event, error) {
	if f.Limit < 1 || f.Limit > 201 {
		f.Limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,at,actor_kind,actor_id,action,outcome,project_id,inbox_id,resource_type,resource_id,count,role_before,role_after,permissions FROM audit_events
 WHERE (?='' OR project_id=?) AND (?='' OR inbox_id=?) AND (?='' OR action=?) AND (?='' OR outcome=?) AND (?='' OR actor_id=?) AND (?=0 OR id<?) AND at>=?
 ORDER BY id DESC LIMIT ?`, f.ProjectID, f.ProjectID, f.InboxID, f.InboxID, f.Action, f.Action, f.Outcome, f.Outcome, f.ActorID, f.ActorID, f.Before, f.Before, f.Since.UTC().Format(timestamp), f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []audit.Event{}
	for rows.Next() {
		var e audit.Event
		var at, p string
		if err := rows.Scan(&e.ID, &at, &e.Actor.Kind, &e.Actor.ID, &e.Action, &e.Outcome, &e.ProjectID, &e.InboxID, &e.ResourceType, &e.ResourceID, &e.Count, &e.RoleBefore, &e.RoleAfter, &p); err != nil {
			return nil, err
		}
		e.At, err = time.Parse(timestamp, at)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(p), &e.Permissions); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
