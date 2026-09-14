package relay_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/application"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/relay"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestIdempotentRelayAndUnknownDoesNotRetry(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		user, password, _ := r.BasicAuth()
		if user != "ACtest" || password != "test-secret" {
			t.Error("provider credentials mismatch")
		}
		r.ParseForm()
		if r.PostForm.Get("To") != "+33612345678" {
			t.Error("recipient mismatch")
		}
		if !strings.HasPrefix(r.PostForm.Get("StatusCallback"), "https://receiver.test/relay/twilio/status?message_id=") {
			t.Error("missing correlated callback")
		}
		w.WriteHeader(503)
		w.Write([]byte(`{"message":"ambiguous provider error"}`))
	}))
	defer provider.Close()
	cfg := relay.Config{Driver: "twilio", AccountSID: "ACtest", AuthToken: "test-secret", Endpoint: provider.URL, PublicURL: "https://receiver.test", Limit: 10}
	service := &relay.Service{Store: db, Config: cfg}
	capture := application.Capture{Messages: db, Simulation: db, Relay: service}
	in := message.Input{Mode: "relay", To: "+33612345678", From: "Acme", Body: "Code 482193", IdempotencyKey: "one-intent"}
	m, err := capture.Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := capture.Send(ctx, in, "api")
	if err != nil || duplicate.ID != m.ID {
		t.Fatalf("idempotency: %+v %v", duplicate, err)
	}
	worker := relay.Worker{Store: db, Config: cfg}
	if err := worker.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Step(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("uncertain send retried: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider received %d requests", calls.Load())
	}
	m, _ = db.Get(ctx, m.ID)
	if m.Status != "unknown" {
		t.Fatalf("ambiguous outcome: %s", m.Status)
	}
	in.Body = "Different content"
	if _, err := capture.Send(ctx, in, "api"); !errors.Is(err, relay.ErrIdempotency) {
		t.Fatalf("conflicting key accepted: %v", err)
	}
	in.Body = "Code 482193"
	db.Delete(ctx, m.ID)
	if _, err := capture.Send(ctx, in, "api"); !errors.Is(err, relay.ErrIdempotency) {
		t.Fatalf("purge allowed a duplicate carrier send: %v", err)
	}
}

func TestGatewayLeasesLimitsAndExpiry(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := &relay.Service{Store: db, Config: relay.Config{Driver: "android", Limit: 2}}
	capture := application.Capture{Messages: db, Simulation: db, Relay: service}
	in := message.Input{Mode: "relay", To: "+33612345678", From: "Acme", Body: "Code 123456"}
	a, err := capture.Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	b, err := capture.Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	if a.From != "SIM" {
		t.Fatal("Android must not impersonate an alphanumeric sender")
	}
	if _, err := capture.Send(ctx, in, "api"); !errors.Is(err, relay.ErrLimit) {
		t.Fatalf("admission limit: %v", err)
	}
	now := time.Now().UTC()
	job, err := db.ClaimRelay(ctx, "android", "gateway-a", now, 1)
	if err != nil {
		t.Fatal(err)
	}
	otherID := a.ID
	if job.MessageID == a.ID {
		otherID = b.ID
	}
	if _, err := db.ClaimRelay(ctx, "android", "gateway-b", now, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("dispatch limit: %v", err)
	}
	if _, err := db.CompleteRelay(ctx, job.ID, job.LeaseToken, "gateway-b", relay.Result{State: "sent"}); !errors.Is(err, relay.ErrLease) {
		t.Fatalf("cross-gateway ack: %v", err)
	}
	if _, err := db.CompleteRelay(ctx, job.ID, job.LeaseToken, "gateway-a", relay.Result{State: "sent"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CompleteRelay(ctx, job.ID, job.LeaseToken, "gateway-a", relay.Result{State: "sent"}); err != nil {
		t.Fatalf("replayed acknowledgement: %v", err)
	}
	if _, err := db.ClaimRelay(ctx, "android", "gateway-a", now.Add(61*time.Second), 1); err != nil {
		t.Fatal(err)
	}
	if n, err := db.ExpireRelays(ctx, now.Add(122*time.Second)); err != nil || n != 1 {
		t.Fatalf("lost lease: %d %v", n, err)
	}
	if _, err := db.ClaimRelay(ctx, "android", "gateway-a", now.Add(123*time.Second), 2); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired lease was redelivered")
	}
	b, _ = db.Get(ctx, otherID)
	if b.Status != "unknown" {
		t.Fatalf("lost dispatch: %s", b.Status)
	}
	short := &relay.Service{Store: db, Config: relay.Config{Driver: "android", Limit: 10, TTL: time.Second}}
	c, err := (application.Capture{Messages: db, Simulation: db, Relay: short}).Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ClaimRelay(ctx, "android", "gateway-a", now.Add(time.Hour), 10); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired queue job dispatched")
	}
	db.ExpireRelays(ctx, now.Add(time.Hour))
	c, _ = db.Get(ctx, c.ID)
	if c.Status != "expired" {
		t.Fatalf("queue TTL: %s", c.Status)
	}
}
