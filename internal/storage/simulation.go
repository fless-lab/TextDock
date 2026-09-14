package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/simulation"
)

func (s *SQLite) migrateSimulation() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version > 4 {
		return errors.New("database schema is newer than this TextDock version")
	}
	if version == 3 {
		_, err := tx.Exec(`CREATE TABLE scenarios(id TEXT PRIMARY KEY, inbox TEXT NOT NULL REFERENCES inboxes(id), config TEXT NOT NULL);
		CREATE TABLE message_events(id INTEGER PRIMARY KEY AUTOINCREMENT, message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE, status TEXT NOT NULL, at TEXT NOT NULL, detail TEXT NOT NULL);
		CREATE INDEX events_message ON message_events(message_id, id);
		CREATE TABLE jobs(id TEXT PRIMARY KEY, message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE, kind TEXT NOT NULL, due INTEGER NOT NULL, state TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0, max_attempts INTEGER NOT NULL DEFAULT 5, lease_until INTEGER NOT NULL DEFAULT 0, payload TEXT NOT NULL);
		CREATE INDEX jobs_ready ON jobs(state, due);
		CREATE INDEX jobs_message_order ON jobs(message_id,kind,due);
		CREATE TABLE webhook_attempts(id INTEGER PRIMARY KEY AUTOINCREMENT, job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE, message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE, payload TEXT NOT NULL);
		CREATE INDEX attempts_message ON webhook_attempts(message_id, id);
		UPDATE messages SET payload = json_set(payload, '$.mode', 'capture', '$.direction', 'outbound');
		INSERT INTO message_events(message_id,status,at,detail) SELECT id, json_extract(payload,'$.status'), created_at, 'Imported capture' FROM messages;
		INSERT INTO schema_migrations VALUES (4);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) PutScenario(ctx context.Context, scenario simulation.Scenario) error {
	data, err := json.Marshal(scenario)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO scenarios VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET inbox=excluded.inbox, config=excluded.config`, scenario.ID, scenario.Inbox, string(data))
	return err
}
func (s *SQLite) GetScenario(ctx context.Context, id string) (simulation.Scenario, error) {
	var scenario simulation.Scenario
	var data string
	if err := s.db.QueryRowContext(ctx, `SELECT config FROM scenarios WHERE id=?`, id).Scan(&data); err != nil {
		return scenario, err
	}
	err := json.Unmarshal([]byte(data), &scenario)
	return scenario, err
}
func (s *SQLite) ListScenarios(ctx context.Context, inbox string) ([]simulation.Scenario, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config FROM scenarios WHERE inbox=? ORDER BY id LIMIT 200`, inbox)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]simulation.Scenario, 0)
	for rows.Next() {
		var data string
		var scenario simulation.Scenario
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &scenario); err != nil {
			return nil, err
		}
		out = append(out, scenario)
	}
	return out, rows.Err()
}
func (s *SQLite) DeleteScenario(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scenarios WHERE id=?`, id)
	return err
}

func insertMessage(ctx context.Context, tx *sql.Tx, m message.Message) (int64, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO messages(id,recipient,run_id,body,created_at,payload,inbox,favorite) VALUES(?,?,?,?,?,?,?,?)`, m.ID, m.To, m.RunID, m.Body, m.CreatedAt.UTC().Format(timestamp), string(data), m.Inbox, m.Favorite)
	if err != nil {
		return 0, err
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO message_events(message_id,status,at,detail) VALUES(?,?,?,?)`, m.ID, m.Status, m.CreatedAt.UTC().Format(timestamp), "Message accepted in "+m.Mode+" mode")
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}
func insertJob(ctx context.Context, tx *sql.Tx, job simulation.Job) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO jobs(id,message_id,kind,due,payload) VALUES(?,?,?,?,?)`, job.ID, job.MessageID, job.Kind, job.Due.UnixMilli(), job.Payload)
	return err
}
func (s *SQLite) Schedule(ctx context.Context, m message.Message, jobs []simulation.Job) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	eventID, err := insertMessage(ctx, tx, m)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.Kind == "webhook" {
			var d simulation.Delivery
			if err := json.Unmarshal([]byte(job.Payload), &d); err != nil {
				return err
			}
			d.EventID = eventID
			data, _ := json.Marshal(d)
			job.Payload = string(data)
		}
		if err := insertJob(ctx, tx, job); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) NextJob(ctx context.Context, now time.Time) (simulation.Job, error) {
	var job simulation.Job
	var due int64
	err := s.db.QueryRowContext(ctx, `UPDATE jobs SET state='running', attempts=attempts+1, lease_until=?
		WHERE id=(SELECT candidate.id FROM jobs AS candidate WHERE candidate.due<=? AND (candidate.state='pending' OR (candidate.state='running' AND candidate.lease_until<=?))
		AND (candidate.kind!='transition' OR NOT EXISTS(SELECT 1 FROM jobs AS earlier WHERE earlier.message_id=candidate.message_id AND earlier.kind='transition' AND earlier.state!='done' AND (earlier.due<candidate.due OR (earlier.due=candidate.due AND earlier.id<candidate.id)))) ORDER BY candidate.due,candidate.id LIMIT 1)
		RETURNING id,message_id,kind,due,attempts,max_attempts,payload`, now.Add(30*time.Second).UnixMilli(), now.UnixMilli(), now.UnixMilli()).Scan(&job.ID, &job.MessageID, &job.Kind, &due, &job.Attempts, &job.MaxAttempts, &job.Payload)
	job.Due = time.UnixMilli(due).UTC()
	return job, err
}
func (s *SQLite) CompleteTransition(ctx context.Context, job simulation.Job, now time.Time) (message.Message, error) {
	var m message.Message
	var transition simulation.Transition
	if err := json.Unmarshal([]byte(job.Payload), &transition); err != nil {
		return m, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE jobs SET state='done' WHERE id=? AND state='running' AND attempts=?`, job.ID, job.Attempts)
	if err != nil {
		return m, err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return m, sql.ErrNoRows
	}
	var payload string
	if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id=?`, job.MessageID).Scan(&payload); err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return m, err
	}
	m.Status = transition.Status
	data, _ := json.Marshal(m)
	if _, err := tx.ExecContext(ctx, `UPDATE messages SET payload=? WHERE id=?`, string(data), m.ID); err != nil {
		return m, err
	}
	r, err = tx.ExecContext(ctx, `INSERT INTO message_events(message_id,status,at,detail) VALUES(?,?,?,?)`, m.ID, m.Status, now.UTC().Format(timestamp), "Simulated transition")
	if err != nil {
		return m, err
	}
	eventID, _ := r.LastInsertId()
	if transition.URL != "" {
		body, _ := json.Marshal(simulation.Delivery{URL: transition.URL, Format: transition.Format, EventID: eventID, Message: m})
		if err := insertJob(ctx, tx, simulation.Job{ID: "job_" + rand.Text(), MessageID: m.ID, Kind: "webhook", Due: now, Payload: string(body)}); err != nil {
			return m, err
		}
	}
	return m, tx.Commit()
}
func (s *SQLite) FinishWebhook(ctx context.Context, job simulation.Job, attempt simulation.Attempt, success bool, next time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	state := "pending"
	if success {
		state = "done"
	} else if job.Attempts >= job.MaxAttempts {
		state = "failed"
	}
	r, err := tx.ExecContext(ctx, `UPDATE jobs SET state=?,due=?,lease_until=0 WHERE id=? AND state='running' AND attempts=?`, state, next.UnixMilli(), job.ID, job.Attempts)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return sql.ErrNoRows
	}
	data, _ := json.Marshal(attempt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO webhook_attempts(job_id,message_id,payload) VALUES(?,?,?)`, job.ID, job.MessageID, string(data)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) Events(ctx context.Context, id string) ([]simulation.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,message_id,status,at,detail FROM message_events WHERE message_id=? ORDER BY id LIMIT 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]simulation.Event, 0)
	for rows.Next() {
		var e simulation.Event
		var at string
		if err := rows.Scan(&e.ID, &e.MessageID, &e.Status, &at, &e.Detail); err != nil {
			return nil, err
		}
		e.At, err = time.Parse(timestamp, at)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *SQLite) Attempts(ctx context.Context, id string) ([]simulation.Attempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,payload FROM webhook_attempts WHERE message_id=? ORDER BY id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]simulation.Attempt, 0)
	for rows.Next() {
		var a simulation.Attempt
		var data string
		var id int64
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &a); err != nil {
			return nil, err
		}
		a.ID = id
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *SQLite) RetryWebhook(ctx context.Context, id string) (bool, error) {
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET state='pending',due=?,lease_until=0,max_attempts=attempts+5 WHERE id=? AND kind='webhook' AND state IN ('done','failed')`, time.Now().UnixMilli(), id)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n > 0, err
}

var _ simulation.Repository = (*SQLite)(nil)
