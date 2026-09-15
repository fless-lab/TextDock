package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
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

const userPassword = "correct horse battery staple"

func createTestUser(t *testing.T, s *httptest.Server, username string) access.User {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"username": username, "name": username, "password": userPassword})
	r := request(t, s, "POST", "/api/v1/team/users", string(data), "operator")
	if r.StatusCode != 201 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("create user %d %s", r.StatusCode, b)
	}
	var u access.User
	json.NewDecoder(r.Body).Decode(&u)
	return u
}
func loginTestUser(t *testing.T, s *httptest.Server, username, password string) (string, access.UserSession) {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"username": username, "password": password})
	r := request(t, s, "POST", "/api/v1/auth/login", string(data), "")
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("login %d %s", r.StatusCode, b)
	}
	var out struct {
		Token   string             `json:"token"`
		Session access.UserSession `json:"session"`
	}
	json.NewDecoder(r.Body).Decode(&out)
	return out.Token, out.Session
}
func grantTestRole(t *testing.T, s *httptest.Server, project, username, role, credential string) int {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"project_id": project, "username": username, "role": role})
	return request(t, s, "PUT", "/api/v1/team/memberships", string(data), credential).StatusCode
}

func TestUserProjectRolesAndNoPrivilegeUnion(t *testing.T) {
	s, store := setup(t, "operator")
	ctx := context.Background()
	for _, p := range []string{"alpha", "beta", "outside"} {
		if err := store.CreateProject(ctx, workspace.Project{ID: p, Name: p}, workspace.Inbox{ID: p, ProjectID: p, Name: p}); err != nil {
			t.Fatal(err)
		}
	}
	createTestUser(t, s, "alice")
	createTestUser(t, s, "bob")
	for _, grant := range [][3]string{{"alpha", "alice", "viewer"}, {"beta", "alice", "member"}, {"alpha", "bob", "admin"}} {
		if grantTestRole(t, s, grant[0], grant[1], grant[2], "operator") != 204 {
			t.Fatal("grant failed")
		}
	}
	alice, session := loginTestUser(t, s, "alice", userPassword)
	bob, _ := loginTestUser(t, s, "bob", userPassword)
	if session.Role("alpha") != "viewer" || session.Role("beta") != "member" {
		t.Fatal("missing roles")
	}
	capture := func(inbox string) message.Message {
		t.Helper()
		r := request(t, s, "POST", "/api/v1/messages", `{"inbox":"`+inbox+`","to":"+12025550123","from":"Team","body":"Code 482193","run_id":"team"}`, "operator")
		if r.StatusCode != 201 {
			t.Fatal(r.StatusCode)
		}
		var m message.Message
		json.NewDecoder(r.Body).Decode(&m)
		return m
	}
	alpha, beta, outside := capture("alpha"), capture("beta"), capture("outside")
	cases := []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", "/api/v1/info", "", alice, 200},
		{"GET", "/api/v1/messages?inbox=alpha", "", alice, 200},
		{"GET", "/api/v1/messages?inbox=outside", "", alice, 403},
		{"GET", "/api/v1/messages", "", alice, 400},
		{"POST", "/api/v1/messages", `{"inbox":"alpha","to":"+12025550123","from":"T","body":"no"}`, alice, 403},
		{"POST", "/api/v1/messages", `{"inbox":"beta","to":"+12025550123","from":"T","body":"yes"}`, alice, 201},
		{"POST", "/api/v1/messages", `{"inbox":"beta","mode":"relay"}`, alice, 403},
		{"PATCH", "/api/v1/messages/" + alpha.ID, `{"favorite":true}`, alice, 403},
		{"PATCH", "/api/v1/messages/" + beta.ID, `{"favorite":true}`, alice, 200},
		{"GET", "/api/v1/messages/" + outside.ID + "/events", "", alice, 404},
		{"GET", "/api/v1/export?inbox=outside", "", alice, 403},
		{"GET", "/api/v1/export?inbox=alpha&format=csv", "", alice, 200},
		{"GET", "/api/v1/otp?inbox=alpha&to=%2B12025550123&run_id=team", "", alice, 200},
		{"DELETE", "/api/v1/messages/" + beta.ID, "", alice, 403},
		{"POST", "/api/v1/messages/delete", `{"ids":["` + alpha.ID + `","` + outside.ID + `"]}`, bob, 404},
		{"POST", "/api/v1/inboxes", `{"project_id":"alpha","name":"Allowed"}`, bob, 201},
		{"POST", "/api/v1/inboxes", `{"project_id":"beta","name":"Denied"}`, bob, 403},
		{"POST", "/api/v1/inboxes", `{"project_id":"beta","name":"Denied"}`, alice, 403},
		{"GET", "/api/v1/team/memberships?project_id=alpha", "", alice, 200},
		{"GET", "/api/v1/team/memberships?project_id=outside", "", alice, 403},
		{"GET", "/api/v1/team/users", "", bob, 403},
		{"GET", "/api/v1/keys", "", bob, 403},
		{"POST", "/api/v1/pairings", `{}`, bob, 403},
		{"GET", "/api/v1/relay", "", bob, 403},
		{"GET", "/api/v1/lab/devices", "", bob, 403},
		{"GET", "/api/v1/scenarios", "", bob, 403},
		{"GET", "/api/v1/webhooks?message_id=" + alpha.ID, "", bob, 403},
		{"POST", "/2010-04-01/Accounts/ACtest/Messages.json", `{}`, bob, 403},
		{"GET", "/connect/v1/messages", "", bob, 401},
	}
	for _, tc := range cases {
		r := request(t, s, tc.method, tc.path, tc.body, tc.token)
		if r.StatusCode != tc.status {
			b, _ := io.ReadAll(r.Body)
			t.Errorf("%s %s: %d want %d: %s", tc.method, tc.path, r.StatusCode, tc.status, b)
		}
	}
	if _, err := store.Get(ctx, alpha.ID); err != nil {
		t.Fatal("cross-project bulk deletion partially applied")
	}
	r := request(t, s, "GET", "/api/v1/workspaces", "", alice)
	body, _ := io.ReadAll(r.Body)
	if strings.Contains(string(body), "outside") || strings.Contains(string(body), "default") {
		t.Fatal("workspace leak", string(body))
	}
	if grantTestRole(t, s, "beta", "bob", "admin", bob) != 403 {
		t.Fatal("project administrator escalated into another project")
	}
	if grantTestRole(t, s, "alpha", "alice", "admin", alice) != 403 {
		t.Fatal("viewer promoted herself")
	}
	if grantTestRole(t, s, "alpha", "alice", "member", bob) != 204 {
		t.Fatal("project administrator cannot assign member")
	}
	if request(t, s, "GET", "/api/v1/info", "", alice).StatusCode != 401 {
		t.Fatal("role change left old session alive")
	}
}

func TestUserSessionPasswordDisableAndOwnership(t *testing.T) {
	s, _ := setup(t, "operator")
	alice := createTestUser(t, s, "alice")
	createTestUser(t, s, "bob")
	a, first := loginTestUser(t, s, "alice", userPassword)
	other, second := loginTestUser(t, s, "alice", userPassword)
	b, bob := loginTestUser(t, s, "bob", userPassword)
	if request(t, s, "DELETE", "/api/v1/account/sessions/"+bob.ID, "", a).StatusCode != 404 {
		t.Fatal("cross-user session revocation")
	}
	if request(t, s, "DELETE", "/api/v1/account/sessions/"+second.ID, "", a).StatusCode != 204 {
		t.Fatal("own session revocation")
	}
	if request(t, s, "GET", "/api/v1/info", "", other).StatusCode != 401 {
		t.Fatal("session remained valid")
	}
	if request(t, s, "GET", "/api/v1/account", "", b).StatusCode != 200 {
		t.Fatal("unrelated user was affected")
	}
	r := request(t, s, "PUT", "/api/v1/account/password", `{"current_password":"wrong password here","password":"a new long password"}`, a)
	if r.StatusCode != 403 {
		t.Fatal(r.StatusCode)
	}
	r = request(t, s, "PUT", "/api/v1/account/password", `{"current_password":"`+userPassword+`","password":"a new long password"}`, a)
	if r.StatusCode != 204 {
		t.Fatal(r.StatusCode)
	}
	if request(t, s, "GET", "/api/v1/account/sessions", "", a).StatusCode != 401 {
		t.Fatal("password change did not revoke current session", first.ID)
	}
	a, _ = loginTestUser(t, s, "alice", "a new long password")
	if request(t, s, "PATCH", "/api/v1/team/users/"+alice.ID, `{"disabled":true}`, "operator").StatusCode != 204 {
		t.Fatal("disable failed")
	}
	if request(t, s, "GET", "/api/v1/info", "", a).StatusCode != 401 {
		t.Fatal("disabled session accepted")
	}
	r = request(t, s, "POST", "/api/v1/auth/login", `{"username":"alice","password":"a new long password"}`, "")
	if r.StatusCode != 401 {
		t.Fatal("disabled login accepted")
	}
	r = request(t, s, "POST", "/api/v1/auth/login", `{"username":"absent","password":"a new long password"}`, "")
	body, _ := io.ReadAll(r.Body)
	if r.StatusCode != 401 || !strings.Contains(string(body), access.ErrLogin.Error()) {
		t.Fatal("login discloses unknown account")
	}
	r = request(t, s, "GET", "/api/v1/team/users", "", "operator")
	body, _ = io.ReadAll(r.Body)
	if strings.Contains(string(body), "password") || strings.Contains(string(body), "argon2") {
		t.Fatal("password material leaked")
	}
}

func TestMembershipChangeClosesUserStream(t *testing.T) {
	s, _ := setup(t, "operator")
	createTestUser(t, s, "alice")
	grantTestRole(t, s, "default", "alice", "member", "operator")
	token, _ := loginTestUser(t, s, "alice", userPassword)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/v1/events?inbox=local", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	scanner := bufio.NewScanner(r.Body)
	if !scanner.Scan() || scanner.Text() != "event: sync" {
		t.Fatal("missing initial event")
	}
	if grantTestRole(t, s, "default", "alice", "viewer", "operator") != 204 {
		t.Fatal("downgrade failed")
	}
	revoked := false
	for scanner.Scan() {
		if scanner.Text() == "event: revoked" {
			revoked = true
		}
	}
	if !revoked {
		t.Fatal("stream survived membership change", scanner.Err())
	}
}
