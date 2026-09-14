package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fless-lab/TextDock/internal/relay"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestRealRelayRequiresOptInAndGatewayScope(t *testing.T) {
	s, _ := setup(t, "")
	if r := request(t, s, "POST", "/api/v1/messages", `{"mode":"relay","to":"+33612345678","from":"Acme","body":"Code 123456"}`, ""); r.StatusCode != 400 {
		t.Fatalf("disabled relay: %d", r.StatusCode)
	}
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer((&Server{Store: db, Devices: db, Workspaces: db, Simulation: db, Relay: &relay.Service{Store: db, Config: relay.Config{Driver: "android", Limit: 10}}, UI: fstest.MapFS{}}).Handler())
	defer server.Close()
	r := request(t, server, "POST", "/api/v1/gateways", `{"name":"Android"}`, "")
	var enrollment struct {
		Token   string        `json:"token"`
		Gateway relay.Gateway `json:"gateway"`
	}
	json.NewDecoder(r.Body).Decode(&enrollment)
	if !strings.HasPrefix(enrollment.Token, "td_gateway_") {
		t.Fatalf("enrollment: %d", r.StatusCode)
	}
	if r := request(t, server, "GET", "/api/v1/messages", "", enrollment.Token); r.StatusCode != 403 {
		t.Fatal("gateway credential opened admin inbox")
	}
	if r := request(t, server, "POST", "/relay/v1/jobs/claim", `{}`, ""); r.StatusCode != 401 {
		t.Fatal("anonymous gateway claim")
	}
	request(t, server, "POST", "/api/v1/messages", `{"mode":"relay","to":"+33612345678","from":"Acme","body":"Code 123456"}`, "")
	r = request(t, server, "POST", "/relay/v1/jobs/claim", `{}`, enrollment.Token)
	var job relay.Job
	json.NewDecoder(r.Body).Decode(&job)
	if job.LeaseToken == "" || job.Body != "Code 123456" {
		t.Fatalf("claim: %+v", job)
	}
	result, _ := json.Marshal(map[string]string{"id": job.ID, "lease_token": job.LeaseToken, "state": "sent"})
	if r := request(t, server, "POST", "/relay/v1/jobs/result", string(result), enrollment.Token); r.StatusCode != 204 {
		t.Fatalf("gateway result: %d", r.StatusCode)
	}
	request(t, server, "DELETE", "/api/v1/gateways/"+enrollment.Gateway.ID, "", "")
	if r := request(t, server, "POST", "/relay/v1/jobs/claim", `{}`, enrollment.Token); r.StatusCode != 401 {
		t.Fatal("revoked gateway claim")
	}
}

func TestSignedRelayReceiptDoesNotRegressDelivery(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := relay.Config{Driver: "twilio", AuthToken: "receipt-secret", PublicURL: "https://public.example", Limit: 10}
	s := httptest.NewServer((&Server{Store: db, Workspaces: db, Devices: db, Simulation: db, Relay: &relay.Service{Store: db, Config: cfg}, UI: fstest.MapFS{}}).Handler())
	defer s.Close()
	r := request(t, s, "POST", "/api/v1/messages", `{"mode":"relay","to":"+33612345678","from":"Acme","body":"Code 123456"}`, "")
	var m struct {
		ID string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&m)
	job, err := db.ClaimRelay(context.Background(), "twilio", "server", time.Now().UTC(), 10)
	if err != nil {
		t.Fatal(err)
	}
	path := "/relay/twilio/status?message_id=" + url.QueryEscape(m.ID)
	for _, status := range []string{"delivered", "sent"} {
		form := url.Values{"MessageSid": {"SMtest"}, "MessageStatus": {status}}
		canonical := cfg.PublicURL + path + "MessageSidSMtestMessageStatus" + status
		mac := hmac.New(sha1.New, []byte(cfg.AuthToken))
		mac.Write([]byte(canonical))
		req, _ := http.NewRequest("POST", s.URL+path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Twilio-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		response, err := s.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 204 {
			t.Fatalf("signed receipt: %d", response.StatusCode)
		}
	}
	// The HTTP acceptance response may arrive after an early delivery callback.
	if _, err := db.CompleteRelay(context.Background(), job.ID, job.LeaseToken, "server", relay.Result{State: "accepted", ProviderID: "SMtest"}); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Get(context.Background(), m.ID)
	if got.Status != "delivered" {
		t.Fatal("delivery state regressed")
	}
	if r := request(t, s, "POST", path, `{}`, ""); r.StatusCode != 403 {
		t.Fatalf("unsigned callback: %d", r.StatusCode)
	}
}
