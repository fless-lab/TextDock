package httpapi

import (
	"bufio"
	"context"
	"github.com/fless-lab/TextDock/internal/workspace"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIKeyStreamIgnoresOtherInbox(t *testing.T) {
	s, store := setup(t, "operator")
	if err := store.CreateProject(context.Background(), workspace.Project{ID: "stream", Name: "Stream"}, workspace.Inbox{ID: "stream-inbox", ProjectID: "stream", Name: "Stream"}); err != nil {
		t.Fatal(err)
	}
	_, token := issueKey(t, s, "operator", "stream", "stream-inbox", "messages:read")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	for i := 0; i < 3; i++ {
		if !scanner.Scan() {
			t.Fatal("initial event missing")
		}
	}
	events := make(chan string, 4)
	go func() {
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "event:") {
				events <- scanner.Text()
			}
		}
		close(events)
	}()
	send := func(inbox string) {
		r := request(t, s, "POST", "/api/v1/messages", `{"inbox":"`+inbox+`","to":"+12025550123","from":"Test","body":"Code"}`, "operator")
		if r.StatusCode != 201 {
			t.Fatal(r.StatusCode)
		}
	}
	send("local")
	select {
	case event := <-events:
		t.Fatal("other inbox emitted an event", event)
	case <-time.After(100 * time.Millisecond):
	}
	send("stream-inbox")
	select {
	case event := <-events:
		if event != "event: sync" {
			t.Fatal(event)
		}
	case <-ctx.Done():
		t.Fatal("scoped update missing")
	}
}
