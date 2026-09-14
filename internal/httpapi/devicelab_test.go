package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/fless-lab/TextDock/internal/devicelab"
	"github.com/fless-lab/TextDock/internal/storage"
)

type noCommands struct{ calls int }

func (r *noCommands) Run(context.Context, ...string) (string, error) {
	r.calls++
	return "", errors.New("fixture must not execute")
}

func TestDeviceLabAuthorizationAndRejectedInputs(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runner := &noCommands{}
	lab := &devicelab.Service{Config: devicelab.Config{Enabled: true}, Runner: runner, Store: db}
	s := httptest.NewServer((&Server{Store: db, Workspaces: db, Devices: db, Simulation: db, Lab: lab, Token: "lab-admin-token", UI: fstest.MapFS{}}).Handler())
	defer s.Close()
	for _, credential := range []string{"", "wrong", "td_device_fake", "td_gateway_fake"} {
		response := request(t, s, "POST", "/api/v1/lab/injections", `{}`, credential)
		if response.StatusCode != 401 && response.StatusCode != 403 {
			t.Fatalf("unauthorized lab access: %d", response.StatusCode)
		}
	}
	for _, body := range []string{
		`{"serial":"physical","to":"+12025550123","from":"+12025550100","body":"hello"}`,
		`{"serial":"emulator-5554\nkill","to":"+12025550123","from":"+12025550100","body":"hello"}`,
		`{"serial":"emulator-5554","to":"+12025550123","from":"+12025550100","body":"hello\u0000"}`,
		`{"serial":"emulator-5554","command":"shell"}`,
	} {
		if response := request(t, s, "POST", "/api/v1/lab/injections", body, "lab-admin-token"); response.StatusCode != 400 {
			t.Fatalf("invalid injection accepted: %d", response.StatusCode)
		}
	}
	if runner.calls != 0 {
		t.Fatal("rejected requests reached ADB")
	}
	if response := request(t, s, "GET", "/api/v1/lab/history?limit=201", "", "lab-admin-token"); response.StatusCode != 400 {
		t.Fatal("unbounded history request accepted")
	}
}
