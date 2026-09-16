package httpapi

import (
	"context"
	"errors"
	"github.com/fless-lab/TextDock/internal/audit"
	"net/http"
	"slices"
	"strconv"
	"time"
)

func auditActor(r *http.Request, kind, id string) {
	*r = *r.WithContext(audit.WithActor(r.Context(), audit.Actor{Kind: kind, ID: id}))
}

func (s *Server) auditAPI(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/v1/audit" {
		return false
	}
	if r.Method != "GET" {
		fail(w, 405, "audit events are read-only")
		return true
	}
	p, scoped := keyPrincipal(r)
	q := r.URL.Query()
	f := audit.Filter{ProjectID: q.Get("project_id"), InboxID: q.Get("inbox_id"), Action: q.Get("action"), Outcome: q.Get("outcome"), ActorID: q.Get("actor_id"), Limit: 100}
	if scoped && (p.Session == nil || f.ProjectID == "" || p.Session.Role(f.ProjectID) != "admin") {
		fail(w, 403, "audit access requires operator or project-admin rights")
		return true
	}
	if s.Audit == nil {
		fail(w, 503, "audit storage unavailable")
		return true
	}
	for _, value := range []string{f.ProjectID, f.InboxID, f.ActorID} {
		if len(value) > 128 {
			fail(w, 400, "audit identifiers must be at most 128 bytes")
			return true
		}
	}
	if f.Action != "" && !slices.Contains(audit.Actions, f.Action) {
		fail(w, 400, "unknown audit action")
		return true
	}
	if f.Outcome != "" && !slices.Contains([]string{"committed", "denied", "prepared"}, f.Outcome) {
		fail(w, 400, "invalid audit outcome")
		return true
	}
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 || n > 200 {
			fail(w, 400, "limit must be 1–200")
			return true
		}
		f.Limit = n
	}
	if q.Has("before") {
		n, err := strconv.ParseInt(q.Get("before"), 10, 64)
		if err != nil || n < 1 {
			fail(w, 400, "before must be a positive event ID")
			return true
		}
		f.Before = n
	}
	if q.Has("since") {
		at, err := time.Parse(time.RFC3339Nano, q.Get("since"))
		if err != nil {
			fail(w, 400, "since must be an RFC3339 timestamp")
			return true
		}
		f.Since = at
	}
	if f.InboxID != "" {
		boxes, err := s.Workspaces.ListInboxes(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		found := false
		for _, b := range boxes {
			if b.ID == f.InboxID && (f.ProjectID == "" || b.ProjectID == f.ProjectID) {
				found = true
				f.ProjectID = b.ProjectID
				break
			}
		}
		if !found {
			fail(w, 403, "inbox outside selected audit project")
			return true
		}
	}
	limit := f.Limit
	f.Limit++
	items, err := s.Audit.ListAudit(r.Context(), f)
	if err != nil {
		internalError(w, err)
		return true
	}
	var next int64
	if len(items) > limit {
		items = items[:limit]
		next = items[len(items)-1].ID
	}
	writeJSON(w, 200, map[string]any{"events": items, "next_cursor": next, "actions": audit.Actions})
	return true
}

func (s *Server) recordAudit(w http.ResponseWriter, r *http.Request, e audit.Event) bool {
	if s.Audit == nil {
		internalError(w, errors.New("audit storage unavailable"))
		return false
	}
	// Read/rejected operations have no mutation transaction. Finish a bounded
	// metadata append even when the client has already disconnected.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	if err := s.Audit.AppendAudit(ctx, e); err != nil {
		internalError(w, err)
		return false
	}
	return true
}
