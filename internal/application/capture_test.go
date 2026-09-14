package application

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/simulation"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestDurableOrderedTransitionsAndLeaseFencing(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "simulation.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario := simulation.Scenario{ID: "scenario-1", Inbox: "local", Name: "delayed", Outcome: "delivered", DelayMS: 2, Seed: "fixed", WebhookFormat: "json"}
	if err := db.PutScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	m, err := (Capture{Messages: db, Simulation: db}).Send(ctx, message.Input{To: "+33612345678", From: "Acme", Body: "Code 482193", ScenarioID: scenario.ID}, "api")
	if err != nil || m.Status != "queued" || m.Mode != "simulate" {
		t.Fatalf("schedule: %+v %v", m, err)
	}
	now := m.CreatedAt.Add(time.Second)
	stale, err := db.NextJob(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	db.Close() // Crash after claim, before committing the first transition.
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w := simulation.Worker{Store: db}
	if err := w.Step(ctx, now.Add(time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("later transition passed a leased predecessor: %v", err)
	}
	if err := w.Step(ctx, now.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CompleteTransition(ctx, stale, now.Add(32*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale worker was not fenced: %v", err)
	}
	if err := w.Step(ctx, now.Add(32*time.Second)); err != nil {
		t.Fatal(err)
	}
	events, err := db.Events(ctx, m.ID)
	if err != nil || len(events) != 3 {
		t.Fatalf("history: %+v %v", events, err)
	}
	for i, want := range []string{"queued", "sent", "delivered"} {
		if events[i].Status != want {
			t.Fatalf("event %d: %s", i, events[i].Status)
		}
	}
	if got, err := db.Get(ctx, m.ID); err != nil || got.Status != "delivered" {
		t.Fatalf("final message: %+v %v", got, err)
	}
}

func TestRejectionAndSeededOutcomes(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := simulation.Scenario{ID: "rule", Inbox: "local", Name: "limited", Outcome: "delivered", RejectStatus: 429, RetryAfter: 7, WebhookFormat: "json"}
	if err := db.PutScenario(ctx, s); err != nil {
		t.Fatal(err)
	}
	service := Capture{Messages: db, Simulation: db}
	in := message.Input{To: "+33612345678", From: "Acme", Body: "Code 123456", ScenarioID: s.ID, RunID: "seed-test"}
	_, err = service.Send(ctx, in, "api")
	var rejected Rejected
	if !errors.As(err, &rejected) || rejected.Status != 429 || rejected.RetryAfter != 7 {
		t.Fatalf("rejection: %v", err)
	}
	if items, err := db.List(ctx, message.Filter{}); err != nil || len(items) != 0 {
		t.Fatal("rejected request was captured")
	}
	s.RejectStatus = 0
	s.FailurePercent = 50
	s.Seed = "reproducible"
	if err := db.PutScenario(ctx, s); err != nil {
		t.Fatal(err)
	}
	a, err := service.Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	b, err := service.Send(ctx, in, "api")
	if err != nil {
		t.Fatal(err)
	}
	w := simulation.Worker{Store: db}
	for range 4 {
		if err := w.Step(ctx, time.Now().UTC().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	a, _ = db.Get(ctx, a.ID)
	b, _ = db.Get(ctx, b.ID)
	if a.Status != b.Status || (a.Status != "failed" && a.Status != "delivered") {
		t.Fatalf("seeded outcome drift: %s %s", a.Status, b.Status)
	}
}
