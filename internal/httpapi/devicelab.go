package httpapi

import (
	"errors"
	"net/http"

	"github.com/fless-lab/TextDock/internal/devicelab"
)

func (s *Server) deviceLabAPI(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/lab/status":
		if s.Lab == nil {
			writeJSON(w, 200, devicelab.Status{Devices: []devicelab.Device{}})
			return true
		}
		writeJSON(w, 200, s.Lab.Status(r.Context()))
	case r.Method == "POST" && r.URL.Path == "/api/v1/lab/injections":
		if s.Lab == nil || !s.Lab.Config.Enabled {
			fail(w, 403, devicelab.ErrDisabled.Error())
			return true
		}
		var in devicelab.Input
		if decode(r, &in) != nil {
			fail(w, 400, "invalid injection JSON")
			return true
		}
		if in.Inbox == "" {
			in.Inbox = "local"
		}
		exists, err := s.Workspaces.InboxExists(r.Context(), in.Inbox)
		if err != nil {
			internalError(w, err)
			return true
		}
		if !exists {
			fail(w, 400, "inbox does not exist")
			return true
		}
		result, err := s.Lab.Inject(r.Context(), in)
		switch {
		case errors.Is(err, devicelab.ErrInput):
			fail(w, 400, err.Error())
		case errors.Is(err, devicelab.ErrTarget), errors.Is(err, devicelab.ErrConflict):
			fail(w, 409, err.Error())
		case errors.Is(err, devicelab.ErrUnavailable):
			fail(w, 503, err.Error())
		case err != nil:
			internalError(w, err)
		default:
			writeJSON(w, 201, result)
		}
	case r.Method == "GET" && r.URL.Path == "/api/v1/lab/history":
		if s.Lab == nil {
			writeJSON(w, 200, map[string]any{"injections": []devicelab.Injection{}})
			return true
		}
		f, err := filter(r)
		if err != nil {
			fail(w, 400, err.Error())
			return true
		}
		items, err := s.Lab.Store.InjectionHistory(r.Context(), f.Inbox, f.Limit)
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"injections": items})
	default:
		return false
	}
	return true
}
