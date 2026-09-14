// Package relay implements explicitly enabled real SMS dispatch. It never
// retries an uncertain send automatically.
package relay

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
)

var ErrDisabled = errors.New("real SMS relay is disabled")
var ErrLimit = errors.New("relay sending limit reached; try again after the current minute")
var ErrRecipient = errors.New("recipient is not in TEXTDOCK_RELAY_ALLOWED_TO")
var ErrCredential = errors.New("gateway credential is invalid or expired")
var ErrLease = errors.New("relay lease is invalid or the result conflicts with the recorded state")
var ErrIdempotency = errors.New("idempotency key was already used with different data or a deleted message")

type Config struct {
	Driver     string
	AccountSID string
	AuthToken  string
	Endpoint   string
	PublicURL  string
	Limit      int
	TTL        time.Duration
	AllowedTo  []string
}

func FromEnv(publicURL string) (Config, error) {
	c := Config{Driver: os.Getenv("TEXTDOCK_RELAY_DRIVER"), AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"), AuthToken: os.Getenv("TWILIO_AUTH_TOKEN"), Endpoint: os.Getenv("TEXTDOCK_RELAY_ENDPOINT"), PublicURL: strings.TrimSuffix(publicURL, "/"), Limit: 10, TTL: 10 * time.Minute}
	if c.Driver == "" {
		return c, nil
	}
	if c.Driver != "twilio" && c.Driver != "android" {
		return c, errors.New("TEXTDOCK_RELAY_DRIVER must be twilio or android")
	}
	if value := os.Getenv("TEXTDOCK_RELAY_TTL"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < time.Second || duration > 24*time.Hour {
			return c, errors.New("TEXTDOCK_RELAY_TTL must be between 1s and 24h")
		}
		c.TTL = duration
	}
	if value := os.Getenv("TEXTDOCK_RELAY_LIMIT"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 1000 {
			return c, errors.New("TEXTDOCK_RELAY_LIMIT must be 1–1000 sends per minute")
		}
		c.Limit = n
	}
	for _, to := range strings.Split(os.Getenv("TEXTDOCK_RELAY_ALLOWED_TO"), ",") {
		if to = strings.TrimSpace(to); to != "" {
			if !message.ValidRecipient(to) {
				return c, errors.New("TEXTDOCK_RELAY_ALLOWED_TO must contain E.164-shaped numbers")
			}
			c.AllowedTo = append(c.AllowedTo, to)
		}
	}
	if c.Endpoint == "" {
		c.Endpoint = "https://api.twilio.com"
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return c, errors.New("TEXTDOCK_RELAY_ENDPOINT must be an HTTP(S) origin")
	}
	if c.Driver == "twilio" && (c.AccountSID == "" || c.AuthToken == "") {
		return c, errors.New("Twilio relay requires TWILIO_ACCOUNT_SID and TWILIO_AUTH_TOKEN")
	}
	return c, nil
}

type Job struct {
	Inbox      string    `json:"inbox,omitempty"`
	RunID      string    `json:"run_id,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
	ID         string    `json:"id"`
	MessageID  string    `json:"message_id"`
	Driver     string    `json:"driver"`
	State      string    `json:"state"`
	Owner      string    `json:"owner"`
	ProviderID string    `json:"provider_id"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
	LeaseUntil time.Time `json:"lease_until"`
	LeaseToken string    `json:"lease_token,omitempty"`
	To         string    `json:"to,omitempty"`
	From       string    `json:"from,omitempty"`
	Body       string    `json:"body,omitempty"`
}
type Gateway struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}
type Result struct {
	State      string `json:"state"`
	ProviderID string `json:"provider_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Repository interface {
	EnqueueRelay(context.Context, message.Message, string, int, time.Time) (message.Message, error)
	ClaimRelay(context.Context, string, string, time.Time, int) (Job, error)
	CompleteRelay(context.Context, string, string, string, Result) (message.Message, error)
	RelayReceipt(context.Context, string, Result) (message.Message, error)
	RelayJob(context.Context, string) (Job, error)
	ExpireRelays(context.Context, time.Time) (int, error)
	CreateGateway(context.Context, Gateway, string) error
	GatewayByHash(context.Context, string) (Gateway, error)
	Gateways(context.Context) ([]Gateway, error)
	RevokeGateway(context.Context, string) error
}

type Service struct {
	Store  Repository
	Config Config
}

func (s Service) Queue(ctx context.Context, m message.Message) (message.Message, error) {
	if s.Config.Driver == "" {
		return m, ErrDisabled
	}
	if len(s.Config.AllowedTo) > 0 {
		allowed := false
		for _, to := range s.Config.AllowedTo {
			if m.To == to {
				allowed = true
				break
			}
		}
		if !allowed {
			return m, ErrRecipient
		}
	}
	m.Status = "queued"
	if s.Config.Driver == "android" {
		m.From = "SIM"
	}
	ttl := s.Config.TTL
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	return s.Store.EnqueueRelay(ctx, m, s.Config.Driver, s.Config.Limit, m.CreatedAt.Add(ttl))
}
