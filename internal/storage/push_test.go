package storage

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/push"
)

func TestPushScopeCoalescingRevocationAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "push.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.SetPushEnabled(true)
	keys, err := s.PushKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreatePair(ctx, "pair", connect.Scope{Inbox: "local", To: "+12025550123", RunID: "run-a"}, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	d, err := s.ClaimPair(ctx, "pair", "token-hash", connect.Device{ID: "phone-a", Name: "Phone", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	var sub push.Subscription
	sub.Endpoint = "https://fcm.googleapis.com/test"
	sub.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	sub.Keys.P256dh = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	if _, err := s.PutPushSubscription(ctx, d.ID, sub); err != nil {
		t.Fatal(err)
	}
	for _, in := range []message.Input{
		{To: "+12025550124", From: "Acme", Body: "other recipient", RunID: "run-a"},
		{To: d.Scope.To, From: "Acme", Body: "other run", RunID: "run-b"},
	} {
		m, _ := message.New(in, "api")
		if err := s.Save(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ClaimPush(ctx, time.Now().Add(2*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("out-of-scope message produced a push")
	}
	for range 3 {
		m, _ := message.New(message.Input{To: d.Scope.To, From: "Acme", Body: "Code 123456", RunID: "run-a"}, "api")
		if err := s.Save(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	s.db.QueryRow(`SELECT count(*) FROM push_jobs`).Scan(&count)
	if count != 1 {
		t.Fatalf("alerts did not coalesce: %d", count)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, _ := s.PushKeys(ctx)
	if after != keys {
		t.Fatal("VAPID identity changed after restart")
	}
	job, err := s.ClaimPush(ctx, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if job.DeviceID != d.ID {
		t.Fatal("push routed to wrong device")
	}
	if ok, err := s.RevokeDevice(ctx, d.ID); !ok || err != nil {
		t.Fatal(err)
	}
	if active, _ := s.PushJobActive(ctx, job); active {
		t.Fatal("revoked device retained active dispatch")
	}
	if err := s.FinishPush(ctx, job, push.Outcome{State: "accepted"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutPushSubscription(ctx, d.ID, sub); !errors.Is(err, push.ErrSession) {
		t.Fatal("revoked session re-registered notifications")
	}
}
