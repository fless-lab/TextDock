package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/access"
	"github.com/fless-lab/TextDock/internal/events"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
)

type principalKey struct{}
type principal struct {
	Key  access.Key
	Hash string
}

func keyPrincipal(r *http.Request) (principal, bool) {
	p, ok := r.Context().Value(principalKey{}).(principal)
	return p, ok
}

// Resolve credentials before the anonymous-local fallback, so restricted/invalid
// credentials can never acquire operator privileges by using another encoding.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, credential string) bool {
	if len(r.Header.Values("Authorization")) > 1 {
		fail(w, 401, "provide one authorization credential")
		return false
	}
	if r.Header.Get("Authorization") != "" && bearer(r) == "" {
		_, _, basic := r.BasicAuth()
		if !basic || !strings.HasPrefix(r.URL.Path, "/2010-04-01/") {
			fail(w, 401, "use Bearer authorization")
			return false
		}
	}
	if strings.HasPrefix(credential, "td_device_") || strings.HasPrefix(credential, "td_gateway_") {
		fail(w, 403, "device credentials cannot access the desktop API")
		return false
	}
	if strings.HasPrefix(credential, "td_key_") {
		if s.Keys == nil {
			fail(w, 401, "invalid API key")
			return false
		}
		hash := access.Hash(credential)
		k, err := s.Keys.APIKeyByHash(r.Context(), hash)
		if errors.Is(err, access.ErrInvalid) {
			fail(w, 401, err.Error())
			return false
		}
		if err != nil {
			internalError(w, err)
			return false
		}
		*r = *r.WithContext(context.WithValue(r.Context(), principalKey{}, principal{k, hash}))
		return true
	}
	if s.Token != "" {
		a, b := sha256.Sum256([]byte(credential)), sha256.Sum256([]byte(s.Token))
		if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
			fail(w, 401, "a valid TextDock token is required")
			return false
		}
	}
	return true
}

func (s *Server) keysAPI(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/keys" && !strings.HasPrefix(r.URL.Path, "/api/v1/keys/") {
		return false
	}
	if s.Keys == nil {
		fail(w, 503, "API key storage unavailable")
		return true
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/keys":
		keys, err := s.Keys.ListAPIKeys(r.Context())
		if err != nil {
			internalError(w, err)
		} else {
			writeJSON(w, 200, map[string]any{"keys": keys})
		}
	case r.Method == "POST" && r.URL.Path == "/api/v1/keys":
		var in struct {
			Name        string    `json:"name"`
			ProjectID   string    `json:"project_id"`
			InboxID     string    `json:"inbox_id"`
			Permissions []string  `json:"permissions"`
			ExpiresAt   time.Time `json:"expires_at"`
		}
		if decode(r, &in) != nil {
			fail(w, 400, "invalid API key request")
			return true
		}
		k := access.Key{ID: "key_" + rand.Text(), Name: strings.TrimSpace(in.Name), ProjectID: in.ProjectID, InboxID: in.InboxID, Permissions: in.Permissions, CreatedAt: time.Now().UTC(), ExpiresAt: in.ExpiresAt}
		if err := k.Validate(time.Now()); err != nil {
			fail(w, 400, err.Error())
			return true
		}
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			internalError(w, err)
			return true
		}
		token := "td_key_" + base64.RawURLEncoding.EncodeToString(secret)
		if err := s.Keys.CreateAPIKey(r.Context(), k, access.Hash(token)); err != nil {
			if errors.Is(err, access.ErrScope) {
				fail(w, 400, err.Error())
			} else {
				internalError(w, err)
			}
			return true
		}
		writeJSON(w, 201, map[string]any{"key": k, "token": token})
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/keys/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/")
		ok, err := s.Keys.RevokeAPIKey(r.Context(), id)
		if err != nil {
			internalError(w, err)
		} else if !ok {
			fail(w, 404, "API key not found")
		} else {
			s.Hub.Revoke(id)
			w.WriteHeader(204)
		}
	default:
		fail(w, 405, "method not allowed")
	}
	return true
}

func (s *Server) keyInbox(w http.ResponseWriter, r *http.Request, inbox *string) bool {
	p, ok := keyPrincipal(r)
	if !ok {
		return true
	}
	if *inbox == "" {
		*inbox = p.Key.InboxID
	}
	if *inbox == "" {
		fail(w, 400, "project API keys require an explicit inbox")
		return false
	}
	allowed, err := s.Keys.KeyInboxAllowed(r.Context(), p.Key, *inbox)
	if err != nil {
		internalError(w, err)
		return false
	}
	if !allowed {
		fail(w, 403, "inbox outside API key scope")
		return false
	}
	return true
}
func (s *Server) keyCapture(w http.ResponseWriter, r *http.Request, in *message.Input) bool {
	p, ok := keyPrincipal(r)
	if !ok {
		return true
	}
	if !p.Key.Allows("messages:write") || (in.Mode != "" && in.Mode != "capture") || (in.Direction != "" && in.Direction != "outbound") || in.ScenarioID != "" || in.CallbackURL != "" {
		fail(w, 403, "API key permits local capture only; simulation, callbacks and real relay require operator access")
		return false
	}
	return s.keyInbox(w, r, &in.Inbox)
}
func (s *Server) keyMessage(w http.ResponseWriter, r *http.Request, id string) bool {
	m, err := s.Store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "message not found")
		return false
	}
	if err != nil {
		internalError(w, err)
		return false
	}
	p, _ := keyPrincipal(r)
	ok, err := s.Keys.KeyInboxAllowed(r.Context(), p.Key, m.Inbox)
	if err != nil {
		internalError(w, err)
		return false
	}
	if !ok {
		fail(w, 404, "message not found")
		return false
	}
	return true
}

// Explicit allowlist: newly added operator routes remain closed to API keys.
// Every resource selector is validated before calling existing handlers.
func (s *Server) scopedAPI(w http.ResponseWriter, r *http.Request) bool {
	p, scoped := keyPrincipal(r)
	if !scoped {
		return false
	}
	path, method := r.URL.Path, r.Method
	permission := ""
	queryInbox := false
	messageID := ""
	switch {
	case method == "GET" && path == "/api/v1/session":
		writeJSON(w, 200, p.Key)
		return true
	case method == "GET" && path == "/api/v1/info":
		return false
	case method == "GET" && path == "/api/v1/workspaces":
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
		projects = filterProjects(projects, p.Key.ProjectID)
		boxes = filterInboxes(boxes, p.Key)
		writeJSON(w, 200, map[string]any{"projects": projects, "inboxes": boxes})
		return true
	case method == "GET" && (path == "/api/v1/messages" || path == "/api/v1/otp" || path == "/api/v1/export" || path == "/api/v1/events"):
		permission = "messages:read"
		queryInbox = true
	case method == "GET" && strings.HasPrefix(path, "/api/v1/messages/") && strings.HasSuffix(path, "/events"):
		permission = "messages:read"
		messageID = strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/messages/"), "/events")
	case method == "POST" && path == "/api/v1/messages":
		permission = "messages:write"
	case method == "PATCH" && strings.HasPrefix(path, "/api/v1/messages/"):
		permission = "messages:write"
		messageID = strings.TrimPrefix(path, "/api/v1/messages/")
	case method == "DELETE" && strings.HasPrefix(path, "/api/v1/messages/"):
		permission = "messages:delete"
		messageID = strings.TrimPrefix(path, "/api/v1/messages/")
	case method == "POST" && path == "/api/v1/messages/delete":
		permission = "messages:delete"
	case method == "POST" && strings.HasPrefix(path, "/api/v1/inboxes/") && strings.HasSuffix(path, "/purge"):
		permission = "messages:delete"
	default:
		fail(w, 403, "this endpoint requires operator access")
		return true
	}
	if !p.Key.Allows(permission) {
		fail(w, 403, "API key lacks "+permission)
		return true
	}
	if method == "PATCH" && !p.Key.Allows("messages:read") {
		fail(w, 403, "metadata editing also requires messages:read")
		return true
	}
	if queryInbox {
		q := r.URL.Query()
		inbox := q.Get("inbox")
		if !s.keyInbox(w, r, &inbox) {
			return true
		}
		q.Set("inbox", inbox)
		r.URL.RawQuery = q.Encode()
		if path == "/api/v1/events" {
			s.stream(w, r, events.Filter{Inbox: inbox, DeviceID: p.Key.ID}, p.Key.ExpiresAt)
			return true
		}
	}
	if messageID != "" && !s.keyMessage(w, r, messageID) {
		return true
	}
	if method == "POST" && strings.HasSuffix(path, "/purge") {
		inbox := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/inboxes/"), "/purge")
		if !s.keyInbox(w, r, &inbox) {
			return true
		}
	}
	if path == "/api/v1/messages/delete" {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			fail(w, 400, "invalid bulk deletion")
			return true
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		var in struct {
			IDs []string `json:"ids"`
		}
		if json.Unmarshal(data, &in) != nil || len(in.IDs) == 0 || len(in.IDs) > 200 {
			fail(w, 400, "provide 1–200 message IDs")
			return true
		}
		for _, id := range in.IDs {
			if !s.keyMessage(w, r, id) {
				return true
			}
		}
	}
	return false
}
func (s *Server) keyAlive(r *http.Request) error {
	if p, ok := keyPrincipal(r); ok {
		_, err := s.Keys.APIKeyByHash(r.Context(), p.Hash)
		return err
	}
	return nil
}
func filterProjects(all []workspace.Project, id string) []workspace.Project {
	out := []workspace.Project{}
	for _, p := range all {
		if p.ID == id {
			out = append(out, p)
		}
	}
	return out
}
func filterInboxes(all []workspace.Inbox, k access.Key) []workspace.Inbox {
	out := []workspace.Inbox{}
	for _, b := range all {
		if b.ProjectID == k.ProjectID && (k.InboxID == "" || k.InboxID == b.ID) {
			out = append(out, b)
		}
	}
	return out
}
