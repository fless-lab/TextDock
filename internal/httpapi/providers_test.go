package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestVonageAndOVHCaptureSubsets(t *testing.T) {
	const token = "provider-test-token"
	s, _ := setup(t, token)
	r := request(t, s, "POST", "/sms/json", `{"api_key":"fake","api_secret":"provider-test-token","to":"33612345678","from":"Acme","text":"Code 123456","type":"unicode"}`, "")
	if r.StatusCode != 200 {
		t.Fatalf("Vonage capture: %d", r.StatusCode)
	}
	var vonage map[string]any
	if err := json.NewDecoder(r.Body).Decode(&vonage); err != nil {
		t.Fatal(err)
	}
	if vonage["message-count"] != "1" {
		t.Fatalf("Vonage shape: %+v", vonage)
	}
	req, _ := http.NewRequest("POST", s.URL+"/1.0/sms/local-service/jobs", strings.NewReader(`{"receivers":["+33612345678"],"sender":"Acme","message":"OVH code 654321","priority":"high","noStopClause":false}`))
	req.Header.Set("X-Ovh-Consumer", token)
	req.Header.Set("Content-Type", "application/json")
	response, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("OVH capture: %d", response.StatusCode)
	}
	var ovh struct {
		IDs []uint64 `json:"ids"`
	}
	if err := json.NewDecoder(response.Body).Decode(&ovh); err != nil || len(ovh.IDs) != 1 {
		t.Fatalf("OVH shape: %+v %v", ovh, err)
	}
	r = request(t, s, "GET", "/api/v1/messages", "", token)
	var list struct {
		Messages []struct {
			Source   string `json:"source"`
			Body     string `json:"body"`
			Analysis struct {
				Encoding string `json:"encoding"`
			} `json:"analysis"`
		} `json:"messages"`
	}
	json.NewDecoder(r.Body).Decode(&list)
	if len(list.Messages) != 2 || list.Messages[0].Source != "ovh" || list.Messages[1].Analysis.Encoding != "UTF-16" {
		t.Fatalf("provider normalization: %+v", list)
	}
	if r := request(t, s, "POST", "/sms/json", `{"api_secret":"wrong","to":"33612345678","from":"Acme","text":"no"}`, ""); r.StatusCode != 401 {
		t.Fatalf("provider auth: %d", r.StatusCode)
	}
	if r := request(t, s, "POST", "/1.0/sms/x/jobs", `{"receivers":["+33612345678","+33699999999"],"sender":"Acme","message":"no partial batch"}`, token); r.StatusCode != 422 {
		t.Fatalf("unsupported batch: %d", r.StatusCode)
	}
}
