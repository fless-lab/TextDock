package httpapi

import (
	"crypto/rand"
	"net/http"
	"strings"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/simulation"
)

func (s *Server) simulationAPI(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/scenarios":
		inbox := r.URL.Query().Get("inbox")
		if inbox == "" {
			inbox = "local"
		}
		items, err := s.Simulation.ListScenarios(r.Context(), inbox)
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"scenarios": items})
	case r.Method == "POST" && r.URL.Path == "/api/v1/scenarios":
		var scenario simulation.Scenario
		if decode(r, &scenario) != nil {
			fail(w, 400, "invalid scenario JSON")
			return true
		}
		scenario.ID = "scenario_" + rand.Text()
		if scenario.Inbox == "" {
			scenario.Inbox = "local"
		}
		if scenario.Outcome == "" {
			scenario.Outcome = "delivered"
		}
		if scenario.WebhookFormat == "" {
			scenario.WebhookFormat = "json"
		}
		if err := scenario.Validate(); err != nil {
			fail(w, 400, err.Error())
			return true
		}
		if ok, err := s.Workspaces.InboxExists(r.Context(), scenario.Inbox); err != nil {
			internalError(w, err)
			return true
		} else if !ok {
			fail(w, 400, "inbox does not exist")
			return true
		}
		if err := s.Simulation.PutScenario(r.Context(), scenario); err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 201, scenario)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/scenarios/"):
		if err := s.Simulation.DeleteScenario(r.Context(), strings.TrimPrefix(r.URL.Path, "/api/v1/scenarios/")); err != nil {
			internalError(w, err)
			return true
		}
		w.WriteHeader(204)
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/messages/") && strings.HasSuffix(r.URL.Path, "/events"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/messages/"), "/events")
		items, err := s.Simulation.Events(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"events": items})
	case r.Method == "GET" && r.URL.Path == "/api/v1/webhooks":
		id := r.URL.Query().Get("message_id")
		if id == "" {
			fail(w, 400, "message_id is required")
			return true
		}
		items, err := s.Simulation.Attempts(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"attempts": items, "signing_enabled": s.WebhookSecret != ""})
	case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v1/webhooks/") && strings.HasSuffix(r.URL.Path, "/retry"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/webhooks/"), "/retry")
		ok, err := s.Simulation.RetryWebhook(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return true
		}
		if !ok {
			fail(w, 409, "webhook is missing or already pending/running")
			return true
		}
		w.WriteHeader(204)
	case r.Method == "POST" && r.URL.Path == "/api/v1/inbound":
		var in message.Input
		if decode(r, &in) != nil {
			fail(w, 400, "invalid inbound message JSON")
			return true
		}
		in.Direction = "inbound"
		in.Mode = "simulate"
		s.capture(w, r, in, "api", false)
	default:
		return false
	}
	return true
}
