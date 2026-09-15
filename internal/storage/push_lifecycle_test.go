package storage

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/push"
)

func TestPushTransferRetryAndSessionExpiry(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"a", "b"} {
		if err := s.CreatePair(ctx, id, connect.Scope{Inbox: "local", To: "+12025550123"}, time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ClaimPair(ctx, id, id, connect.Device{ID: id, Name: id, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	var sub push.Subscription
	sub.Endpoint = "https://fcm.googleapis.com/endpoint"
	sub.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	sub.Keys.P256dh = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	first, err := s.PutPushSubscription(ctx, "a", sub)
	if err != nil {
		t.Fatal(err)
	}
	s.QueuePushTest(ctx, "a")
	old, err := s.ClaimPush(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	wrong := sub
	wrong.Keys.Auth = "different"
	if _, err := s.PutPushSubscription(ctx, "b", wrong); !errors.Is(err, push.ErrConflict) {
		t.Fatal("different endpoint keys took over a subscription")
	}
	second, err := s.PutPushSubscription(ctx, "b", sub)
	if err != nil {
		t.Fatal(err)
	}
	if first.Generation == second.Generation {
		t.Fatal("session transfer reused the delivery generation")
	}
	if active, _ := s.PushJobActive(ctx, old); active {
		t.Fatal("old session retained a pending alert after transfer")
	}
	s.FinishPush(ctx, old, push.Outcome{State: "expired"})
	state, _ := s.PushState(ctx, "b")
	if !state.Subscribed {
		t.Fatal("stale expiry deleted a replacement subscription")
	}
	s.QueuePushTest(ctx, "b")
	job, err := s.ClaimPush(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPush(ctx, job, push.Outcome{State: "retrying", HTTPStatus: 503, RetryAfter: time.Second}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimPush(ctx, time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("retry ran before its deadline")
	}
	if _, err := s.db.Exec(`UPDATE devices SET expires_at=0 WHERE id='b'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimPush(ctx, time.Now().Add(2*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired session received a retry")
	}
}
