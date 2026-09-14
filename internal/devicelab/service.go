package devicelab

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
)

type Injection struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	Inbox     string    `json:"inbox"`
	Serial    string    `json:"serial"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail"`
	Output    string    `json:"output"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Input struct {
	Serial         string `json:"serial"`
	Inbox          string `json:"inbox,omitempty"`
	To             string `json:"to"`
	From           string `json:"from"`
	Body           string `json:"body"`
	RunID          string `json:"run_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}
type Result struct {
	Message   message.Message `json:"message"`
	Injection Injection       `json:"injection"`
	Replayed  bool            `json:"replayed"`
}
type Repository interface {
	LookupInjection(context.Context, message.Message, string) (Result, error)
	ReserveInjection(context.Context, message.Message, string) (Result, error)
	FinishInjection(context.Context, string, string, string, string) (Result, error)
	InjectionHistory(context.Context, string, int) ([]Injection, error)
	RecoverInjections(context.Context, time.Time) (int, error)
}
type Service struct {
	Config  Config
	Runner  Runner
	Store   Repository
	Changed func(string, string, string)
	Resync  func()
}

func (s *Service) Status(ctx context.Context) Status {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	status := Status{Enabled: s.Config.Enabled, Devices: []Device{}}
	if !s.Config.Enabled {
		return status
	}
	version, err := s.Runner.Run(ctx, "version")
	if err != nil {
		status.Error = fmt.Sprintf("%s: %v", ErrUnavailable, err)
		return status
	}
	status.Version = version
	devices, err := s.Runner.Run(ctx, "devices", "-l")
	if err != nil {
		status.Error = fmt.Sprintf("ADB device discovery failed: %v", err)
		return status
	}
	status.Available = true
	status.Devices = ParseDevices(devices)
	return status
}

func (s *Service) Inject(ctx context.Context, in Input) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !s.Config.Enabled {
		return Result{}, ErrDisabled
	}
	if !EmulatorSerial(in.Serial) || !message.ValidRecipient(in.From) {
		return Result{}, fmt.Errorf("%w: select an emulator serial and an E.164-shaped sender", ErrInput)
	}
	body := strings.ReplaceAll(in.Body, "\r\n", "\n")
	encoded, err := ConsoleText(body)
	if err != nil {
		return Result{}, err
	}
	m, err := message.New(message.Input{Inbox: in.Inbox, To: in.To, From: in.From, Body: body, RunID: in.RunID, IdempotencyKey: in.IdempotencyKey, Mode: "simulate", Direction: "inbound"}, "emulator")
	if err != nil {
		return Result{}, fmt.Errorf("%w: invalid message fields", ErrInput)
	}
	if m.IdempotencyKey != "" {
		previous, err := s.Store.LookupInjection(ctx, m, in.Serial)
		if err != nil || previous.Replayed {
			return previous, err
		}
	}
	status := s.Status(ctx)
	if !status.Available {
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, status.Error)
	}
	selected := false
	for _, d := range status.Devices {
		if d.Serial == in.Serial && d.CanInject {
			selected = true
			break
		}
	}
	if !selected {
		return Result{}, ErrTarget
	}
	boot, err := s.Runner.Run(ctx, "-s", in.Serial, "shell", "getprop", "sys.boot_completed")
	if err != nil || strings.TrimSpace(boot) != "1" {
		return Result{}, ErrTarget
	}
	sim, err := s.Runner.Run(ctx, "-s", in.Serial, "shell", "getprop", "gsm.sim.state")
	if err != nil || (!strings.Contains(sim, "READY") && !strings.Contains(sim, "LOADED")) {
		return Result{}, fmt.Errorf("%w: virtual SIM is not ready", ErrTarget)
	}
	name, err := s.Runner.Run(ctx, "-s", in.Serial, "emu", "avd", "name")
	if err != nil || strings.Contains(name, "KO:") || !hasOK(name) {
		return Result{}, ErrTarget
	}
	m.Status = "injecting"
	result, err := s.Store.ReserveInjection(ctx, m, in.Serial)
	if err != nil || result.Replayed {
		return result, err
	}
	if s.Changed != nil {
		s.Changed(m.Inbox, m.To, m.RunID)
	}
	output, commandErr := s.Runner.Run(ctx, "-s", in.Serial, "emu", "sms", "send", m.From, encoded)
	state, detail := "unknown", "ADB outcome uncertain; automatic reinjection is disabled"
	if strings.Contains(output, "KO:") {
		state, detail = "failed", "Emulator console rejected the SMS"
	} else if commandErr == nil && hasOK(output) {
		state, detail = "injected", "Emulator console accepted the simulated incoming SMS"
	}
	if len(output) > 4096 {
		output = output[:4096]
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	result, err = s.Store.FinishInjection(finish, m.ID, state, detail, output)
	if err == nil && s.Changed != nil {
		s.Changed(m.Inbox, m.To, m.RunID)
	}
	return result, err
}
func hasOK(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "OK" {
			return true
		}
	}
	return false
}

func (s *Service) Recover(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		n, err := s.Store.RecoverInjections(ctx, time.Now().UTC())
		if err != nil && ctx.Err() == nil {
			slog.Error("device lab recovery", "error", err)
		}
		if err == nil && n > 0 && s.Resync != nil {
			s.Resync()
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
