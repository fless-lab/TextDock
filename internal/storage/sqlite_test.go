package storage

import (
	"context"
	"database/sql/driver"
	"path/filepath"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
)

func TestMemoryDatabaseSurvivesDiscardedRequestConnection(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, _ := message.New(message.Input{To: "+33612345678", From: "Acme", Body: "Keep this message"}, "api")
	if err := s.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	connection, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Raw(func(any) error { return driver.ErrBadConn })
	connection.Close()
	if got, err := s.Get(ctx, m.ID); err != nil || got.Body != m.Body {
		t.Fatalf("recycled connection lost the memory database: %+v %v", got, err)
	}
	if _, err := s.db.Exec(`INSERT INTO inboxes VALUES ('invalid', 'missing-project', 'bad')`); err == nil {
		t.Fatal("recycled connection lost foreign-key enforcement")
	}
}

func TestPersistenceIsolationOrderingAndLiteralSearch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "messages.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := message.New(message.Input{To: "+33612345678", From: "Acme", Body: "100% code 482193", RunID: "run-a"}, "api")
	m.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 100000000, time.UTC)
	if err := s.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	n := m
	n.ID = "newer"
	n.RunID = "run-b"
	n.Body = "another message"
	n.CreatedAt = m.CreatedAt.Add(time.Millisecond)
	if err := s.Save(ctx, n); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items, err := s.List(ctx, message.Filter{})
	if err != nil || len(items) != 2 || items[0].ID != "newer" {
		t.Fatalf("reopen/order: %+v %v", items, err)
	}
	for _, filter := range []message.Filter{{RunID: "run-a"}, {Query: "%"}, {Query: "482193", To: m.To}} {
		items, err := s.List(ctx, filter)
		if err != nil || len(items) != 1 || items[0].ID != m.ID {
			t.Fatalf("filter %+v: %+v %v", filter, items, err)
		}
	}
	items, err = s.List(ctx, message.Filter{Since: n.CreatedAt})
	if err != nil || len(items) != 1 || items[0].ID != n.ID {
		t.Fatalf("since: %+v %v", items, err)
	}
	if ok, err := s.Delete(ctx, m.ID); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}
	if ok, err := s.Delete(ctx, m.ID); ok || err != nil {
		t.Fatalf("repeat delete: %v %v", ok, err)
	}
}
