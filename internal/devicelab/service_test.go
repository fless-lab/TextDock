package devicelab_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/devicelab"
	"github.com/fless-lab/TextDock/internal/storage"
)

type runner struct {
	injections [][]string
	outcome    string
	err        error
	boot       string
}

func (r *runner) Run(ctx context.Context, args ...string) (string, error) {
	switch {
	case reflect.DeepEqual(args, []string{"version"}):
		return "Android Debug Bridge test", nil
	case reflect.DeepEqual(args, []string{"devices", "-l"}):
		return "List of devices attached\nemulator-5554\tdevice model:Test\nphysical\tdevice\nemulator-5556\toffline\n", nil
	case len(args) == 5 && args[2] == "shell":
		if args[4] == "gsm.sim.state" {
			return "LOADED", nil
		}
		if r.boot != "" {
			return r.boot, nil
		}
		return "1", nil
	case len(args) == 5 && args[3] == "avd":
		return "Test_AVD\nOK", nil
	case len(args) == 6 && args[3] == "sms":
		r.injections = append(r.injections, append([]string(nil), args...))
		return r.outcome, r.err
	default:
		return "", errors.New("unexpected command")
	}
}

func TestInjectionScopePersistenceAndIdempotency(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "lab.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{outcome: "OK\r\n"}
	s := &devicelab.Service{Config: devicelab.Config{Enabled: true}, Runner: r, Store: db}
	in := devicelab.Input{Serial: "emulator-5554", To: "+12025550123", From: "+12025550100", Body: "Code 482193\n\n@login.example #482193", RunID: "run-1", IdempotencyKey: "intent-1"}
	result, err := s.Inject(ctx, in)
	if err != nil || result.Injection.Status != "injected" || result.Message.Source != "emulator" || result.Message.Mode != "simulate" {
		t.Fatalf("injection: %+v %v", result, err)
	}
	if len(r.injections) != 1 || !reflect.DeepEqual(r.injections[0][:5], []string{"-s", "emulator-5554", "emu", "sms", "pdu"}) || strings.ContainsAny(r.injections[0][5], "\r\n") {
		t.Fatalf("unsafe command: %+v", r.injections)
	}
	second, err := s.Inject(ctx, in)
	if err != nil || !second.Replayed || second.Message.ID != result.Message.ID || len(r.injections) != 1 {
		t.Fatalf("duplicate injection: %+v %v", second, err)
	}
	r.boot = "0"
	if replay, err := s.Inject(ctx, in); err != nil || !replay.Replayed {
		t.Fatalf("replay depended on emulator availability: %+v %v", replay, err)
	}
	r.boot = "1"
	in.Body = "changed"
	if _, err := s.Inject(ctx, in); !errors.Is(err, devicelab.ErrConflict) {
		t.Fatalf("conflicting key: %v", err)
	}
	in.Body = result.Message.Body
	db.Close()
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s.Store = db
	log, err := db.InjectionHistory(ctx, "local", 100)
	if err != nil || len(log) != 1 || log[0].Status != "injected" {
		t.Fatalf("history after restart: %+v %v", log, err)
	}
	other, err := db.InjectionHistory(ctx, "different-inbox", 100)
	if err != nil || len(other) != 0 {
		t.Fatal("history leaked across inboxes")
	}
	db.Delete(ctx, result.Message.ID)
	if _, err := s.Inject(ctx, in); !errors.Is(err, devicelab.ErrConflict) {
		t.Fatalf("deleted intent was replayed: %v", err)
	}
}

func TestFailuresDoNotBecomeDeliveryOrImplicitRetries(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := &runner{outcome: "KO: modem unavailable"}
	s := &devicelab.Service{Config: devicelab.Config{Enabled: true}, Runner: r, Store: db}
	in := devicelab.Input{Serial: "physical", To: "+12025550123", From: "+12025550100", Body: "Code 123456"}
	if _, err := s.Inject(ctx, in); !errors.Is(err, devicelab.ErrInput) || len(r.injections) != 0 {
		t.Fatal("physical target reached SMS command")
	}
	in.Serial = "emulator-5556"
	if _, err := s.Inject(ctx, in); !errors.Is(err, devicelab.ErrTarget) {
		t.Fatal("offline target accepted")
	}
	in.Serial = "emulator-5554"
	r.boot = "0"
	if _, err := s.Inject(ctx, in); !errors.Is(err, devicelab.ErrTarget) {
		t.Fatal("unbooted target accepted")
	}
	r.boot = "1"
	failed, err := s.Inject(ctx, in)
	if err != nil || failed.Message.Status != "failed" {
		t.Fatalf("console rejection: %+v %v", failed, err)
	}
	r.outcome = ""
	r.err = context.DeadlineExceeded
	in.IdempotencyKey = "timeout"
	unknown, err := s.Inject(ctx, in)
	if err != nil || unknown.Message.Status != "unknown" {
		t.Fatalf("timeout: %+v %v", unknown, err)
	}
	before := len(r.injections)
	s.Inject(ctx, in)
	if len(r.injections) != before {
		t.Fatal("uncertain injection was automatically repeated")
	}
	m := unknown.Message
	m.ID = "recovery-case"
	m.IdempotencyKey = "recovery"
	m.Status = "injecting"
	if _, err := db.ReserveInjection(ctx, m, in.Serial); err != nil {
		t.Fatal(err)
	}
	if count, err := db.RecoverInjections(ctx, time.Now().Add(2*time.Minute)); err != nil || count != 1 {
		t.Fatalf("recovery: %d %v", count, err)
	}
	recovered, _ := db.Get(ctx, m.ID)
	if recovered.Status != "unknown" {
		t.Fatalf("recovery status: %s", recovered.Status)
	}
}
