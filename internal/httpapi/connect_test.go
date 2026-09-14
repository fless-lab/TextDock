package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pairPhone(t *testing.T, s *httptest.Server, admin string) (string, string) {
	t.Helper()
	r := request(t, s, "POST", "/api/v1/pairings", `{"to":"+33612345678","run_id":"paired-run"}`, admin)
	if r.StatusCode != 201 {
		t.Fatalf("pair status: %d", r.StatusCode)
	}
	var pair struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pair); err != nil {
		t.Fatal(err)
	}
	body := `{"code":"` + pair.Code + `","name":"Test phone"}`
	r = request(t, s, "POST", "/connect/v1/claim", body, "")
	if r.StatusCode != 201 {
		t.Fatalf("claim: %d", r.StatusCode)
	}
	var result struct {
		Token  string `json:"token"`
		Device struct {
			ID string `json:"id"`
		} `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if r := request(t, s, "POST", "/connect/v1/claim", body, ""); r.StatusCode != 401 {
		t.Fatalf("replayed pairing: %d", r.StatusCode)
	}
	return result.Token, result.Device.ID
}

func TestDeviceScopeAndReadOnlyBoundaries(t *testing.T) {
	for _, admin := range []string{"", "admin-token-for-tests"} {
		s, _ := setup(t, admin)
		token, id := pairPhone(t, s, admin)
		for _, body := range []string{
			`{"to":"+33612345678","from":"Acme","body":"visible 123456","run_id":"paired-run"}`,
			`{"to":"+33612345678","from":"Acme","body":"other-run secret","run_id":"private-run"}`,
			`{"to":"+33699999999","from":"Acme","body":"other recipient","run_id":"paired-run"}`,
		} {
			if r := request(t, s, "POST", "/api/v1/messages", body, admin); r.StatusCode != 201 {
				t.Fatalf("capture: %d", r.StatusCode)
			}
		}
		r := request(t, s, "GET", "/connect/v1/messages", "", token)
		var list struct {
			Messages []struct {
				Body string `json:"body"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
			t.Fatal(err)
		}
		if len(list.Messages) != 1 || list.Messages[0].Body != "visible 123456" {
			t.Fatalf("scope leakage: %+v", list)
		}
		for _, path := range []string{"/api/v1/messages", "/api/v1/events", "/api/v1/devices", "/connect/v1/messages?to=%2B33699999999", "/connect/v1/messages?run_id=private-run"} {
			if r := request(t, s, "GET", path, "", token); r.StatusCode != 403 {
				t.Errorf("scope bypass %s: %d", path, r.StatusCode)
			}
		}
		if r := request(t, s, "POST", "/connect/v1/messages", `{}`, token); r.StatusCode != 403 {
			t.Fatalf("device write: %d", r.StatusCode)
		}
		if r := request(t, s, "GET", "/connect/v1/messages", "", ""); r.StatusCode != 401 {
			t.Fatalf("anonymous device: %d", r.StatusCode)
		}
		if r := request(t, s, "DELETE", "/api/v1/devices/"+id, "", admin); r.StatusCode != 204 {
			t.Fatalf("revoke: %d", r.StatusCode)
		}
		if r := request(t, s, "GET", "/connect/v1/messages", "", token); r.StatusCode != 401 {
			t.Fatalf("revoked read: %d", r.StatusCode)
		}
	}
}

func TestDeviceStreamSyncScopeAndRevocation(t *testing.T) {
	const admin = "admin-token-for-tests"
	s, _ := setup(t, admin)
	token, id := pairPhone(t, s, admin)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/connect/v1/events", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	events := make(chan string, 5)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "event: ") {
				events <- strings.TrimPrefix(line, "event: ")
			}
		}
		close(events)
	}()
	want := func(name string) {
		t.Helper()
		select {
		case got := <-events:
			if got != name {
				t.Fatalf("event: got %q, want %q", got, name)
			}
		case <-ctx.Done():
			t.Fatal("stream timed out")
		}
	}
	want("sync")
	request(t, s, "POST", "/api/v1/messages", `{"to":"+33699999999","from":"Acme","body":"private","run_id":"paired-run"}`, admin)
	select {
	case got := <-events:
		t.Fatalf("foreign message produced device event: %s", got)
	case <-time.After(50 * time.Millisecond):
	}
	request(t, s, "POST", "/api/v1/messages", `{"to":"+33612345678","from":"Acme","body":"code 123456","run_id":"paired-run"}`, admin)
	want("sync")
	request(t, s, "DELETE", "/api/v1/devices/"+id, "", admin)
	want("revoked")
	select {
	case _, open := <-events:
		if open {
			t.Fatal("revoked stream is still active")
		}
	case <-ctx.Done():
		t.Fatal("revoked stream did not close")
	}
}
