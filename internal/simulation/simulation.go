package simulation

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
)

type Scenario struct {
	ID             string `json:"id"`
	Inbox          string `json:"inbox"`
	Name           string `json:"name"`
	Prefix         string `json:"prefix"`
	Outcome        string `json:"outcome"`
	DelayMS        int    `json:"delay_ms"`
	FailurePercent int    `json:"failure_percent"`
	Seed           string `json:"seed"`
	RejectStatus   int    `json:"reject_status"`
	RetryAfter     int    `json:"retry_after"`
	WebhookURL     string `json:"webhook_url"`
	WebhookFormat  string `json:"webhook_format"`
}

func (s Scenario) Validate() error {
	if strings.TrimSpace(s.Name) == "" || len(s.Name) > 160 || s.DelayMS < 0 || s.DelayMS > 86400000 || s.FailurePercent < 0 || s.FailurePercent > 100 || len(s.Seed) > 128 || len(s.Prefix) > 16 || s.RetryAfter < 0 || s.RetryAfter > 3600 {
		return errors.New("invalid scenario name, delay, percentage, prefix or retry interval")
	}
	if s.Outcome != "delivered" && s.Outcome != "failed" && s.Outcome != "expired" {
		return errors.New("outcome must be delivered, failed or expired")
	}
	if s.RejectStatus != 0 && s.RejectStatus != 400 && s.RejectStatus != 429 && s.RejectStatus != 503 {
		return errors.New("reject_status must be 0, 400, 429 or 503")
	}
	if s.WebhookFormat != "json" && s.WebhookFormat != "twilio" {
		return errors.New("webhook_format must be json or twilio")
	}
	return ValidateURL(s.WebhookURL)
}
func ValidateURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("webhook must be an HTTP(S) URL without credentials or fragment")
	}
	return nil
}
func (s Scenario) Result(m message.Message) string {
	h := sha256.Sum256([]byte(s.Seed + "\x00" + m.To + "\x00" + m.RunID + "\x00" + m.Body))
	if (int(h[0])*256+int(h[1]))%100 < s.FailurePercent {
		return "failed"
	}
	return s.Outcome
}

type Event struct {
	ID        int64     `json:"id"`
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
	Detail    string    `json:"detail"`
}
type Job struct {
	ID          string    `json:"id"`
	MessageID   string    `json:"message_id"`
	Kind        string    `json:"kind"`
	Due         time.Time `json:"due"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	Payload     string    `json:"payload"`
}
type Transition struct {
	Status string `json:"status"`
	URL    string `json:"url"`
	Format string `json:"format"`
}
type Delivery struct {
	URL     string          `json:"url"`
	Format  string          `json:"format"`
	EventID int64           `json:"event_id"`
	Message message.Message `json:"message"`
}
type Attempt struct {
	ID                int64             `json:"id"`
	JobID             string            `json:"job_id"`
	MessageID         string            `json:"message_id"`
	At                time.Time         `json:"at"`
	URL               string            `json:"url"`
	Request           string            `json:"request"`
	ContentType       string            `json:"content_type"`
	Headers           map[string]string `json:"headers"`
	ResponseTruncated bool              `json:"response_truncated"`
	Response          string            `json:"response"`
	Status            int               `json:"status"`
	Error             string            `json:"error"`
	DurationMS        int64             `json:"duration_ms"`
}
type Repository interface {
	PutScenario(context.Context, Scenario) error
	GetScenario(context.Context, string) (Scenario, error)
	ListScenarios(context.Context, string) ([]Scenario, error)
	DeleteScenario(context.Context, string) error
	Schedule(context.Context, message.Message, []Job) error
	NextJob(context.Context, time.Time) (Job, error)
	CompleteTransition(context.Context, Job, time.Time) (message.Message, error)
	FinishWebhook(context.Context, Job, Attempt, bool, time.Time) error
	Events(context.Context, string) ([]Event, error)
	Attempts(context.Context, string) ([]Attempt, error)
	RetryWebhook(context.Context, string) (bool, error)
}
