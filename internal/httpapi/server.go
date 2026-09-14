package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/events"
	"github.com/fless-lab/TextDock/internal/message"
)

type Server struct {
	Store     message.Repository
	Token     string
	Version   string
	UI        fs.FS
	Devices   connect.Repository
	Hub       events.Hub
	Listen    string
	PublicURL string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.Handle("/api/", s.authorize(http.HandlerFunc(s.api)))
	mux.HandleFunc("POST /connect/v1/claim", s.claimPair)
	mux.HandleFunc("/connect/v1/", s.deviceAPI)
	mux.Handle("POST /2010-04-01/Accounts/{account}/Messages.json", s.authorize(http.HandlerFunc(s.twilio)))
	files := http.FileServer(http.FS(s.UI))
	mux.HandleFunc("GET /phone", func(w http.ResponseWriter, r *http.Request) { r.URL.Path = "/"; files.ServeHTTP(w, r) })
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			fail(w, 405, "method not allowed")
			return
		}
		files.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Token == "" {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			ip := net.ParseIP(host)
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				fail(w, 403, "unauthenticated access requires a loopback host")
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		// Prevent cross-origin browser access even when loopback auth is disabled.
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
				fail(w, 403, "cross-origin requests are not allowed")
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "cross-site requests are not allowed")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A device token never grants desktop access, including on loopback
		// instances whose desktop API otherwise permits anonymous requests.
		if strings.HasPrefix(bearer(r), "td_device_") {
			fail(w, 403, "device credentials cannot access the desktop API")
			return
		}
		if s.Token != "" {
			got := bearer(r)
			if strings.HasPrefix(r.URL.Path, "/2010-04-01/") {
				_, password, ok := r.BasicAuth()
				if ok {
					got = password
				}
			}
			a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(s.Token))
			if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
				fail(w, 401, "a valid TextDock token is required")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/network":
		s.network(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/v1/events":
		s.stream(w, r, events.Filter{}, time.Time{})
	case r.Method == "POST" && r.URL.Path == "/api/v1/pairings":
		s.createPair(w, r)
	case r.Method == "GET" && r.URL.Path == "/api/v1/devices":
		devices, err := s.Devices.ListDevices(r.Context())
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"devices": devices})
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/devices/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
		ok, err := s.Devices.RevokeDevice(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return
		}
		if !ok {
			fail(w, 404, "device not found")
			return
		}
		s.Hub.Revoke(id)
		w.WriteHeader(204)
	case r.Method == "GET" && r.URL.Path == "/api/v1/info":
		writeJSON(w, 200, map[string]any{"name": "TextDock", "version": s.Version, "mode": "capture", "auth_enabled": s.Token != ""})
	case r.Method == "POST" && r.URL.Path == "/api/v1/messages":
		var in message.Input
		if err := decode(r, &in); err != nil {
			fail(w, 400, "invalid JSON: use to, from, body and optional run_id")
			return
		}
		s.capture(w, r, in, "api", false)
	case r.Method == "GET" && r.URL.Path == "/api/v1/messages":
		f, err := filter(r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		items, err := s.Store.List(r.Context(), f)
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"messages": items, "limit": f.Limit})
	case r.Method == "GET" && r.URL.Path == "/api/v1/otp":
		s.waitOTP(w, r)
	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/messages/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/messages/")
		m, err := s.Store.Get(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "message not found")
			return
		}
		if err != nil {
			internalError(w, err)
			return
		}
		ok, err := s.Store.Delete(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return
		}
		if !ok {
			fail(w, 404, "message not found")
			return
		}
		s.Hub.Changed(m.To, m.RunID)
		w.WriteHeader(204)
	default:
		fail(w, 404, "endpoint not found")
	}
}

func (s *Server) capture(w http.ResponseWriter, r *http.Request, in message.Input, source string, twilio bool) {
	m, err := message.New(in, source)
	if errors.Is(err, message.ErrInvalid) {
		fail(w, 400, "to must be E.164-shaped (+ and 7–15 digits); from is required (max 64 characters); body is required (max 4096 characters); run_id max 128 bytes")
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	if err := s.Store.Save(r.Context(), m); err != nil {
		internalError(w, err)
		return
	}
	s.Hub.Changed(m.To, m.RunID)
	if twilio {
		sid := fmt.Sprintf("SM%x", sha256.Sum256([]byte(m.ID)))[:34]
		writeJSON(w, 201, map[string]any{
			"sid": sid, "account_sid": r.PathValue("account"), "to": m.To, "from": m.From,
			"body": m.Body, "status": "queued", "direction": "outbound-api",
			"num_segments": strconv.Itoa(m.Analysis.Segments), "error_code": nil,
			"error_message": nil, "date_created": m.CreatedAt.Format(time.RFC1123Z),
		})
		return
	}
	writeJSON(w, 201, m)
}

func (s *Server) twilio(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		fail(w, 415, "use application/x-www-form-urlencoded")
		return
	}
	if err := r.ParseForm(); err != nil {
		fail(w, 400, "invalid form")
		return
	}
	for key := range r.PostForm {
		if key != "To" && key != "From" && key != "Body" {
			fail(w, 422, "unsupported Twilio parameter: "+key)
			return
		}
	}
	s.capture(w, r, message.Input{To: r.PostForm.Get("To"), From: r.PostForm.Get("From"), Body: r.PostForm.Get("Body")}, "twilio", true)
}

func filter(r *http.Request) (message.Filter, error) {
	q := r.URL.Query()
	f := message.Filter{Query: q.Get("q"), To: q.Get("to"), RunID: q.Get("run_id"), Limit: 100}
	if q.Has("limit") {
		n, err := strconv.Atoi(q.Get("limit"))
		if err != nil || n < 1 || n > 200 {
			return f, errors.New("limit must be 1–200")
		}
		f.Limit = n
	}
	if q.Has("since") {
		t, err := time.Parse(time.RFC3339Nano, q.Get("since"))
		if err != nil {
			return f, errors.New("since must be an RFC3339 timestamp")
		}
		f.Since = t
	}
	return f, nil
}

func (s *Server) waitOTP(w http.ResponseWriter, r *http.Request) {
	f, err := filter(r)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if f.To == "" || f.RunID == "" {
		fail(w, 400, "to and a unique run_id are required to avoid stale OTPs")
		return
	}
	wait := 0
	if r.URL.Query().Has("timeout") {
		wait, err = strconv.Atoi(r.URL.Query().Get("timeout"))
		if err != nil || wait < 0 || wait > 30 {
			fail(w, 400, "timeout must be 0–30 seconds")
			return
		}
	}
	deadline := time.NewTimer(time.Duration(wait) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		items, err := s.Store.List(r.Context(), f)
		if err != nil {
			internalError(w, err)
			return
		}
		for _, m := range items {
			if m.Analysis.OTP != "" {
				writeJSON(w, 200, map[string]string{"code": m.Analysis.OTP, "message_id": m.ID})
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			fail(w, 404, "no matching OTP received before timeout")
			return
		case <-ticker.C:
		}
	}
}

func decode(r *http.Request, out any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, code int, text string) {
	writeJSON(w, code, map[string]string{"error": text})
}

func internalError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	slog.Error("request failed", "error", err)
	fail(w, 500, "internal server error")
}
