package httpapi

import (
	"errors"
	"net/http"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/push"
)

func (s *Server) devicePushAPI(w http.ResponseWriter, r *http.Request, d connect.Device) bool {
	if r.URL.Path == "/connect/v1/push/config" && r.Method == "GET" {
		enabled := s.Push != nil && s.Push.Config.Enabled
		public := ""
		if enabled {
			public = s.Push.Keys.Public
		}
		writeJSON(w, 200, map[string]any{"enabled": enabled, "public_key": public, "privacy": "generic", "service_worker": "/phone-sw.js"})
		return true
	}
	if s.Push == nil {
		if r.URL.Path == "/connect/v1/push/state" && r.Method == "GET" {
			writeJSON(w, 200, push.State{Status: "disabled"})
			return true
		}
		return false
	}
	switch {
	case r.URL.Path == "/connect/v1/push/state" && r.Method == "GET":
		state, err := s.Push.Store.PushState(r.Context(), d.ID)
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, state)
	case r.URL.Path == "/connect/v1/push/subscription" && r.Method == "PUT":
		if !s.Push.Config.Enabled {
			fail(w, 403, "Web Push is disabled on this server")
			return true
		}
		var sub push.Subscription
		if decode(r, &sub) != nil {
			fail(w, 400, "invalid push subscription JSON")
			return true
		}
		if err := s.Push.Config.Validate(&sub); err != nil {
			fail(w, 400, err.Error())
			return true
		}
		state, err := s.Push.Store.PutPushSubscription(r.Context(), d.ID, sub)
		if errors.Is(err, push.ErrSession) {
			fail(w, 401, err.Error())
			return true
		}
		if errors.Is(err, push.ErrConflict) {
			fail(w, 409, err.Error())
			return true
		}
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, state)
	case r.URL.Path == "/connect/v1/push/subscription" && r.Method == "DELETE":
		if err := s.Push.Store.DeletePushSubscription(r.Context(), d.ID); err != nil {
			internalError(w, err)
			return true
		}
		w.WriteHeader(204)
	case r.URL.Path == "/connect/v1/push/test" && r.Method == "POST":
		if !s.Push.Config.Enabled {
			fail(w, 403, "Web Push is disabled on this server")
			return true
		}
		if err := s.Push.Store.QueuePushTest(r.Context(), d.ID); err != nil {
			if errors.Is(err, push.ErrSubscription) {
				fail(w, 409, "enable notifications for this session first")
			} else {
				internalError(w, err)
			}
			return true
		}
		writeJSON(w, 202, map[string]string{"status": "queued"})
	default:
		return false
	}
	return true
}
