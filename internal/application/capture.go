package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/relay"
	"github.com/fless-lab/TextDock/internal/simulation"
)

type Rejected struct{ Status, RetryAfter int }

var ErrRequest = errors.New("invalid simulation request")

func invalid(text string) error  { return fmt.Errorf("%w: %s", ErrRequest, text) }
func (r Rejected) Error() string { return "request rejected by simulation scenario" }

type Capture struct {
	Messages   message.Repository
	Simulation simulation.Repository
	OTPPattern *regexp.Regexp
	Changed    func(string, string, string)
	Relay      *relay.Service
}

func (s Capture) Send(ctx context.Context, in message.Input, source string) (message.Message, error) {
	if in.Mode == "relay" && (in.ScenarioID != "" || in.Direction == "inbound" || in.CallbackURL != "") {
		return message.Message{}, invalid("relay cannot use a simulation scenario, inbound direction or arbitrary callback URL")
	}
	m, err := message.New(in, source)
	if err != nil {
		return m, err
	}
	if s.OTPPattern != nil {
		m.Analysis.OTP = ""
		if match := s.OTPPattern.FindStringSubmatch(m.Body); len(match) > 1 && len(match[1]) <= 64 {
			m.Analysis.OTP = match[1]
		}
	}
	if m.Mode == "relay" {
		if s.Relay == nil || s.Relay.Config.Driver == "" {
			return m, invalid("real SMS relay is disabled")
		}
		m, err = s.Relay.Queue(ctx, m)
		if err == nil && s.Changed != nil {
			s.Changed(m.Inbox, m.To, m.RunID)
		}
		return m, err
	}
	if in.ScenarioID != "" {
		m.Mode = "simulate"
	}
	if in.Direction == "inbound" {
		m.Direction = "inbound"
		m.Mode = "simulate"
	}
	if m.Mode != "capture" && m.Mode != "simulate" {
		return m, message.ErrInvalid
	}
	if err := simulation.ValidateURL(in.CallbackURL); err != nil {
		return m, invalid(err.Error())
	}
	if m.Mode == "capture" {
		if in.CallbackURL != "" {
			return m, invalid("callbacks require simulation mode")
		}
		err = s.Messages.Save(ctx, m)
	} else {
		jobs := make([]simulation.Job, 0)
		if m.Direction == "inbound" {
			m.Status = "received"
			if in.CallbackURL != "" {
				payload, _ := json.Marshal(simulation.Delivery{URL: in.CallbackURL, Format: "json", Message: m})
				jobs = append(jobs, simulation.Job{ID: "job_" + rand.Text(), MessageID: m.ID, Kind: "webhook", Due: m.CreatedAt, Payload: string(payload)})
			}
		} else {
			scenario, err := s.Simulation.GetScenario(ctx, in.ScenarioID)
			if errors.Is(err, sql.ErrNoRows) {
				return m, invalid("choose an existing simulation scenario")
			}
			if err != nil {
				return m, err
			}
			if scenario.Inbox != m.Inbox || !strings.HasPrefix(m.To, scenario.Prefix) {
				return m, invalid("scenario does not match this inbox or recipient prefix")
			}
			if scenario.RejectStatus != 0 {
				return m, Rejected{scenario.RejectStatus, max(1, scenario.RetryAfter)}
			}
			m.Status = "queued"
			callback := scenario.WebhookURL
			if in.CallbackURL != "" {
				callback = in.CallbackURL
			}
			format := scenario.WebhookFormat
			if source == "twilio" {
				format = "twilio"
			}
			delay := max(scenario.DelayMS, 2)
			for i, status := range []string{"sent", scenario.Result(m)} {
				payload, _ := json.Marshal(simulation.Transition{Status: status, URL: callback, Format: format})
				due := m.CreatedAt.Add(time.Duration(delay) * time.Millisecond)
				if i == 0 {
					due = m.CreatedAt.Add(time.Duration(max(1, delay/2)) * time.Millisecond)
				}
				jobs = append(jobs, simulation.Job{ID: "job_" + rand.Text(), MessageID: m.ID, Kind: "transition", Due: due, Payload: string(payload)})
			}
		}
		err = s.Simulation.Schedule(ctx, m, jobs)
	}
	if err == nil && s.Changed != nil {
		s.Changed(m.Inbox, m.To, m.RunID)
	}
	return m, err
}
