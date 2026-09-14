package httpapi

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/relay"
)

func (s *Server) relayAPI(w http.ResponseWriter, r *http.Request) bool {
	if s.Relay == nil {
		if r.URL.Path == "/api/v1/relay" {
			writeJSON(w, 200, map[string]any{"enabled": false, "driver": "", "limit_per_minute": 10})
			return true
		}
		return false
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/relay":
		c := s.Relay.Config
		writeJSON(w, 200, map[string]any{"enabled": c.Driver != "", "driver": c.Driver, "limit_per_minute": c.Limit, "queue_ttl_seconds": int(c.TTL.Seconds()), "allowed_to": c.AllowedTo, "receipts_enabled": c.AuthToken != "" && c.PublicURL != ""})
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/messages/") && strings.HasSuffix(r.URL.Path, "/relay"):
		job, err := s.Relay.Store.RelayJob(r.Context(), strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/messages/"), "/relay"))
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "not a relayed message")
			return true
		}
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, job)
	case r.Method == "GET" && r.URL.Path == "/api/v1/gateways":
		items, err := s.Relay.Store.Gateways(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"gateways": items})
	case r.Method == "POST" && r.URL.Path == "/api/v1/gateways":
		if s.Relay.Config.Driver != "android" {
			fail(w, 400, "enable the android relay driver first")
			return true
		}
		var in struct {
			Name string `json:"name"`
		}
		if decode(r, &in) != nil || !validName(in.Name) {
			fail(w, 400, "a gateway name (1–80 characters) is required")
			return true
		}
		token := "td_gateway_" + rand.Text()
		g := relay.Gateway{ID: "gw_" + rand.Text(), Name: strings.TrimSpace(in.Name), ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour)}
		if err := s.Relay.Store.CreateGateway(r.Context(), g, connect.Hash(token)); err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 201, map[string]any{"gateway": g, "token": token})
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/gateways/"):
		if err := s.Relay.Store.RevokeGateway(r.Context(), strings.TrimPrefix(r.URL.Path, "/api/v1/gateways/")); err != nil {
			internalError(w, err)
			return true
		}
		w.WriteHeader(204)
	default:
		return false
	}
	return true
}

func (s *Server) gatewayAPI(w http.ResponseWriter, r *http.Request) {
	if s.Relay == nil || s.Relay.Config.Driver != "android" {
		fail(w, 403, "Android relay is disabled")
		return
	}
	token := bearer(r)
	if !strings.HasPrefix(token, "td_gateway_") {
		fail(w, 401, "gateway credential required")
		return
	}
	g, err := s.Relay.Store.GatewayByHash(r.Context(), connect.Hash(token))
	if errors.Is(err, relay.ErrCredential) {
		fail(w, 401, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/relay/v1/jobs/claim":
		j, err := s.Relay.Store.ClaimRelay(r.Context(), "android", g.ID, time.Now().UTC(), s.Relay.Config.Limit)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(204)
			return
		}
		if err != nil {
			internalError(w, err)
			return
		}
		s.Hub.Resync()
		writeJSON(w, 200, j)
	case r.Method == "POST" && r.URL.Path == "/relay/v1/heartbeat":
		var health relay.Health
		if decode(r, &health) != nil || len(health.Model) > 120 || len(health.AppVersion) > 48 || health.SubscriptionID < -1 || health.SubscriptionID > 2147483647 {
			fail(w, 400, "invalid gateway health fields")
			return
		}
		if err := s.Relay.Store.GatewayHeartbeat(r.Context(), g.ID, health); err != nil {
			internalError(w, err)
			return
		}
		w.WriteHeader(204)
	case r.Method == "POST" && r.URL.Path == "/relay/v1/jobs/result":
		var in struct {
			ID         string `json:"id"`
			LeaseToken string `json:"lease_token"`
			State      string `json:"state"`
			Error      string `json:"error"`
		}
		if decode(r, &in) != nil || len(in.Error) > 512 || (in.State != "sent" && in.State != "failed" && in.State != "unknown") {
			fail(w, 400, "result state must be sent, failed or unknown")
			return
		}
		m, err := s.Relay.Store.CompleteRelay(r.Context(), in.ID, in.LeaseToken, g.ID, relay.Result{State: in.State, Error: in.Error})
		if errors.Is(err, relay.ErrLease) {
			fail(w, 409, err.Error())
			return
		}
		if err != nil {
			internalError(w, err)
			return
		}
		s.Hub.Changed(m.Inbox, m.To, m.RunID)
		w.WriteHeader(204)
	default:
		fail(w, 404, "gateway endpoint not found")
	}
}

func (s *Server) twilioReceipt(w http.ResponseWriter, r *http.Request) {
	if s.Relay == nil || s.Relay.Config.AuthToken == "" || s.Relay.Config.PublicURL == "" {
		fail(w, 403, "Twilio receipts are not configured")
		return
	}
	if err := r.ParseForm(); err != nil {
		fail(w, 400, "invalid receipt")
		return
	}
	target := s.Relay.Config.PublicURL + r.URL.RequestURI()
	if !relay.VerifyTwilioSignature(s.Relay.Config.AuthToken, target, r.Header.Get("X-Twilio-Signature"), r.PostForm) {
		fail(w, 403, "invalid Twilio signature")
		return
	}
	state := r.PostForm.Get("MessageStatus")
	switch state {
	case "queued", "accepted", "sending":
		state = "accepted"
	case "sent", "delivered":
	case "failed", "undelivered":
		state = "failed"
	default:
		fail(w, 400, "unsupported receipt status")
		return
	}
	id := r.PostForm.Get("MessageSid")
	if !strings.HasPrefix(id, "SM") || len(id) > 128 {
		fail(w, 400, "invalid message SID")
		return
	}
	m, err := s.Relay.Store.RelayReceipt(r.Context(), r.URL.Query().Get("message_id"), relay.Result{State: state, ProviderID: id})
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "relay message not found")
		return
	}
	if errors.Is(err, relay.ErrLease) {
		fail(w, 409, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	s.Hub.Changed(m.Inbox, m.To, m.RunID)
	w.WriteHeader(204)
}
