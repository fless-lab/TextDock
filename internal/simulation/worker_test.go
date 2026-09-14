package simulation_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/application"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/simulation"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestInboundSignedWebhookRetryReplayAndCascade(t *testing.T) {
	ctx := context.Background()
	const secret = "test-webhook-secret"
	var calls atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(data)
		if r.Header.Get("X-TextDock-Signature") != "sha256="+hex.EncodeToString(mac.Sum(nil)) {
			t.Error("invalid webhook signature")
		}
		if r.Header.Get("X-TextDock-Event-ID") == "" {
			t.Error("missing stable delivery key")
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			w.Write([]byte("try later"))
			return
		}
		w.WriteHeader(204)
	}))
	defer receiver.Close()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m, err := (application.Capture{Messages: db, Simulation: db}).Send(ctx, message.Input{To: "+33612345678", From: "+33699999999", Body: "STOP", Direction: "inbound", CallbackURL: receiver.URL}, "api")
	if err != nil || m.Status != "received" {
		t.Fatalf("inbound: %+v %v", m, err)
	}
	w := simulation.Worker{Store: db, Secret: secret}
	now := time.Now().UTC().Add(time.Second)
	if err := w.Step(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := w.Step(ctx, now.Add(time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("retry ran too early: %v", err)
	}
	if err := w.Step(ctx, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	attempts, err := db.Attempts(ctx, m.ID)
	if err != nil || len(attempts) != 2 || attempts[0].Status != 204 || attempts[1].Status != 503 || attempts[1].Response != "try later" {
		t.Fatalf("attempt history: %+v %v", attempts, err)
	}
	if ok, err := db.RetryWebhook(ctx, attempts[0].JobID); !ok || err != nil {
		t.Fatalf("manual replay: %v %v", ok, err)
	}
	if err := w.Step(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("delivery count: %d", calls.Load())
	}
	if ok, err := db.Delete(ctx, m.ID); !ok || err != nil {
		t.Fatal(err)
	}
	if items, err := db.Attempts(ctx, m.ID); err != nil || len(items) != 0 {
		t.Fatalf("deleted message retained callback payloads: %+v %v", items, err)
	}
}
