package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/relay"
)

func (s *SQLite) migrateRelay() error {
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
	if version == 4 {
		_, err := tx.Exec(`CREATE TABLE relay_jobs(id TEXT PRIMARY KEY, message_id TEXT UNIQUE NOT NULL REFERENCES messages(id) ON DELETE CASCADE, driver TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'queued', owner TEXT NOT NULL DEFAULT '', lease_hash TEXT NOT NULL DEFAULT '', lease_until INTEGER NOT NULL DEFAULT 0, provider_id TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL);
		CREATE INDEX relay_queue ON relay_jobs(driver,state,created_at);
		CREATE TABLE relay_keys(inbox TEXT NOT NULL, key TEXT NOT NULL, fingerprint BLOB NOT NULL, message_id TEXT REFERENCES messages(id) ON DELETE SET NULL, PRIMARY KEY(inbox,key));
		CREATE TABLE relay_dispatches(at INTEGER NOT NULL);
		CREATE INDEX relay_dispatch_time ON relay_dispatches(at);
		CREATE TABLE relay_admissions(at INTEGER NOT NULL);
		CREATE INDEX relay_admission_time ON relay_admissions(at);
		CREATE TABLE gateways(id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL, expires_at INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0);
		INSERT INTO schema_migrations VALUES (5);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) EnqueueRelay(ctx context.Context, m message.Message, driver string, limit int, expires time.Time) (message.Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	canonical, err := json.Marshal([]string{driver, m.To, m.From, m.Body, m.RunID})
	if err != nil {
		return m, err
	}
	fingerprint := sha256.Sum256(canonical)
	if m.IdempotencyKey != "" {
		var previous []byte
		var id sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT fingerprint,message_id FROM relay_keys WHERE inbox=? AND key=?`, m.Inbox, m.IdempotencyKey).Scan(&previous, &id)
		if err == nil {
			if string(previous) != string(fingerprint[:]) || !id.Valid {
				return m, relay.ErrIdempotency
			}
			var data string
			if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id=?`, id.String).Scan(&data); err != nil {
				return m, err
			}
			var existing message.Message
			if err := json.Unmarshal([]byte(data), &existing); err != nil {
				return m, err
			}
			return existing, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return m, err
		}
	}
	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `DELETE FROM relay_admissions WHERE at < ?`, now-60000); err != nil {
		return m, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM relay_admissions`).Scan(&count); err != nil {
		return m, err
	}
	if count >= limit {
		return m, relay.ErrLimit
	}
	if _, err := insertMessage(ctx, tx, m); err != nil {
		return m, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO relay_jobs(id,message_id,driver,created_at,expires_at) VALUES(?,?,?,?,?)`, "relay_"+rand.Text(), m.ID, driver, now, expires.UnixMilli()); err != nil {
		return m, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO relay_admissions VALUES(?)`, now); err != nil {
		return m, err
	}
	if m.IdempotencyKey != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO relay_keys VALUES(?,?,?,?)`, m.Inbox, m.IdempotencyKey, fingerprint[:], m.ID); err != nil {
			return m, err
		}
	}
	return m, tx.Commit()
}

const relayColumns = `id,message_id,driver,state,owner,provider_id,error,created_at,lease_until,expires_at`

func scanRelay(row interface{ Scan(...any) error }) (relay.Job, error) {
	var j relay.Job
	var created, lease, expires int64
	err := row.Scan(&j.ID, &j.MessageID, &j.Driver, &j.State, &j.Owner, &j.ProviderID, &j.Error, &created, &lease, &expires)
	j.CreatedAt, j.LeaseUntil, j.ExpiresAt = time.UnixMilli(created).UTC(), time.UnixMilli(lease).UTC(), time.UnixMilli(expires).UTC()
	return j, err
}
func (s *SQLite) RelayJob(ctx context.Context, messageID string) (relay.Job, error) {
	return scanRelay(s.db.QueryRowContext(ctx, `SELECT `+relayColumns+` FROM relay_jobs WHERE message_id=?`, messageID))
}
func (s *SQLite) ClaimRelay(ctx context.Context, driver, owner string, now time.Time, limit int) (relay.Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return relay.Job{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM relay_dispatches WHERE at < ?`, now.UnixMilli()-60000); err != nil {
		return relay.Job{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM relay_dispatches`).Scan(&count); err != nil {
		return relay.Job{}, err
	}
	if count >= limit {
		return relay.Job{}, sql.ErrNoRows
	}
	token := rand.Text()
	j, err := scanRelay(tx.QueryRowContext(ctx, `UPDATE relay_jobs SET state='dispatching',owner=?,lease_hash=?,lease_until=? WHERE id=(SELECT id FROM relay_jobs WHERE driver=? AND state='queued' AND expires_at>? ORDER BY created_at,id LIMIT 1) RETURNING `+relayColumns, owner, connect.Hash(token), now.Add(time.Minute).UnixMilli(), driver, now.UnixMilli()))
	if err != nil {
		return j, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO relay_dispatches VALUES(?)`, now.UnixMilli()); err != nil {
		return j, err
	}
	var data string
	if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id=?`, j.MessageID).Scan(&data); err != nil {
		return j, err
	}
	var m message.Message
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return j, err
	}
	j.To, j.From, j.Body, j.LeaseToken = m.To, m.From, m.Body, token
	j.Inbox, j.RunID = m.Inbox, m.RunID
	if _, err := updateRelayMessage(ctx, tx, j.MessageID, "dispatching", "Real SMS dispatch started"); err != nil {
		return j, err
	}
	return j, tx.Commit()
}

func updateRelayMessage(ctx context.Context, tx *sql.Tx, id, state, detail string) (message.Message, error) {
	var m message.Message
	var payload string
	if err := tx.QueryRowContext(ctx, `SELECT payload FROM messages WHERE id=?`, id).Scan(&payload); err != nil {
		return m, err
	}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return m, err
	}
	if m.Status == state {
		return m, nil
	}
	m.Status = state
	data, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE messages SET payload=? WHERE id=?`, string(data), id); err != nil {
		return m, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO message_events(message_id,status,at,detail) VALUES(?,?,?,?)`, id, state, time.Now().UTC().Format(timestamp), detail); err != nil {
		return m, err
	}
	return m, nil
}
func (s *SQLite) CompleteRelay(ctx context.Context, id, token, owner string, result relay.Result) (message.Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return message.Message{}, err
	}
	defer tx.Rollback()
	var messageID, state, providerID string
	err = tx.QueryRowContext(ctx, `SELECT message_id,state,provider_id FROM relay_jobs WHERE id=? AND lease_hash=? AND owner=?`, id, connect.Hash(token), owner).Scan(&messageID, &state, &providerID)
	if errors.Is(err, sql.ErrNoRows) {
		return message.Message{}, relay.ErrLease
	}
	if err != nil {
		return message.Message{}, err
	}
	if providerID != "" && result.ProviderID != "" && providerID != result.ProviderID {
		return message.Message{}, relay.ErrLease
	}
	if (state == "sent" || state == "delivered" || state == "failed") && (result.State == "accepted" || result.State == "unknown") {
		result.State = state
		result.Error = ""
	}
	if state != "dispatching" && state != "unknown" && state != result.State {
		return message.Message{}, relay.ErrLease
	}
	if result.ProviderID == "" {
		result.ProviderID = providerID
	}
	if _, err := tx.ExecContext(ctx, `UPDATE relay_jobs SET state=?,provider_id=?,error=?,lease_until=0 WHERE id=?`, result.State, result.ProviderID, result.Error, id); err != nil {
		return message.Message{}, err
	}
	m, err := updateRelayMessage(ctx, tx, messageID, result.State, "Real SMS relay: "+result.State+" "+result.Error)
	if err != nil {
		return m, err
	}
	return m, tx.Commit()
}
func (s *SQLite) RelayReceipt(ctx context.Context, messageID string, result relay.Result) (message.Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return message.Message{}, err
	}
	defer tx.Rollback()
	var state, providerID string
	if err := tx.QueryRowContext(ctx, `SELECT state,provider_id FROM relay_jobs WHERE message_id=? AND driver='twilio'`, messageID).Scan(&state, &providerID); err != nil {
		return message.Message{}, err
	}
	if providerID != "" && providerID != result.ProviderID {
		return message.Message{}, relay.ErrLease
	}
	// Final receipts and later progress cannot be regressed by delayed callbacks.
	rank := map[string]int{"queued": 0, "dispatching": 0, "unknown": 0, "accepted": 1, "sent": 2, "delivered": 3, "failed": 3}
	if rank[state] >= rank[result.State] {
		result.State = state
	}
	detail := ""
	if result.State == "failed" {
		detail = "Twilio reported delivery failure"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE relay_jobs SET state=?,provider_id=?,error=?,lease_until=0 WHERE message_id=?`, result.State, result.ProviderID, detail, messageID); err != nil {
		return message.Message{}, err
	}
	m, err := updateRelayMessage(ctx, tx, messageID, result.State, "Twilio delivery receipt")
	if err != nil {
		return m, err
	}
	return m, tx.Commit()
}
func (s *SQLite) ExpireRelays(ctx context.Context, now time.Time) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT message_id,state FROM relay_jobs WHERE (state='dispatching' AND lease_until<=?) OR (state='queued' AND expires_at<=?)`, now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return 0, err
	}
	states := map[string]string{}
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			rows.Close()
			return 0, err
		}
		states[id] = state
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for id, state := range states {
		result, detail := "unknown", "Dispatch interrupted; automatic resend disabled"
		if state == "queued" {
			result, detail = "expired", "Queue TTL expired before dispatch"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE relay_jobs SET state=?,error=?,lease_until=0 WHERE message_id=?`, result, detail, id); err != nil {
			return 0, err
		}
		if _, err := updateRelayMessage(ctx, tx, id, result, detail); err != nil {
			return 0, err
		}
	}
	return len(states), tx.Commit()
}
func (s *SQLite) CreateGateway(ctx context.Context, g relay.Gateway, hash string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO gateways(id,name,token_hash,expires_at) VALUES(?,?,?,?)`, g.ID, g.Name, hash, g.ExpiresAt.UnixMilli())
	return err
}
func (s *SQLite) GatewayByHash(ctx context.Context, hash string) (relay.Gateway, error) {
	var g relay.Gateway
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT id,name,expires_at,revoked FROM gateways WHERE token_hash=? AND revoked=0 AND expires_at>?`, hash, time.Now().UnixMilli()).Scan(&g.ID, &g.Name, &expires, &g.Revoked)
	if errors.Is(err, sql.ErrNoRows) {
		err = relay.ErrCredential
	}
	g.ExpiresAt = time.UnixMilli(expires).UTC()
	return g, err
}
func (s *SQLite) Gateways(ctx context.Context) ([]relay.Gateway, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,expires_at,revoked FROM gateways ORDER BY id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]relay.Gateway, 0)
	for rows.Next() {
		var g relay.Gateway
		var expires int64
		if err := rows.Scan(&g.ID, &g.Name, &expires, &g.Revoked); err != nil {
			return nil, err
		}
		g.ExpiresAt = time.UnixMilli(expires).UTC()
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *SQLite) RevokeGateway(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE gateways SET revoked=1 WHERE id=?`, id)
	return err
}

var _ relay.Repository = (*SQLite)(nil)
