package storage

import (
	"context"
	"errors"
	"github.com/fless-lab/TextDock/internal/access"
	"path/filepath"
	"testing"
	"time"
)

func TestUserSessionsPersistAndRejectStalePassword(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "users.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	u := access.User{ID: "user_test", Username: "alice", Name: "Alice", CreatedAt: time.Now()}
	if err := s.CreateUser(ctx, u, "hash-one"); err != nil {
		t.Fatal(err)
	}
	session := access.UserSession{ID: "session_test", User: u, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateUserSession(ctx, session, access.Hash("td_user_test"), "hash-one"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.UserSessionByHash(ctx, access.Hash("td_user_test")); err != nil || got.User.ID != u.ID {
		t.Fatal("session lost", err)
	}
	if _, err := s.SetUserPassword(ctx, u.ID, "hash-two", "hash-one"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserSessionByHash(ctx, access.Hash("td_user_test")); !errors.Is(err, access.ErrSession) {
		t.Fatal("reset did not invalidate session", err)
	}
	session.ID = "session_late"
	if err := s.CreateUserSession(ctx, session, access.Hash("late"), "hash-one"); !errors.Is(err, access.ErrLogin) {
		t.Fatal("stale password issued a session", err)
	}
	if err := s.CreateUserSession(ctx, session, access.Hash("fresh"), "hash-two"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE user_sessions SET expires_at=0`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserSessionByHash(ctx, access.Hash("fresh")); !errors.Is(err, access.ErrSession) {
		t.Fatal("expired session accepted", err)
	}
}
