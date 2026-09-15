package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/storage"
)

func setup(t *testing.T, token string) (*httptest.Server, *storage.SQLite) {
	t.Helper()
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: store, Devices: store, Workspaces: store, Simulation: store, Token: token, Version: "test", UI: fstest.MapFS{"index.html": {Data: []byte("TextDock")}}}
	s.Keys = store
	server := httptest.NewServer(s.Handler())
	t.Cleanup(func() { server.Close(); store.Close() })
	return server, store
}

func request(t *testing.T, server *httptest.Server, method, path, body, token string) *http.Response {
	t.Helper()
	r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestCaptureContract(t *testing.T) {
	s, _ := setup(t, "")
	r := request(t, s, "POST", "/api/v1/messages", `{"to":"+33612345678","from":"Acme","body":"Your code is 482193","run_id":"test-1"}`, "")
	if r.StatusCode != 201 {
		t.Fatalf("status: %d", r.StatusCode)
	}
	var m message.Message
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m.Analysis.OTP != "482193" || m.Status != "captured" {
		t.Fatalf("capture: %+v", m)
	}
	r = request(t, s, "GET", "/api/v1/otp?to=%2B33612345678&run_id=test-1", "", "")
	if r.StatusCode != 200 {
		t.Fatalf("otp: %d", r.StatusCode)
	}
	r = request(t, s, "GET", "/api/v1/otp?to=%2B33612345678&run_id=other-run", "", "")
	if r.StatusCode != 404 {
		t.Fatalf("stale OTP leaked: %d", r.StatusCode)
	}
	r = request(t, s, "DELETE", "/api/v1/messages/"+m.ID, "", "")
	if r.StatusCode != 204 {
		t.Fatalf("delete: %d", r.StatusCode)
	}
}

func TestInvalidRequests(t *testing.T) {
	s, _ := setup(t, "")
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/v1/messages", `{}`, 400},
		{"POST", "/api/v1/messages", `{"unsupported":true}`, 400},
		{"POST", "/api/v1/messages", `{} {}`, 400},
		{"POST", "/api/v1/messages", strings.Repeat("x", 33000), 400},
		{"GET", "/api/v1/messages?limit=201", "", 400},
		{"GET", "/api/v1/messages?since=yesterday", "", 400},
		{"GET", "/api/v1/otp?to=%2B33612345678", "", 400},
		{"GET", "/api/v1/otp?to=%2B33612345678&run_id=x&timeout=31", "", 400},
		{"GET", "/api/v1/nope", "", 404},
	} {
		r := request(t, s, tc.method, tc.path, tc.body, "")
		if r.StatusCode != tc.status {
			t.Errorf("%s %s: %d want %d", tc.method, tc.path, r.StatusCode, tc.status)
		}
	}
}

func TestAuthenticationAndBrowserBoundaries(t *testing.T) {
	s, _ := setup(t, "test-token-12345678")
	for _, token := range []string{"", "wrong"} {
		if r := request(t, s, "GET", "/api/v1/messages", "", token); r.StatusCode != 401 {
			t.Fatalf("unauthorized: %d", r.StatusCode)
		}
	}
	if r := request(t, s, "GET", "/api/v1/messages", "", "test-token-12345678"); r.StatusCode != 200 {
		t.Fatalf("authorized: %d", r.StatusCode)
	}
	r, _ := http.NewRequest("GET", s.URL+"/api/v1/info", nil)
	r.Header.Set("Origin", "https://unrelated.example")
	r.Header.Set("Authorization", "Bearer test-token-12345678")
	resp, err := s.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin: %d", resp.StatusCode)
	}
	local, _ := setup(t, "")
	r, _ = http.NewRequest("GET", local.URL+"/api/v1/info", nil)
	r.Host = "rebound.example"
	resp, err = local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("rebinding: %d", resp.StatusCode)
	}
}

func TestWaitForAsynchronousOTP(t *testing.T) {
	s, store := setup(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errors := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		m, _ := message.New(message.Input{To: "+33612345678", From: "Acme", Body: "Code 918273", RunID: "async-run"}, "api")
		errors <- store.Save(ctx, m)
	}()
	r := request(t, s, "GET", "/api/v1/otp?to=%2B33612345678&run_id=async-run&timeout=2", "", "")
	if r.StatusCode != 200 {
		t.Fatalf("wait: %d", r.StatusCode)
	}
	data, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(data), "918273") {
		t.Fatalf("wrong OTP: %s", data)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}

func TestTwilioSubsetAndUnsupportedParameters(t *testing.T) {
	s, _ := setup(t, "test-token-12345678")
	for _, callback := range []bool{false, true} {
		form := url.Values{"To": {"+33612345678"}, "From": {"Acme"}, "Body": {"Your code is 482193"}}
		if callback {
			form.Set("StatusCallback", "https://example.com/hook")
		}
		r, _ := http.NewRequest("POST", s.URL+"/2010-04-01/Accounts/ACtest/Messages.json", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.SetBasicAuth("ACtest", "test-token-12345678")
		resp, err := s.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		err = json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if callback {
			if resp.StatusCode != 422 {
				t.Fatalf("unsupported callback: %d", resp.StatusCode)
			}
		} else if resp.StatusCode != 201 || data["status"] != "queued" || !strings.HasPrefix(data["sid"].(string), "SM") {
			t.Fatalf("Twilio response: %d %+v", resp.StatusCode, data)
		}
	}
}
