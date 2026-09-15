// Package push manages opt-in, generic Web Push alerts for paired phone sessions.
package push

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrSubscription = errors.New("invalid push subscription")
var ErrConflict = errors.New("push endpoint is registered with different encryption keys")
var ErrSession = errors.New("phone session expired or revoked")

type Config struct {
	Enabled      bool
	PublicURL    string
	Subject      string
	AllowedHosts []string
}

func FromEnv(publicURL string) (Config, error) {
	c := Config{PublicURL: publicURL}
	if raw := os.Getenv("TEXTDOCK_PUSH_ENABLED"); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("TEXTDOCK_PUSH_ENABLED must be a boolean")
		}
		c.Enabled = enabled
	}
	if !c.Enabled {
		return c, nil
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		return c, errors.New("Web Push requires TEXTDOCK_PUBLIC_URL")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return c, errors.New("Web Push requires an HTTPS origin (or loopback HTTP for tests)")
	}
	c.Subject = os.Getenv("TEXTDOCK_PUSH_SUBJECT")
	if c.Subject == "" {
		c.Subject = publicURL
	}
	if strings.HasPrefix(c.Subject, "mailto:") {
		c.Subject = strings.TrimPrefix(c.Subject, "mailto:")
	}
	if strings.HasPrefix(c.Subject, "https:") {
		contact, err := url.Parse(c.Subject)
		if err != nil || contact.Host == "" || contact.User != nil {
			return c, errors.New("invalid HTTPS push contact URL")
		}
	} else {
		contact, err := mail.ParseAddress(c.Subject)
		if err != nil {
			return c, errors.New("TEXTDOCK_PUSH_SUBJECT must be an HTTPS contact URL or email address")
		}
		c.Subject = contact.Address
	}
	for _, host := range strings.Split(os.Getenv("TEXTDOCK_PUSH_ALLOWED_HOSTS"), ",") {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if strings.ContainsAny(host, "/?#@*") {
			return c, errors.New("push allowed hosts must be exact hostnames")
		}
		c.AllowedHosts = append(c.AllowedHosts, host)
	}
	return c, nil
}

type Keys struct {
	Public  string
	Private string
}
type Subscription struct {
	Endpoint       string   `json:"endpoint"`
	ExpirationTime *float64 `json:"expirationTime,omitempty"`
	Keys           struct {
		Auth   string `json:"auth"`
		P256dh string `json:"p256dh"`
	} `json:"keys"`
}

func (c Config) Validate(s *Subscription) error {
	u, err := url.Parse(s.Endpoint)
	if err != nil || len(s.Endpoint) > 2048 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ErrSubscription
	}
	host := strings.ToLower(u.Hostname())
	allowed := host == "fcm.googleapis.com" || host == "fcm-push.googleapis.com" || host == "updates.push.services.mozilla.com" || host == "web.push.apple.com"
	for _, domain := range []string{"push.services.mozilla.com", "push.apple.com", "notify.windows.com"} {
		if strings.HasSuffix(host, "."+domain) {
			allowed = true
		}
	}
	for _, candidate := range c.AllowedHosts {
		if host == candidate {
			allowed = true
		}
	}
	if u.Port() != "" && u.Port() != "443" {
		custom := false
		for _, candidate := range c.AllowedHosts {
			if host == candidate {
				custom = true
			}
		}
		if !custom {
			return ErrSubscription
		}
	}
	if !allowed {
		return errors.New("push endpoint host is not supported; the server operator can add an exact trusted host")
	}
	auth, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.Auth, "="))
	if err != nil || len(auth) != 16 {
		return ErrSubscription
	}
	pub, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.P256dh, "="))
	if err != nil || len(pub) != 65 || pub[0] != 4 {
		return ErrSubscription
	}
	if _, err := ecdh.P256().NewPublicKey(pub); err != nil {
		return ErrSubscription
	}
	s.Keys.Auth, s.Keys.P256dh = base64.RawURLEncoding.EncodeToString(auth), base64.RawURLEncoding.EncodeToString(pub)
	return nil
}

type State struct {
	Subscribed   bool       `json:"subscribed"`
	Generation   string     `json:"generation,omitempty"`
	EndpointHost string     `json:"endpoint_host,omitempty"`
	Status       string     `json:"status"`
	HTTPStatus   int        `json:"http_status"`
	Detail       string     `json:"detail,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	Queued       bool       `json:"queued"`
}
type Job struct {
	ID           int64
	DeviceID     string
	Generation   string
	Kind         string
	Attempts     int
	Subscription Subscription
}
type Outcome struct {
	State      string
	HTTPStatus int
	Detail     string
	RetryAfter time.Duration
}
type Repository interface {
	PushKeys(context.Context) (Keys, error)
	SetPushEnabled(bool)
	PutPushSubscription(context.Context, string, Subscription) (State, error)
	DeletePushSubscription(context.Context, string) error
	PushState(context.Context, string) (State, error)
	QueuePushTest(context.Context, string) error
	ClaimPush(context.Context, time.Time) (Job, error)
	PushJobActive(context.Context, Job) (bool, error)
	FinishPush(context.Context, Job, Outcome) error
}
