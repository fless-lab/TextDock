package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
)

func TestLargeInboxCursorSearchAndSnapshot(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "large.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO messages(id,recipient,run_id,body,created_at,payload,inbox,favorite) VALUES (?,?,?,?,?,?,?,0)`)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 100000 {
		m := message.Message{ID: fmt.Sprintf("msg_%06d", i), Inbox: "local", To: "+33612345678", From: "Load test", Body: fmt.Sprintf("Message %06d", i), RunID: "load", CreatedAt: base.Add(time.Duration(i) * time.Millisecond), Tags: []string{}, Status: "captured"}
		data, _ := json.Marshal(m)
		if _, err := stmt.Exec(m.ID, m.To, m.RunID, m.Body, m.CreatedAt.Format(timestamp), string(data), m.Inbox); err != nil {
			t.Fatal(err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	page, err := s.List(ctx, message.Filter{Inbox: "local", Limit: 200})
	if err != nil || len(page) != 200 || page[0].ID != "msg_099999" {
		t.Fatalf("large page: %d %v", len(page), err)
	}
	last := page[len(page)-1]
	page, err = s.List(ctx, message.Filter{Inbox: "local", Limit: 200, Before: last.CreatedAt, BeforeID: last.ID})
	if err != nil || len(page) != 200 || page[0].ID != "msg_099799" {
		t.Fatalf("large cursor: %d %v", len(page), err)
	}
	page, err = s.List(ctx, message.Filter{Inbox: "local", Query: "Message 001234"})
	if err != nil || len(page) != 1 || page[0].ID != "msg_001234" {
		t.Fatalf("large search: %d %v", len(page), err)
	}
	t.Logf("100k rows: two bounded pages and literal search in %s", time.Since(started))
	snapshot := filepath.Join(t.TempDir(), "backup.db")
	if err := s.Backup(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Prune(ctx, base.Add(99999*time.Millisecond)); err != nil || n != 99999 {
		t.Fatalf("retention: %d %v", n, err)
	}
	backup, err := Open(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if n, err := backup.Purge(ctx, "local"); err != nil || n != 100000 {
		t.Fatalf("snapshot lost data: %d %v", n, err)
	}
}
