package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/fless-lab/TextDock/internal/access"
	"github.com/fless-lab/TextDock/internal/audit"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditFailureRollsBackMutations(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := audit.WithActor(context.Background(), audit.Actor{Kind: "user", ID: "user_actor"})
	u := access.User{ID: "user_target", Username: "target", Name: "Private name", CreatedAt: time.Now()}
	if err := s.CreateUser(ctx, u, "private-password-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetMembership(ctx, "default", u.Username, "member"); err != nil {
		t.Fatal(err)
	}
	session := access.UserSession{ID: "session_test", User: u, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateUserSession(ctx, session, "session-hash", "private-password-hash"); err != nil {
		t.Fatal(err)
	}
	key := access.Key{ID: "key_test", Name: "CI", ProjectID: "default", InboxID: "local", Permissions: []string{"messages:read"}, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateAPIKey(ctx, key, "key-hash"); err != nil {
		t.Fatal(err)
	}
	m, err := message.New(message.Input{Inbox: "local", To: "+12025550123", From: "Private", Body: "Sensitive code 482193"}, "api")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	before, err := s.ListAudit(ctx, audit.Filter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'audit unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetMembership(ctx, "default", u.Username, "admin"); err == nil {
		t.Fatal("membership ignored audit failure")
	}
	members, _ := s.ProjectMembers(ctx, "default")
	if len(members) != 1 || members[0].Role != "member" {
		t.Fatal("membership was not rolled back")
	}
	if _, err := s.UserSessionByHash(ctx, "session-hash"); err != nil {
		t.Fatal("session revocation escaped rollback", err)
	}
	if _, err := s.SetUserPassword(ctx, u.ID, "new-hash", ""); err == nil {
		t.Fatal("password ignored audit failure")
	}
	_, hash, _ := s.UserCredential(ctx, u.Username)
	if hash != "private-password-hash" {
		t.Fatal("password change escaped rollback")
	}
	if _, err := s.RevokeAPIKey(ctx, key.ID); err == nil {
		t.Fatal("key revoke ignored audit failure")
	}
	if _, err := s.APIKeyByHash(ctx, "key-hash"); err != nil {
		t.Fatal("key revocation escaped rollback", err)
	}
	favorite := true
	if _, err := s.Update(ctx, m.ID, message.Update{Favorite: &favorite}); err == nil {
		t.Fatal("metadata ignored audit failure")
	}
	if _, err := s.Delete(ctx, m.ID); err == nil {
		t.Fatal("delete ignored audit failure")
	}
	if _, err := s.Purge(ctx, "local"); err == nil {
		t.Fatal("purge ignored audit failure")
	}
	if _, err := s.Prune(ctx, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("retention ignored audit failure")
	}
	got, err := s.Get(ctx, m.ID)
	if err != nil || got.Favorite {
		t.Fatal("message mutation escaped rollback", err)
	}
	m.ID = "msg_rollback"
	if err := s.Save(ctx, m); err == nil {
		t.Fatal("capture ignored audit failure")
	}
	if _, err := s.Get(ctx, m.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("capture escaped rollback", err)
	}
	if err := s.CreateProject(ctx, workspace.Project{ID: "rolled_back", Name: "Rollback"}, workspace.Inbox{ID: "rolled_back", ProjectID: "rolled_back", Name: "Inbox"}); err == nil {
		t.Fatal("project ignored audit failure")
	}
	if exists, _ := s.InboxExists(ctx, "rolled_back"); exists {
		t.Fatal("project inbox escaped rollback")
	}
	after, err := s.ListAudit(ctx, audit.Filter{Limit: 200})
	if err != nil || len(after) != len(before) {
		t.Fatal("failed writes changed audit history", err)
	}
}

func TestAuditSurvivesDeletionRestartAndIdempotentRevocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := audit.WithActor(context.Background(), audit.Actor{Kind: "api_key", ID: "key_actor"})
	m, _ := message.New(message.Input{Inbox: "local", To: "+12025550123", From: "Test", Body: "Code 482193"}, "api")
	if err := s.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Delete(ctx, m.ID); err != nil || !ok {
		t.Fatal(err)
	}
	if ok, err := s.Delete(ctx, m.ID); err != nil || ok {
		t.Fatal("repeated delete", err)
	}
	key := access.Key{ID: "key_idempotent", Name: "CI", ProjectID: "default", Permissions: []string{"messages:read"}, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateAPIKey(ctx, key, "key-hash"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if ok, err := s.RevokeAPIKey(ctx, key.ID); err != nil || !ok {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	events, err := s.ListAudit(ctx, audit.Filter{ProjectID: "default", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("expected create/delete/key create/single revoke: %+v", events)
	}
	for _, e := range events {
		if e.Actor.ID != "key_actor" || e.Outcome != "committed" {
			t.Fatal("actor/result lost")
		}
	}
	first, err := s.ListAudit(ctx, audit.Filter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.ListAudit(ctx, audit.Filter{Before: first[1].ID, Limit: 2})
	if err != nil || len(next) != 2 || next[0].ID >= first[1].ID {
		t.Fatal("cursor overlapped", err)
	}
}
