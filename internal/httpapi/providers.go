package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/application"
	"github.com/fless-lab/TextDock/internal/message"
)

func (s *Server) providerAuthorized(w http.ResponseWriter, r *http.Request, credential string) bool {
	if token := bearer(r); token != "" {
		credential = token
	}
	if strings.HasPrefix(credential, "td_device_") || strings.HasPrefix(credential, "td_gateway_") {
		fail(w, 403, "paired devices cannot send messages")
		return false
	}
	if s.Token != "" {
		a, b := sha256.Sum256([]byte(credential)), sha256.Sum256([]byte(s.Token))
		if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
			fail(w, 401, "invalid TextDock token")
			return false
		}
	}
	return true
}

func (s *Server) providerMessage(w http.ResponseWriter, r *http.Request, in message.Input, source string) (message.Message, bool) {
	in.Inbox = r.Header.Get("X-TextDock-Inbox")
	if in.Inbox == "" {
		in.Inbox = "local"
	}
	in.RunID = r.Header.Get("X-TextDock-Run-ID")
	in.ScenarioID = r.Header.Get("X-TextDock-Scenario")
	ok, err := s.Workspaces.InboxExists(r.Context(), in.Inbox)
	if err != nil {
		internalError(w, err)
		return message.Message{}, false
	}
	if !ok {
		fail(w, 400, "inbox does not exist")
		return message.Message{}, false
	}
	m, err := (application.Capture{Messages: s.Store, Simulation: s.Simulation, OTPPattern: s.OTPPattern, Changed: s.Hub.Changed}).Send(r.Context(), in, source)
	if err != nil {
		var rejected application.Rejected
		if errors.As(err, &rejected) {
			w.Header().Set("Retry-After", strconv.Itoa(rejected.RetryAfter))
			fail(w, rejected.Status, rejected.Error())
		} else if errors.Is(err, message.ErrInvalid) || errors.Is(err, application.ErrRequest) {
			fail(w, 400, err.Error())
		} else {
			internalError(w, err)
		}
		return message.Message{}, false
	}
	return m, true
}

func (s *Server) vonage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		APIKey    string `json:"api_key"`
		APISecret string `json:"api_secret"`
		To        string `json:"to"`
		From      string `json:"from"`
		Text      string `json:"text"`
		Type      string `json:"type"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if r.ParseForm() != nil {
			fail(w, 400, "invalid form")
			return
		}
		for key := range r.PostForm {
			if key != "api_key" && key != "api_secret" && key != "to" && key != "from" && key != "text" && key != "type" {
				fail(w, 422, "unsupported Vonage field: "+key)
				return
			}
		}
		in.APIKey, in.APISecret, in.To, in.From, in.Text, in.Type = r.PostForm.Get("api_key"), r.PostForm.Get("api_secret"), r.PostForm.Get("to"), r.PostForm.Get("from"), r.PostForm.Get("text"), r.PostForm.Get("type")
	} else if decode(r, &in) != nil {
		fail(w, 400, "supported Vonage fields: api_key, api_secret, to, from, text, type")
		return
	}
	if !s.providerAuthorized(w, r, in.APISecret) {
		return
	}
	if in.Type != "" && in.Type != "text" && in.Type != "unicode" {
		fail(w, 422, "only text and unicode SMS are supported")
		return
	}
	if !strings.HasPrefix(in.To, "+") {
		in.To = "+" + in.To
	}
	m, ok := s.providerMessage(w, r, message.Input{To: in.To, From: in.From, Body: in.Text, ForceUnicode: in.Type == "unicode"}, "vonage")
	if !ok {
		return
	}
	id := sha256.Sum256([]byte(m.ID))
	writeJSON(w, 200, map[string]any{"message-count": "1", "messages": []map[string]string{{"to": strings.TrimPrefix(m.To, "+"), "message-id": hex.EncodeToString(id[:16]), "status": "0", "remaining-balance": "0", "message-price": "0", "network": ""}}})
}

func (s *Server) ovh(w http.ResponseWriter, r *http.Request) {
	if !s.providerAuthorized(w, r, r.Header.Get("X-Ovh-Consumer")) {
		return
	}
	var in struct {
		Receivers    []string `json:"receivers"`
		Sender       string   `json:"sender"`
		Message      string   `json:"message"`
		Priority     string   `json:"priority"`
		NoStopClause bool     `json:"noStopClause"`
	}
	if decode(r, &in) != nil || len(in.Receivers) != 1 {
		fail(w, 422, "OVH subset requires exactly one receiver and supports sender, message, priority, noStopClause")
		return
	}
	if in.NoStopClause {
		fail(w, 422, "noStopClause behavior is not emulated")
		return
	}
	if in.Priority != "" && in.Priority != "high" {
		fail(w, 422, "only the high priority subset is supported")
		return
	}
	m, ok := s.providerMessage(w, r, message.Input{To: in.Receivers[0], From: in.Sender, Body: in.Message}, "ovh")
	if !ok {
		return
	}
	hash := sha256.Sum256([]byte(m.ID))
	id := binary.BigEndian.Uint64(hash[:8]) >> 11
	writeJSON(w, 200, map[string]any{"ids": []uint64{id}, "invalidReceivers": []string{}, "validReceivers": []string{m.To}, "totalCreditsRemoved": 0})
}

func providerTime(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, time.Now().Unix()) }
