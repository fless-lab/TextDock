package httpapi

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/events"
	"github.com/fless-lab/TextDock/internal/message"
)

func (s *Server) createPair(w http.ResponseWriter, r *http.Request) {
	var scope connect.Scope
	if decode(r, &scope) != nil || !message.ValidRecipient(scope.To) || len(scope.RunID) > 128 {
		fail(w, 400, "to must be an E.164-shaped recipient; run_id is optional (max 128 bytes)")
		return
	}
	code, expires := rand.Text(), time.Now().UTC().Add(2*time.Minute)
	if err := s.Devices.CreatePair(r.Context(), connect.Hash(code), scope, expires); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"code": code, "expires_at": expires, "scope": scope})
}

func (s *Server) claimPair(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if decode(r, &in) != nil || len(in.Code) > 128 || strings.TrimSpace(in.Name) == "" || utf8.RuneCountInString(in.Name) > 64 {
		fail(w, 400, "code and a device name (1–64 characters) are required")
		return
	}
	token := "td_device_" + rand.Text()
	d, err := s.Devices.ClaimPair(r.Context(), connect.Hash(in.Code), connect.Hash(token), connect.Device{
		ID: "dev_" + rand.Text(), Name: strings.TrimSpace(in.Name), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour),
	})
	if errors.Is(err, connect.ErrInvalid) {
		fail(w, 401, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "device": d})
}

func bearer(r *http.Request) string {
	if v, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return v
	}
	return ""
}

func (s *Server) deviceAPI(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if !strings.HasPrefix(token, "td_device_") {
		fail(w, 401, "a paired device credential is required")
		return
	}
	d, err := s.Devices.DeviceByHash(r.Context(), connect.Hash(token))
	if errors.Is(err, connect.ErrInvalid) {
		fail(w, 401, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	if r.Method != http.MethodGet {
		fail(w, 403, "paired devices have read-only access")
		return
	}
	switch r.URL.Path {
	case "/connect/v1/session":
		writeJSON(w, 200, d)
	case "/connect/v1/messages":
		f, err := filter(r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if (f.To != "" && f.To != d.Scope.To) || (d.Scope.RunID != "" && f.RunID != "" && f.RunID != d.Scope.RunID) {
			fail(w, 403, "requested messages are outside this device's scope")
			return
		}
		f.To = d.Scope.To
		if d.Scope.RunID != "" {
			f.RunID = d.Scope.RunID
		}
		items, err := s.Store.List(r.Context(), f)
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"messages": items, "limit": f.Limit})
	case "/connect/v1/events":
		s.stream(w, r, events.Filter{To: d.Scope.To, RunID: d.Scope.RunID, DeviceID: d.ID}, d.ExpiresAt)
	default:
		fail(w, 404, "device endpoint not found")
	}
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, filter events.Filter, expires time.Time) {
	changes, cancel := s.Hub.Subscribe(filter)
	defer cancel()
	// Subscribe before rechecking: a revocation between authentication and
	// subscribing must not leave a stream alive until its expiry.
	if filter.DeviceID != "" {
		if _, err := s.Devices.DeviceByHash(r.Context(), connect.Hash(bearer(r))); err != nil {
			fail(w, 401, "device session ended")
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	send := func(event string) error {
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event); err != nil {
			return err
		}
		err := controller.Flush()
		_ = controller.SetWriteDeadline(time.Time{})
		return err
	}
	if err := send("sync"); err != nil {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	var expiration <-chan time.Time
	if !expires.IsZero() {
		timer := time.NewTimer(time.Until(expires))
		defer timer.Stop()
		expiration = timer.C
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-expiration:
			_ = send("revoked")
			return
		case <-heartbeat.C:
			if send("heartbeat") != nil {
				return
			}
		case event := <-changes:
			if send(event) != nil || event == "revoked" {
				return
			}
		}
	}
}

func (s *Server) network(w http.ResponseWriter, r *http.Request) {
	host, port, _ := net.SplitHostPort(s.Listen)
	ip := net.ParseIP(host)
	enabled := ip != nil && !ip.IsLoopback() && s.Token != ""
	urls := make([]string, 0)
	if s.PublicURL != "" {
		urls = append(urls, s.PublicURL)
		enabled = true
	}
	if enabled && (ip == nil || ip.IsUnspecified()) {
		addresses, _ := net.InterfaceAddrs()
		for _, addr := range addresses {
			candidate, _, err := net.ParseCIDR(addr.String())
			if err == nil && candidate.To4() != nil && candidate.IsPrivate() {
				urls = append(urls, "http://"+net.JoinHostPort(candidate.String(), port))
			}
		}
	} else if enabled && host != "" {
		urls = append(urls, "http://"+net.JoinHostPort(host, port))
	}
	writeJSON(w, 200, map[string]any{"lan_enabled": enabled, "urls": urls})
}
