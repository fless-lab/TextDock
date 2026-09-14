package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/fless-lab/TextDock/internal/storage"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"testing/fstest"
)

func TestCustomOTPCandidateOverridesNumericHeuristic(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := httptest.NewServer((&Server{Store: store, Devices: store, Workspaces: store, UI: fstest.MapFS{}, OTPPattern: regexp.MustCompile(`code=([A-Z0-9]{6})`)}).Handler())
	defer s.Close()
	r := request(t, s, "POST", "/api/v1/messages", `{"to":"+33612345678","from":"Acme","body":"Reference 123456; code=AB12CD","run_id":"custom"}`, "")
	if r.StatusCode != 201 {
		t.Fatalf("custom capture: %d", r.StatusCode)
	}
	r = request(t, s, "GET", "/api/v1/otp?to=%2B33612345678&run_id=custom", "", "")
	var result map[string]string
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != "AB12CD" {
		t.Fatalf("custom OTP: %+v", result)
	}
}

func TestWorkspacePaginationFavoritesAndScopedPhone(t *testing.T) {
	s, _ := setup(t, "")
	r := request(t, s, "POST", "/api/v1/projects", `{"name":"Checkout"}`, "")
	if r.StatusCode != 201 {
		t.Fatalf("project: %d", r.StatusCode)
	}
	var project struct {
		Inbox struct {
			ID string `json:"id"`
		} `json:"inbox"`
	}
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		t.Fatal(err)
	}
	inbox := project.Inbox.ID
	var lastID string
	for i := range 7 {
		r = request(t, s, "POST", "/api/v1/messages", fmt.Sprintf(`{"inbox":%q,"to":"+33612345678","from":"Acme","body":"Code 123456 %d","run_id":"paired-run"}`, inbox, i), "")
		if r.StatusCode != 201 {
			t.Fatalf("capture: %d", r.StatusCode)
		}
		var m struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&m)
		lastID = m.ID
	}
	seen := map[string]bool{}
	next := ""
	for {
		r = request(t, s, "GET", "/api/v1/messages?inbox="+inbox+"&limit=3&cursor="+url.QueryEscape(next), "", "")
		var p struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			Next string `json:"next_cursor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Fatal(err)
		}
		if len(p.Messages) > 3 {
			t.Fatal("unbounded page")
		}
		for _, m := range p.Messages {
			if seen[m.ID] {
				t.Fatal("duplicate across pages")
			}
			seen[m.ID] = true
		}
		if p.Next == "" {
			break
		}
		next = p.Next
	}
	if len(seen) != 7 {
		t.Fatalf("pagination lost messages: %d", len(seen))
	}
	r = request(t, s, "PATCH", "/api/v1/messages/"+lastID, `{"favorite":true,"tags":["signup","signup"]}`, "")
	if r.StatusCode != 200 {
		t.Fatalf("metadata: %d", r.StatusCode)
	}
	r = request(t, s, "GET", "/api/v1/messages?inbox="+inbox+"&favorite=true&tag=signup", "", "")
	var list struct {
		Messages []struct {
			Tags []string `json:"tags"`
		} `json:"messages"`
	}
	json.NewDecoder(r.Body).Decode(&list)
	if len(list.Messages) != 1 || len(list.Messages[0].Tags) != 1 {
		t.Fatalf("favorite/tag filter: %+v", list)
	}
	// A device paired to local must not see the same recipient/run in Checkout.
	token, _ := pairPhone(t, s, "")
	r = request(t, s, "GET", "/connect/v1/messages", "", token)
	json.NewDecoder(r.Body).Decode(&list)
	if len(list.Messages) != 0 {
		t.Fatal("cross-inbox device leakage")
	}
	if r = request(t, s, "GET", "/connect/v1/messages?inbox="+inbox, "", token); r.StatusCode != 403 {
		t.Fatalf("device inbox bypass: %d", r.StatusCode)
	}
	if r = request(t, s, "GET", "/api/v1/messages?cursor=broken", "", ""); r.StatusCode != 400 {
		t.Fatalf("bad cursor: %d", r.StatusCode)
	}
	r = request(t, s, "POST", "/api/v1/inboxes/"+inbox+"/purge", "", "")
	if r.StatusCode != 200 {
		t.Fatalf("purge: %d", r.StatusCode)
	}
}
