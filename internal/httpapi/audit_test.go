package httpapi

import (
	"context"
	"encoding/json"
	"github.com/fless-lab/TextDock/internal/audit"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
	"io"
	"strings"
	"testing"
)

func TestAuditActorsScopesAndPrivacy(t *testing.T) {
	s, store := setup(t, "operator")
	for _, id := range []string{"alpha", "beta"} {
		if err := store.CreateProject(context.Background(), workspace.Project{ID: id, Name: id}, workspace.Inbox{ID: id, ProjectID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	alice := createTestUser(t, s, "alice")
	createTestUser(t, s, "viewer")
	grantTestRole(t, s, "alpha", "alice", "admin", "operator")
	grantTestRole(t, s, "alpha", "viewer", "viewer", "operator")
	token, _ := loginTestUser(t, s, "alice", userPassword)
	viewer, _ := loginTestUser(t, s, "viewer", userPassword)
	key, keyToken := issueKey(t, s, "operator", "alpha", "alpha", "messages:read", "messages:write", "messages:delete")
	response := request(t, s, "POST", "/api/v1/messages", `{"inbox":"alpha","to":"+12025550123","from":"PrivateSenderCanary","body":"PrivateBodyCanary 482193","run_id":"PrivateRunCanary"}`, keyToken)
	if response.StatusCode != 201 {
		t.Fatal(response.StatusCode)
	}
	var m message.Message
	json.NewDecoder(response.Body).Decode(&m)
	if request(t, s, "PATCH", "/api/v1/messages/"+m.ID, `{"tags":["PrivateTagCanary"]}`, token).StatusCode != 200 {
		t.Fatal("metadata update failed")
	}
	if request(t, s, "GET", "/api/v1/export?inbox=alpha&format=csv", "", viewer).StatusCode != 200 {
		t.Fatal("export failed")
	}
	if request(t, s, "DELETE", "/api/v1/messages/"+m.ID, "", keyToken).StatusCode != 204 {
		t.Fatal("delete failed")
	}
	if request(t, s, "POST", "/api/v1/auth/login", `{"username":"PrivateUsernameCanary","password":"PrivatePasswordCanary"}`, "").StatusCode != 401 {
		t.Fatal("expected failed login")
	}
	for _, tc := range []struct {
		path, credential string
		status           int
	}{
		{"/api/v1/audit", token, 403}, {"/api/v1/audit?project_id=beta", token, 403},
		{"/api/v1/audit?project_id=alpha&inbox_id=beta", token, 403},
		{"/api/v1/audit?project_id=alpha", viewer, 403}, {"/api/v1/audit?project_id=alpha", keyToken, 403},
		{"/api/v1/audit?project_id=alpha", token, 200}, {"/api/v1/audit?limit=201", "operator", 400},
		{"/api/v1/audit?before=-1", "operator", 400}, {"/api/v1/audit?action=unknown", "operator", 400},
	} {
		r := request(t, s, "GET", tc.path, "", tc.credential)
		if r.StatusCode != tc.status {
			t.Errorf("%s: %d want %d", tc.path, r.StatusCode, tc.status)
		}
	}
	r := request(t, s, "GET", "/api/v1/audit", "", "operator")
	body, _ := io.ReadAll(r.Body)
	for _, secret := range []string{keyToken, token, userPassword, "PrivateSenderCanary", "PrivateBodyCanary", "PrivateRunCanary", "PrivateTagCanary", "PrivateUsernameCanary", "PrivatePasswordCanary", "+12025550123", "482193", "argon2", "token_hash"} {
		if strings.Contains(strings.ToLower(string(body)), strings.ToLower(secret)) {
			t.Fatalf("private value leaked in audit: %q", secret)
		}
	}
	var result struct {
		Events []audit.Event `json:"events"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, e := range result.Events {
		if e.Action == "message.created" && e.ResourceID == m.ID {
			found["key"] = e.Actor.Kind == "api_key" && e.Actor.ID == key.ID && e.ProjectID == "alpha"
		}
		if e.Action == "message.updated" && e.ResourceID == m.ID {
			found["user"] = e.Actor.Kind == "user" && e.Actor.ID == alice.ID
		}
		if e.Action == "export.prepared" {
			found["export"] = e.Outcome == "prepared" && e.Count == 1
		}
		if e.Action == "login.failed" {
			found["login"] = e.Outcome == "denied" && e.Actor.Kind == "anonymous" && e.Actor.ID == ""
		}
		if e.Action == "user.created" {
			found["operator"] = e.Actor.Kind == "operator"
		}
	}
	for _, name := range []string{"key", "user", "export", "login", "operator"} {
		if !found[name] {
			t.Error("missing event/actor", name)
		}
	}
	r = request(t, s, "GET", "/api/v1/audit?project_id=alpha", "", token)
	body, _ = io.ReadAll(r.Body)
	if strings.Contains(string(body), `"action":"login.failed"`) || strings.Contains(string(body), `"project_id":"beta"`) || strings.Contains(string(body), `"action":"user.created"`) {
		t.Fatal("project audit leaked global or other-project events")
	}
	if request(t, s, "DELETE", "/api/v1/audit", "", "operator").StatusCode != 405 {
		t.Fatal("audit deletion exposed")
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Events {
		if e.ProjectID != "alpha" {
			t.Fatal("out-of-scope audit event")
		}
	}
	if r = request(t, s, "GET", "/api/v1/audit?project_id=alpha&action=message.created&actor_id="+key.ID, "", token); r.StatusCode != 200 {
		t.Fatal("filtered query failed")
	}
}
