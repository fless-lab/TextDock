package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fless-lab/TextDock/internal/access"
	"github.com/fless-lab/TextDock/internal/events"
	"github.com/fless-lab/TextDock/internal/workspace"
)

func (s *Server) passwordWork(w http.ResponseWriter, username string, limited bool) (func(), bool) {
	done, ok := s.Logins.Acquire(username, limited)
	if !ok {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "too many password operations; try again later")
	}
	return done, ok
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.Users == nil || s.Token == "" {
		fail(w, 403, "user sign-in requires a configured server token")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "provide username and password")
		return
	}
	in.Username = access.NormalizeUsername(in.Username)
	if !access.ValidUsername(in.Username) || !access.ValidPassword(in.Password) {
		fail(w, 401, access.ErrLogin.Error())
		return
	}
	done, ok := s.passwordWork(w, in.Username, true)
	if !ok {
		return
	}
	defer done()
	u, hash, err := s.Users.UserCredential(r.Context(), in.Username)
	if err != nil && !errors.Is(err, access.ErrLogin) {
		internalError(w, err)
		return
	}
	if hash == "" {
		hash = access.DummyPasswordHash
	}
	verified := access.VerifyPassword(in.Password, hash)
	if err != nil || u.Disabled || !verified {
		fail(w, 401, access.ErrLogin.Error())
		return
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		internalError(w, err)
		return
	}
	token := "td_user_" + base64.RawURLEncoding.EncodeToString(secret)
	session := access.UserSession{ID: "session_" + rand.Text(), User: u, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(12 * time.Hour)}
	if err := s.Users.CreateUserSession(r.Context(), session, access.Hash(token), hash); err != nil {
		if errors.Is(err, access.ErrLogin) {
			fail(w, 401, access.ErrLogin.Error())
		} else {
			internalError(w, err)
		}
		return
	}
	session, err = s.Users.UserSessionByHash(r.Context(), access.Hash(token))
	if err != nil {
		fail(w, 401, access.ErrLogin.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"token": token, "session": session})
}
func (s *Server) userAuthenticated(w http.ResponseWriter, r *http.Request, credential string) bool {
	if s.Users == nil {
		fail(w, 401, access.ErrSession.Error())
		return false
	}
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		fail(w, 403, "use an API key for provider integrations")
		return false
	}
	hash := access.Hash(credential)
	session, err := s.Users.UserSessionByHash(r.Context(), hash)
	if errors.Is(err, access.ErrSession) {
		fail(w, 401, err.Error())
		return false
	}
	if err != nil {
		internalError(w, err)
		return false
	}
	p := principal{Key: access.Key{ID: session.ID, ExpiresAt: session.ExpiresAt}, Hash: hash, Session: &session}
	*r = *r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
	return true
}

func (s *Server) finishUserMutation(w http.ResponseWriter, ids []string, err error) {
	if errors.Is(err, access.ErrUser) {
		fail(w, 404, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	for _, id := range ids {
		s.Hub.Revoke(id)
	}
	w.WriteHeader(204)
}
func (s *Server) teamAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/team/") {
		return false
	}
	p, restricted := keyPrincipal(r)
	if restricted && p.Session == nil {
		fail(w, 403, "machine keys cannot manage team accounts")
		return true
	}
	if s.Users == nil || s.Token == "" {
		fail(w, 409, "configure the server token before managing users")
		return true
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/team/")
	if path == "memberships" {
		project := r.URL.Query().Get("project_id")
		var in struct {
			ProjectID string `json:"project_id"`
			Username  string `json:"username"`
			Role      string `json:"role"`
		}
		if r.Method == "PUT" {
			if decode(r, &in) != nil {
				fail(w, 400, "invalid membership")
				return true
			}
			project = in.ProjectID
		}
		if project == "" {
			fail(w, 400, "project_id is required")
			return true
		}
		if restricted {
			role := p.Session.Role(project)
			if role == "" || (r.Method != "GET" && role != "admin") {
				fail(w, 403, "project administrator access required")
				return true
			}
		}
		switch r.Method {
		case "GET":
			members, err := s.Users.ProjectMembers(r.Context(), project)
			if err != nil {
				internalError(w, err)
			} else {
				writeJSON(w, 200, map[string]any{"members": members})
			}
		case "PUT":
			if len(access.RolePermissions(in.Role)) == 0 {
				fail(w, 400, "role must be viewer, member or admin")
				return true
			}
			ids, err := s.Users.SetMembership(r.Context(), project, access.NormalizeUsername(in.Username), in.Role)
			s.finishUserMutation(w, ids, err)
		case "DELETE":
			ids, err := s.Users.RemoveMembership(r.Context(), project, r.URL.Query().Get("user_id"))
			s.finishUserMutation(w, ids, err)
		default:
			fail(w, 405, "method not allowed")
		}
		return true
	}
	if restricted {
		fail(w, 403, "account administration requires operator access")
		return true
	}
	switch {
	case path == "users" && r.Method == "GET":
		users, err := s.Users.ListUsers(r.Context())
		if err != nil {
			internalError(w, err)
		} else {
			writeJSON(w, 200, map[string]any{"users": users})
		}
	case path == "users" && r.Method == "POST":
		var in struct {
			Username string `json:"username"`
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if decode(r, &in) != nil {
			fail(w, 400, "invalid user request")
			return true
		}
		in.Username = access.NormalizeUsername(in.Username)
		in.Name = strings.TrimSpace(in.Name)
		if !access.ValidUsername(in.Username) || in.Name == "" || utf8.RuneCountInString(in.Name) > 80 || !access.ValidPassword(in.Password) {
			fail(w, 400, "username: 3–64 lowercase letters/digits/dot/dash/underscore; name: 1–80 characters; password: 12–128 characters")
			return true
		}
		done, ok := s.passwordWork(w, "", false)
		if !ok {
			return true
		}
		defer done()
		hash, err := access.HashPassword(in.Password)
		if err != nil {
			internalError(w, err)
			return true
		}
		u := access.User{ID: "user_" + rand.Text(), Username: in.Username, Name: in.Name, CreatedAt: time.Now().UTC()}
		err = s.Users.CreateUser(r.Context(), u, hash)
		if errors.Is(err, access.ErrUsername) {
			fail(w, 409, err.Error())
		} else if err != nil {
			internalError(w, err)
		} else {
			writeJSON(w, 201, u)
		}
	case strings.HasPrefix(path, "users/") && strings.HasSuffix(path, "/password") && r.Method == "PUT":
		var in struct {
			Password string `json:"password"`
		}
		if decode(r, &in) != nil || !access.ValidPassword(in.Password) {
			fail(w, 400, "password must contain 12–128 characters")
			return true
		}
		done, ok := s.passwordWork(w, "", false)
		if !ok {
			return true
		}
		defer done()
		hash, err := access.HashPassword(in.Password)
		if err != nil {
			internalError(w, err)
			return true
		}
		id := strings.TrimSuffix(strings.TrimPrefix(path, "users/"), "/password")
		ids, err := s.Users.SetUserPassword(r.Context(), id, hash, "")
		s.finishUserMutation(w, ids, err)
	case strings.HasPrefix(path, "users/") && r.Method == "PATCH":
		var in struct {
			Disabled *bool `json:"disabled"`
		}
		if decode(r, &in) != nil || in.Disabled == nil {
			fail(w, 400, "provide disabled")
			return true
		}
		ids, err := s.Users.SetUserDisabled(r.Context(), strings.TrimPrefix(path, "users/"), *in.Disabled)
		s.finishUserMutation(w, ids, err)
	default:
		fail(w, 404, "team endpoint not found")
	}
	return true
}
func (s *Server) accountAPI(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/account") {
		return false
	}
	p, ok := keyPrincipal(r)
	if !ok || p.Session == nil {
		fail(w, 403, "user session required")
		return true
	}
	session := p.Session
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/account/events":
		s.stream(w, r, events.Filter{Inbox: "session:" + session.ID, DeviceID: session.ID}, session.ExpiresAt)
	case r.Method == "GET" && r.URL.Path == "/api/v1/account":
		writeJSON(w, 200, session)
	case r.Method == "GET" && r.URL.Path == "/api/v1/account/sessions":
		items, err := s.Users.UserSessions(r.Context(), session.User.ID)
		if err != nil {
			internalError(w, err)
		} else {
			writeJSON(w, 200, map[string]any{"sessions": items, "current_id": session.ID})
		}
	case r.Method == "POST" && r.URL.Path == "/api/v1/account/logout":
		_, err := s.Users.RevokeUserSession(r.Context(), session.User.ID, session.ID)
		s.finishUserMutation(w, []string{session.ID}, err)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/account/sessions/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/account/sessions/")
		ok, err := s.Users.RevokeUserSession(r.Context(), session.User.ID, id)
		if err == nil && !ok {
			err = access.ErrUser
		}
		s.finishUserMutation(w, []string{id}, err)
	case r.Method == "PUT" && r.URL.Path == "/api/v1/account/password":
		var in struct {
			Current  string `json:"current_password"`
			Password string `json:"password"`
		}
		if decode(r, &in) != nil || !access.ValidPassword(in.Current) || !access.ValidPassword(in.Password) {
			fail(w, 400, "passwords must contain 12–128 characters")
			return true
		}
		done, ok := s.passwordWork(w, session.User.Username, true)
		if !ok {
			return true
		}
		defer done()
		_, previous, err := s.Users.UserCredential(r.Context(), session.User.Username)
		if err != nil {
			internalError(w, err)
			return true
		}
		if !access.VerifyPassword(in.Current, previous) {
			fail(w, 403, "current password is incorrect")
			return true
		}
		hash, err := access.HashPassword(in.Password)
		if err != nil {
			internalError(w, err)
			return true
		}
		ids, err := s.Users.SetUserPassword(r.Context(), session.User.ID, hash, previous)
		s.finishUserMutation(w, ids, err)
	default:
		fail(w, 404, "account endpoint not found")
	}
	return true
}

func readUserBody(w http.ResponseWriter, r *http.Request, out any) bool {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 400, "invalid request body")
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if json.Unmarshal(data, out) != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}
	return true
}

// Translate the selected resource's project role into the existing scoped route
// policy. Permissions from a different project never contribute to this grant.
func (s *Server) userAPI(w http.ResponseWriter, r *http.Request) bool {
	p, ok := keyPrincipal(r)
	if !ok || p.Session == nil {
		return false
	}
	session := p.Session
	if r.Method == "GET" && r.URL.Path == "/api/v1/session" {
		writeJSON(w, 200, session)
		return true
	}
	if r.Method == "GET" && r.URL.Path == "/api/v1/workspaces" {
		projects, err := s.Workspaces.ListProjects(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		boxes, err := s.Workspaces.ListInboxes(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		visibleProjects := []workspace.Project{}
		visibleBoxes := []workspace.Inbox{}
		for _, v := range projects {
			if session.Role(v.ID) != "" {
				visibleProjects = append(visibleProjects, v)
			}
		}
		for _, v := range boxes {
			if session.Role(v.ProjectID) != "" {
				visibleBoxes = append(visibleBoxes, v)
			}
		}
		writeJSON(w, 200, map[string]any{"projects": visibleProjects, "inboxes": visibleBoxes})
		return true
	}
	if r.Method == "POST" && r.URL.Path == "/api/v1/inboxes" {
		var in struct {
			ProjectID string `json:"project_id"`
		}
		if !readUserBody(w, r, &in) {
			return true
		}
		if session.Role(in.ProjectID) != "admin" {
			fail(w, 403, "project administrator access required")
			return true
		}
		return s.workspaceAPI(w, r)
	}
	inbox := ""
	id := ""
	path := r.URL.Path
	switch {
	case r.Method == "GET" && (path == "/api/v1/messages" || path == "/api/v1/otp" || path == "/api/v1/export" || path == "/api/v1/events"):
		inbox = r.URL.Query().Get("inbox")
	case r.Method == "POST" && path == "/api/v1/messages":
		var in struct {
			Inbox string `json:"inbox"`
		}
		if !readUserBody(w, r, &in) {
			return true
		}
		inbox = in.Inbox
	case r.Method == "POST" && path == "/api/v1/messages/delete":
		var in struct {
			IDs []string `json:"ids"`
		}
		if !readUserBody(w, r, &in) {
			return true
		}
		if len(in.IDs) < 1 || len(in.IDs) > 200 {
			fail(w, 400, "provide 1–200 message IDs")
			return true
		}
		id = in.IDs[0]
	case strings.HasPrefix(path, "/api/v1/messages/"):
		id = strings.TrimPrefix(path, "/api/v1/messages/")
		if r.Method == "GET" {
			id = strings.TrimSuffix(id, "/events")
		}
	case r.Method == "POST" && strings.HasPrefix(path, "/api/v1/inboxes/") && strings.HasSuffix(path, "/purge"):
		inbox = strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/inboxes/"), "/purge")
	default:
		return false
	}
	if id != "" {
		m, err := s.Store.Get(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "message not found")
			return true
		}
		if err != nil {
			internalError(w, err)
			return true
		}
		inbox = m.Inbox
	}
	if inbox == "" {
		fail(w, 400, "select an inbox")
		return true
	}
	boxes, err := s.Workspaces.ListInboxes(r.Context())
	if err != nil {
		internalError(w, err)
		return true
	}
	for _, box := range boxes {
		if box.ID == inbox && session.Role(box.ProjectID) != "" {
			p.Key.ProjectID = box.ProjectID
			p.Key.InboxID = ""
			p.Key.Permissions = access.RolePermissions(session.Role(box.ProjectID))
			*r = *r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
			return false
		}
	}
	if id != "" {
		fail(w, 404, "message not found")
	} else {
		fail(w, 403, "inbox outside user projects")
	}
	return true
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"name": "TextDock", "version": s.Version, "mode": "local", "auth_enabled": s.Token != "", "webhook_signing": s.WebhookSecret != "", "operator": true}
	if p, ok := keyPrincipal(r); ok {
		out["operator"] = false
		if p.Session != nil {
			out["session"] = p.Session
		} else {
			out["api_key"] = p.Key
		}
	}
	writeJSON(w, 200, out)
}
