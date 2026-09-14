package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/message"
)

func TestPairingSingleUseExpiryPersistenceAndMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pairings.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := message.New(message.Input{To: "+33612345678", From: "Acme", Body: "Preserve me"}, "api")
	// Original v0.1 schema fixture, independent of the current migration code.
	if _, err := db.Exec(`CREATE TABLE messages(id TEXT PRIMARY KEY, recipient TEXT NOT NULL, run_id TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL);
	CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES (1);`); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(m)
	if _, err := db.Exec(`INSERT INTO messages VALUES (?, ?, ?, ?, ?, ?)`, m.ID, m.To, m.RunID, m.Body, m.CreatedAt.Format(timestamp), string(payload)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, m.ID); err != nil || got.Body != m.Body {
		t.Fatalf("migration lost message: %+v %v", got, err)
	}
	scope := connect.Scope{Inbox: "local", To: m.To, RunID: "run-a"}
	if err := s.CreatePair(ctx, connect.Hash("challenge"), scope, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			d := connect.Device{ID: fmt.Sprintf("dev-%d", i), Name: "Phone", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
			_, err := s.ClaimPair(ctx, connect.Hash("challenge"), connect.Hash("device-secret"), d)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, connect.ErrInvalid) {
				t.Errorf("claim: %v", err)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("one-use challenge had %d winners", successes.Load())
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := s.DeviceByHash(ctx, connect.Hash("device-secret"))
	if err != nil || d.Scope != scope {
		t.Fatalf("session did not survive restart: %+v %v", d, err)
	}
	if ok, err := s.RevokeDevice(ctx, d.ID); !ok || err != nil {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if _, err := s.DeviceByHash(ctx, connect.Hash("device-secret")); !errors.Is(err, connect.ErrInvalid) {
		t.Fatalf("revoked session accepted: %v", err)
	}
	if err := s.CreatePair(ctx, connect.Hash("old"), scope, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimPair(ctx, connect.Hash("old"), "hash", d); !errors.Is(err, connect.ErrInvalid) {
		t.Fatalf("expired pairing accepted: %v", err)
	}
}
