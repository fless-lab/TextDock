package storage

import (
	"context"
	"errors"
	"github.com/fless-lab/TextDock/internal/access"
	"path/filepath"
	"testing"
	"time"
)

func TestAPIKeyPersistenceAndExpiry(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "keys.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	k := access.Key{ID: "key_test", Name: "CI", ProjectID: "default", InboxID: "local", Permissions: []string{"messages:read"}, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour).UTC()}
	hash := access.Hash("td_key_secret")
	if err := s.CreateAPIKey(ctx, k, hash); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.APIKeyByHash(ctx, hash); err != nil || got.ID != k.ID {
		t.Fatal("key lost on restart", err)
	}
	var saved string
	if err := s.db.QueryRow(`SELECT token_hash FROM api_keys`).Scan(&saved); err != nil || saved != hash {
		t.Fatal("wrong stored credential", err)
	}
	if _, err := s.db.Exec(`UPDATE api_keys SET expires_at=0`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.APIKeyByHash(ctx, hash); !errors.Is(err, access.ErrInvalid) {
		t.Fatal("expired key authenticated", err)
	}
	k.ID = "key_bad"
	k.InboxID = "missing"
	if err := s.CreateAPIKey(ctx, k, access.Hash("different")); !errors.Is(err, access.ErrScope) {
		t.Fatal("invalid scope accepted", err)
	}
	k.InboxID = "local"
	k.Permissions = []string{"admin"}
	if err := s.CreateAPIKey(ctx, k, access.Hash("different")); err == nil {
		t.Fatal("unknown permission accepted")
	}
}
