// Package access defines scoped machine credentials, separate from the operator token.
package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("API key expired, revoked or invalid")
var ErrScope = errors.New("invalid API key scope")

type Key struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	ProjectID   string     `json:"project_id"`
	InboxID     string     `json:"inbox_id,omitempty"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

func (k Key) Validate(now time.Time) error {
	if strings.TrimSpace(k.Name) == "" || utf8.RuneCountInString(k.Name) > 80 || k.ProjectID == "" || len(k.ProjectID) > 128 || len(k.InboxID) > 128 {
		return ErrScope
	}
	if !k.ExpiresAt.After(now) || k.ExpiresAt.After(now.Add(365*24*time.Hour)) {
		return errors.New("expires_at must be in the future, within 365 days")
	}
	if len(k.Permissions) == 0 || len(k.Permissions) > 3 {
		return errors.New("select 1–3 permissions")
	}
	seen := map[string]bool{}
	for _, p := range k.Permissions {
		if !slices.Contains([]string{"messages:read", "messages:write", "messages:delete"}, p) || seen[p] {
			return errors.New("invalid or duplicate permission")
		}
		seen[p] = true
	}
	return nil
}
func (k Key) Allows(permission string) bool { return slices.Contains(k.Permissions, permission) }
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type Repository interface {
	CreateAPIKey(context.Context, Key, string) error
	ListAPIKeys(context.Context) ([]Key, error)
	APIKeyByHash(context.Context, string) (Key, error)
	RevokeAPIKey(context.Context, string) (bool, error)
	KeyInboxAllowed(context.Context, Key, string) (bool, error)
}
