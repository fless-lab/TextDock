package access

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"golang.org/x/crypto/argon2"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var ErrLogin = errors.New("invalid username or password")
var ErrSession = errors.New("user session expired or revoked")
var ErrUser = errors.New("user or project unavailable")
var ErrUsername = errors.New("username already exists")
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func ValidUsername(s string) bool       { return usernamePattern.MatchString(s) }
func ValidPassword(s string) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= 12 && n <= 128
}

// Fixed, bounded Argon2id parameters; database contents cannot request arbitrary cost.
const passwordPrefix = "$argon2id$v=19$m=65536,t=2,p=1$"
const DummyPasswordHash = passwordPrefix + "AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func HashPassword(password string) (string, error) {
	if !ValidPassword(password) {
		return "", errors.New("password must contain 12–128 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}
func VerifyPassword(password, encoded string) bool {
	if !ValidPassword(password) || !strings.HasPrefix(encoded, passwordPrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, passwordPrefix), "$")
	if len(parts) != 2 {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[0])
	want, e2 := base64.RawStdEncoding.DecodeString(parts[1])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
}
type Membership struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}
type UserSession struct {
	ID          string       `json:"id"`
	User        User         `json:"user"`
	CreatedAt   time.Time    `json:"created_at"`
	ExpiresAt   time.Time    `json:"expires_at"`
	Memberships []Membership `json:"memberships"`
}
type SessionSummary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s UserSession) Role(project string) string {
	for _, m := range s.Memberships {
		if m.ProjectID == project {
			return m.Role
		}
	}
	return ""
}
func RolePermissions(role string) []string {
	switch role {
	case "viewer":
		return []string{"messages:read"}
	case "member":
		return []string{"messages:read", "messages:write"}
	case "admin":
		return []string{"messages:read", "messages:write", "messages:delete"}
	}
	return nil
}

type Users interface {
	CreateUser(context.Context, User, string) error
	ListUsers(context.Context) ([]User, error)
	UserCredential(context.Context, string) (User, string, error)
	CreateUserSession(context.Context, UserSession, string, string) error
	UserSessionByHash(context.Context, string) (UserSession, error)
	UserSessions(context.Context, string) ([]SessionSummary, error)
	RevokeUserSession(context.Context, string, string) (bool, error)
	SetUserDisabled(context.Context, string, bool) ([]string, error)
	SetUserPassword(context.Context, string, string, string) ([]string, error)
	ProjectMembers(context.Context, string) ([]Membership, error)
	SetMembership(context.Context, string, string, string) ([]string, error)
	RemoveMembership(context.Context, string, string) ([]string, error)
}

// Per-account and global login budgets bound both brute-force attempts and KDF work.
// These in-memory limits reset on restart and allow four concurrent 64 MiB KDFs.
type LoginGuard struct {
	mu            sync.Mutex
	start         time.Time
	total, active int
	accounts      map[string]int
}

func (g *LoginGuard) Acquire(account string, limited bool) (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if now.Sub(g.start) >= time.Minute {
		g.start = now
		g.total = 0
		g.accounts = make(map[string]int)
	}
	if g.active >= 4 || (limited && (g.total >= 60 || g.accounts[account] >= 10)) {
		return nil, false
	}
	if limited {
		g.total++
		g.accounts[account]++
	}
	g.active++
	return func() { g.mu.Lock(); g.active--; g.mu.Unlock() }, true
}
