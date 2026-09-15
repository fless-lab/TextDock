package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/push"
)

func (s *SQLite) migratePush() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version == 7 {
		_, err := tx.Exec(`CREATE TABLE push_keys(id INTEGER PRIMARY KEY CHECK(id=1),public_key TEXT NOT NULL,private_key TEXT NOT NULL);
		CREATE TABLE push_subscriptions(device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,endpoint_hash TEXT UNIQUE NOT NULL,endpoint TEXT NOT NULL,auth TEXT NOT NULL,p256dh TEXT NOT NULL,generation TEXT NOT NULL);
		CREATE TABLE push_jobs(id INTEGER PRIMARY KEY AUTOINCREMENT,device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,generation TEXT NOT NULL,kind TEXT NOT NULL,state TEXT NOT NULL DEFAULT 'pending',attempts INTEGER NOT NULL DEFAULT 0,due INTEGER NOT NULL,expires_at INTEGER NOT NULL,lease_until INTEGER NOT NULL DEFAULT 0,message_id TEXT REFERENCES messages(id) ON DELETE CASCADE);
		CREATE UNIQUE INDEX push_pending_device ON push_jobs(device_id) WHERE state='pending';
		CREATE INDEX push_jobs_due ON push_jobs(state,due);
		CREATE TABLE push_status(device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,state TEXT NOT NULL,http_status INTEGER NOT NULL,detail TEXT NOT NULL,updated_at INTEGER NOT NULL);
		INSERT INTO schema_migrations VALUES(8);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) SetPushEnabled(enabled bool) { s.pushEnabled.Store(enabled) }
func (s *SQLite) PushKeys(ctx context.Context) (push.Keys, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return push.Keys{}, err
	}
	defer tx.Rollback()
	var keys push.Keys
	err = tx.QueryRowContext(ctx, `SELECT public_key,private_key FROM push_keys WHERE id=1`).Scan(&keys.Public, &keys.Private)
	if errors.Is(err, sql.ErrNoRows) {
		private, public, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return keys, err
		}
		keys.Public, keys.Private = public, private
		if _, err := tx.ExecContext(ctx, `INSERT INTO push_keys VALUES(1,?,?)`, public, private); err != nil {
			return keys, err
		}
	} else if err != nil {
		return keys, err
	}
	return keys, tx.Commit()
}
func (s *SQLite) PutPushSubscription(ctx context.Context, device string, sub push.Subscription) (push.State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return push.State{}, err
	}
	defer tx.Rollback()
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=? AND revoked=0 AND expires_at>?)`, device, time.Now().UnixMilli()).Scan(&active); err != nil {
		return push.State{}, err
	}
	if !active {
		return push.State{}, push.ErrSession
	}
	hash := sha256.Sum256([]byte(sub.Endpoint))
	endpointHash := hex.EncodeToString(hash[:])
	var previous, auth, public string
	err = tx.QueryRowContext(ctx, `SELECT device_id,auth,p256dh FROM push_subscriptions WHERE endpoint_hash=?`, endpointHash).Scan(&previous, &auth, &public)
	if err == nil && (auth != sub.Keys.Auth || public != sub.Keys.P256dh) {
		return push.State{}, push.ErrConflict
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return push.State{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE device_id IN (?,?)`, device, previous); err != nil {
		return push.State{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE device_id=? OR endpoint_hash=?`, device, endpointHash); err != nil {
		return push.State{}, err
	}
	generation := rand.Text()
	if _, err := tx.ExecContext(ctx, `INSERT INTO push_subscriptions VALUES(?,?,?,?,?,?)`, device, endpointHash, sub.Endpoint, sub.Keys.Auth, sub.Keys.P256dh, generation); err != nil {
		return push.State{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO push_status VALUES(?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET state=excluded.state,http_status=excluded.http_status,detail=excluded.detail,updated_at=excluded.updated_at`, device, "ready", 0, "Notifications enabled for this session", time.Now().UnixMilli()); err != nil {
		return push.State{}, err
	}
	if err := tx.Commit(); err != nil {
		return push.State{}, err
	}
	return s.PushState(ctx, device)
}
func (s *SQLite) DeletePushSubscription(ctx context.Context, device string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE device_id=?`, device); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE device_id=?`, device); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_status WHERE device_id=?`, device); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) PushState(ctx context.Context, device string) (push.State, error) {
	state := push.State{Status: "disabled"}
	var endpoint string
	var updated sql.NullInt64
	var generation, status, detail sql.NullString
	var httpStatus sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT coalesce(s.endpoint,''),s.generation,p.state,p.http_status,p.detail,p.updated_at,EXISTS(SELECT 1 FROM push_jobs j WHERE j.device_id=d.id AND j.generation=s.generation AND j.expires_at>?) FROM devices d LEFT JOIN push_subscriptions s ON s.device_id=d.id LEFT JOIN push_status p ON p.device_id=d.id WHERE d.id=?`, time.Now().UnixMilli(), device).Scan(&endpoint, &generation, &status, &httpStatus, &detail, &updated, &state.Queued)
	if err != nil {
		return state, err
	}
	state.Subscribed, state.Generation = generation.Valid, generation.String
	if u, err := url.Parse(endpoint); err == nil {
		state.EndpointHost = u.Hostname()
	}
	if status.Valid {
		state.Status = status.String
	}
	state.HTTPStatus, state.Detail = int(httpStatus.Int64), detail.String
	if updated.Valid {
		at := time.UnixMilli(updated.Int64).UTC()
		state.UpdatedAt = &at
	}
	return state, nil
}

func enqueuePush(ctx context.Context, tx *sql.Tx, m message.Message) error {
	now := time.Now().UnixMilli()
	_, err := tx.ExecContext(ctx, `INSERT INTO push_jobs(device_id,generation,kind,due,expires_at,message_id)
	 SELECT s.device_id,s.generation,'inbox',?,?,? FROM push_subscriptions s JOIN devices d ON d.id=s.device_id
	 WHERE d.revoked=0 AND d.expires_at>? AND d.inbox=? AND d.recipient=? AND (d.run_id='' OR d.run_id=?)
	 ON CONFLICT(device_id) WHERE state='pending' DO UPDATE SET message_id=excluded.message_id,generation=excluded.generation,expires_at=excluded.expires_at`, now+1000, now+60000, m.ID, now, m.Inbox, m.To, m.RunID)
	return err
}
func (s *SQLite) QueuePushTest(ctx context.Context, device string) error {
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, `INSERT INTO push_jobs(device_id,generation,kind,due,expires_at) SELECT s.device_id,s.generation,'test',?,? FROM push_subscriptions s JOIN devices d ON d.id=s.device_id WHERE d.id=? AND d.revoked=0 AND d.expires_at>? ON CONFLICT(device_id) WHERE state='pending' DO NOTHING`, now, now+60000, device, now)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM push_subscriptions WHERE device_id=?)`, device).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return push.ErrSubscription
		}
	}
	return nil
}
func (s *SQLite) ClaimPush(ctx context.Context, now time.Time) (push.Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return push.Job{}, err
	}
	defer tx.Rollback()
	// Removing a subscription/session invalidates every pending delivery, including
	// a retry left behind by a previous process.
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE expires_at<=? OR NOT EXISTS(SELECT 1 FROM push_subscriptions s JOIN devices d ON d.id=s.device_id WHERE s.device_id=push_jobs.device_id AND s.generation=push_jobs.generation AND d.revoked=0 AND d.expires_at>?)`, now.UnixMilli(), now.UnixMilli()); err != nil {
		return push.Job{}, err
	}
	var job push.Job
	err = tx.QueryRowContext(ctx, `UPDATE push_jobs SET state='running',attempts=attempts+1,lease_until=? WHERE id=(SELECT id FROM push_jobs WHERE due<=? AND (state='pending' OR (state='running' AND lease_until<=?)) ORDER BY due,id LIMIT 1) RETURNING id,device_id,generation,kind,attempts`, now.Add(15*time.Second).UnixMilli(), now.UnixMilli(), now.UnixMilli()).Scan(&job.ID, &job.DeviceID, &job.Generation, &job.Kind, &job.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return job, err
		}
		return job, sql.ErrNoRows
	}
	if err != nil {
		return job, err
	}
	err = tx.QueryRowContext(ctx, `SELECT endpoint,auth,p256dh FROM push_subscriptions WHERE device_id=? AND generation=?`, job.DeviceID, job.Generation).Scan(&job.Subscription.Endpoint, &job.Subscription.Keys.Auth, &job.Subscription.Keys.P256dh)
	if err != nil {
		return job, err
	}
	return job, tx.Commit()
}
func (s *SQLite) PushJobActive(ctx context.Context, job push.Job) (bool, error) {
	var active bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM push_jobs j JOIN push_subscriptions s ON s.device_id=j.device_id AND s.generation=j.generation JOIN devices d ON d.id=s.device_id WHERE j.id=? AND j.attempts=? AND j.state='running' AND d.revoked=0 AND d.expires_at>?)`, job.ID, job.Attempts, time.Now().UnixMilli()).Scan(&active)
	return active, err
}
func (s *SQLite) FinishPush(ctx context.Context, job push.Job, out push.Outcome) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM push_jobs j JOIN push_subscriptions s ON j.device_id=s.device_id AND j.generation=s.generation JOIN devices d ON d.id=j.device_id WHERE j.id=? AND j.attempts=? AND d.revoked=0 AND d.expires_at>?)`, job.ID, job.Attempts, time.Now().UnixMilli()).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		_, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE id=? AND attempts=?`, job.ID, job.Attempts)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO push_status VALUES(?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET state=excluded.state,http_status=excluded.http_status,detail=excluded.detail,updated_at=excluded.updated_at`, job.DeviceID, out.State, out.HTTPStatus, out.Detail, time.Now().UnixMilli()); err != nil {
		return err
	}
	if out.State == "retrying" {
		// If a newer alert is pending, it replaces this retry instead of producing
		// a burst of stale lock-screen notifications.
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE id=? AND EXISTS(SELECT 1 FROM push_jobs newer WHERE newer.device_id=? AND newer.state='pending' AND newer.id!=?)`, job.ID, job.DeviceID, job.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE push_jobs SET state='pending',due=?,lease_until=0 WHERE id=? AND attempts=?`, time.Now().Add(out.RetryAfter).UnixMilli(), job.ID, job.Attempts); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE id=? AND attempts=?`, job.ID, job.Attempts); err != nil {
			return err
		}
	}
	if out.State == "expired" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_jobs WHERE device_id=? AND generation=?`, job.DeviceID, job.Generation); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE device_id=? AND generation=?`, job.DeviceID, job.Generation); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var _ push.Repository = (*SQLite)(nil)
