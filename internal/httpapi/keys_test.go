package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/access"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
)

func issueKey(t *testing.T, s *httptest.Server, admin, project, inbox string, permissions ...string) (access.Key, string) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"name": "CI key", "project_id": project, "inbox_id": inbox, "permissions": permissions, "expires_at": time.Now().Add(time.Hour)})
	response := request(t, s, "POST", "/api/v1/keys", string(data), admin)
	if response.StatusCode != 201 {
		b, _ := io.ReadAll(response.Body)
		t.Fatalf("issue key: %d %s", response.StatusCode, b)
	}
	var out struct {
		Key   access.Key `json:"key"`
		Token string     `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Key, out.Token
}
func TestScopedKeysIsolation(t *testing.T) {
	for _, admin := range []string{"", "operator-test-token"} {
		t.Run(fmt.Sprintf("token=%t", admin != ""), func(t *testing.T) {
			s, store := setup(t, admin)
			ctx := context.Background()
			if err := store.CreateProject(ctx, workspace.Project{ID: "a", Name: "A"}, workspace.Inbox{ID: "a1", ProjectID: "a", Name: "A1"}); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateInbox(ctx, workspace.Inbox{ID: "a2", ProjectID: "a", Name: "A2"}); err != nil {
				t.Fatal(err)
			}
			_, writer := issueKey(t, s, admin, "a", "a1", "messages:write")
			_, reader := issueKey(t, s, admin, "a", "a1", "messages:read")
			_, project := issueKey(t, s, admin, "a", "", "messages:read", "messages:write", "messages:delete")
			capture := func(inbox, token string) message.Message {
				t.Helper()
				body := `{"to":"+12025550123","from":"Test","body":"Code 482193","run_id":"ci","inbox":"` + inbox + `"}`
				r := request(t, s, "POST", "/api/v1/messages", body, token)
				if r.StatusCode != 201 {
					t.Fatalf("capture: %d", r.StatusCode)
				}
				var m message.Message
				json.NewDecoder(r.Body).Decode(&m)
				return m
			}
			own := capture("", writer)
			other := capture("local", admin)
			sibling := capture("a2", project)
			for _, tc := range []struct {
				method, path, body, token string
				status                    int
			}{
				{"GET", "/api/v1/messages", "", reader, 200},
				{"GET", "/api/v1/messages?inbox=local", "", reader, 403},
				{"GET", "/api/v1/messages?inbox=a2", "", reader, 403},
				{"GET", "/api/v1/messages", "", project, 400},
				{"GET", "/api/v1/messages?inbox=a2", "", project, 200},
				{"GET", "/api/v1/messages", "", writer, 403},
				{"GET", "/api/v1/export?format=csv&inbox=local", "", reader, 403},
				{"GET", "/api/v1/export?format=csv", "", reader, 200},
				{"GET", "/api/v1/otp?to=%2B12025550123&run_id=ci", "", reader, 200},
				{"GET", "/api/v1/otp?inbox=local&to=%2B12025550123&run_id=ci", "", reader, 403},
				{"GET", "/api/v1/messages/" + other.ID + "/events", "", reader, 404},
				{"GET", "/api/v1/messages/" + own.ID + "/events", "", reader, 200},
				{"PATCH", "/api/v1/messages/" + other.ID, `{"favorite":true}`, project, 404},
				{"PATCH", "/api/v1/messages/" + own.ID, `{"favorite":true}`, writer, 403},
				{"PATCH", "/api/v1/messages/" + own.ID, `{"favorite":true}`, project, 200},
				{"DELETE", "/api/v1/messages/" + other.ID, "", project, 404},
				{"DELETE", "/api/v1/messages/" + own.ID, "", reader, 403},
				{"POST", "/api/v1/messages/delete", `{"ids":["` + own.ID + `","` + other.ID + `"]}`, project, 404},
				{"POST", "/api/v1/inboxes/local/purge", `{}`, project, 403},
				{"POST", "/api/v1/messages", `{"inbox":"local"}`, writer, 403},
				{"POST", "/api/v1/messages", `{"mode":"relay"}`, writer, 403},
				{"POST", "/api/v1/messages", `{"direction":"inbound"}`, writer, 403},
				{"POST", "/api/v1/messages", `{"scenario_id":"foreign"}`, writer, 403},
				{"POST", "/api/v1/messages", `{"callback_url":"https://example.test"}`, writer, 403},
			} {
				t.Run(tc.method+tc.path, func(t *testing.T) {
					r := request(t, s, tc.method, tc.path, tc.body, tc.token)
					if r.StatusCode != tc.status {
						b, _ := io.ReadAll(r.Body)
						t.Fatalf("got %d want %d: %s", r.StatusCode, tc.status, b)
					}
				})
			}
			if _, err := store.Get(ctx, own.ID); err != nil {
				t.Fatal("mixed-scope bulk request partially deleted owned message", err)
			}
			r := request(t, s, "GET", "/api/v1/messages", "", reader)
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), own.ID) || strings.Contains(string(b), other.ID) || strings.Contains(string(b), sibling.ID) {
				t.Fatalf("scope: %s", b)
			}
			r = request(t, s, "GET", "/api/v1/workspaces", "", reader)
			b, _ = io.ReadAll(r.Body)
			if strings.Contains(string(b), "a2") || strings.Contains(string(b), "default") {
				t.Fatalf("workspace leak: %s", b)
			}
			for _, path := range []string{"keys", "pairings", "devices", "relay", "gateways", "lab/devices", "scenarios", "webhooks", "inbound", "projects", "inboxes", "network", "future/admin", "otp/format"} {
				for _, method := range []string{"GET", "POST", "DELETE"} {
					r = request(t, s, method, "/api/v1/"+path, `{}`, project)
					if r.StatusCode != 403 {
						t.Fatalf("operator route open: %s %s %d", method, path, r.StatusCode)
					}
				}
			}
			if r = request(t, s, "GET", "/connect/v1/session", "", reader); r.StatusCode != 401 {
				t.Fatal("API key became a phone session")
			}
			if r = request(t, s, "GET", "/connect/v1/push/config", "", reader); r.StatusCode != 401 {
				t.Fatal("API key accessed phone push")
			}
			if err := store.CreateInbox(ctx, workspace.Inbox{ID: "a3", ProjectID: "a", Name: "A3"}); err != nil {
				t.Fatal(err)
			}
			if r = request(t, s, "GET", "/api/v1/messages?inbox=a3", "", project); r.StatusCode != 200 {
				t.Fatal("project key omitted new inbox")
			}
		})
	}
}

func TestKeyCredentialEncodingsAndRevocation(t *testing.T) {
	s, _ := setup(t, "")
	key, token := issueKey(t, s, "", "default", "local", "messages:write")
	_, readOnly := issueKey(t, s, "", "default", "local", "messages:read")
	for _, provider := range []string{"twilio", "vonage", "ovh"} {
		for _, credential := range []string{token, readOnly, "td_key_invalid", "td_device_invalid"} {
			var path, body, content string
			switch provider {
			case "twilio":
				path = "/2010-04-01/Accounts/ACtest/Messages.json"
				body = "To=%2B12025550123&From=Test&Body=Code"
				content = "application/x-www-form-urlencoded"
			case "vonage":
				path = "/sms/json"
				body = `{"api_secret":"` + credential + `","to":"12025550123","from":"Test","text":"Code"}`
				content = "application/json"
			case "ovh":
				path = "/1.0/sms/test/jobs"
				body = `{"receivers":["+12025550123"],"sender":"Test","message":"Code"}`
				content = "application/json"
			}
			req, _ := http.NewRequest("POST", s.URL+path, strings.NewReader(body))
			req.Header.Set("Content-Type", content)
			if provider == "twilio" {
				req.SetBasicAuth("ACtest", credential)
			}
			if provider == "ovh" {
				req.Header.Set("X-Ovh-Consumer", credential)
			}
			r, err := s.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			expected := 403
			if credential == token {
				expected = 200
				if provider == "twilio" {
					expected = 201
				}
			}
			if credential == "td_key_invalid" {
				expected = 401
			}
			if r.StatusCode != expected {
				t.Fatalf("%s credential: got %d expected %d", provider, r.StatusCode, expected)
			}
		}
	}
	req, _ := http.NewRequest("GET", s.URL+"/api/v1/keys", nil)
	req.SetBasicAuth("ignored", token)
	r, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 401 {
		t.Fatal("Basic key fell back to anonymous operator")
	}
	req, _ = http.NewRequest("GET", s.URL+"/api/v1/keys", nil)
	req.Header.Set("Authorization", "bearer "+token)
	r, err = s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal("case-insensitive bearer lost restrictions")
	}
	r = request(t, s, "GET", "/api/v1/keys", "", "")
	b, _ := io.ReadAll(r.Body)
	if strings.Contains(string(b), token) || strings.Contains(string(b), access.Hash(token)) {
		t.Fatal("key material appeared in listing")
	}
	r = request(t, s, "DELETE", "/api/v1/keys/"+key.ID, "", "")
	if r.StatusCode != 204 {
		t.Fatal(r.StatusCode)
	}
	if r = request(t, s, "POST", "/api/v1/messages", `{}`, token); r.StatusCode != 401 {
		t.Fatal("revoked key accepted")
	}
}

func TestAPIKeyRevocationClosesStreamAndOTPWait(t *testing.T) {
	s, _ := setup(t, "operator")
	key, token := issueKey(t, s, "operator", "default", "local", "messages:read")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	scanner := bufio.NewScanner(r.Body)
	if !scanner.Scan() || scanner.Text() != "event: sync" {
		t.Fatal("no initial scoped sync")
	}
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/v1/otp?to=%2B12025550123&run_id=none&timeout=30", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := s.Client().Do(req)
		if err != nil {
			done <- 0
			return
		}
		defer response.Body.Close()
		done <- response.StatusCode
	}()
	response := request(t, s, "DELETE", "/api/v1/keys/"+key.ID, "", "operator")
	if response.StatusCode != 204 {
		t.Fatal(response.StatusCode)
	}
	revoked := false
	for scanner.Scan() {
		if scanner.Text() == "event: revoked" {
			revoked = true
		}
	}
	if !revoked {
		t.Fatal("stream not revoked", scanner.Err())
	}
	if status := <-done; status != 401 {
		t.Fatalf("pending OTP wait returned %d", status)
	}
}
