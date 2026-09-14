package httpapi

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/workspace"
)

type cursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func encodeCursor(m message.Message) string {
	data, _ := json.Marshal(cursor{m.CreatedAt, m.ID})
	return base64.RawURLEncoding.EncodeToString(data)
}
func decodeCursor(raw string, f *message.Filter) error {
	if raw == "" {
		return nil
	}
	if len(raw) > 512 {
		return errors.New("invalid cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	var c cursor
	if err != nil || json.Unmarshal(data, &c) != nil || c.At.IsZero() || c.ID == "" || len(c.ID) > 128 {
		return errors.New("invalid cursor")
	}
	f.Before, f.BeforeID = c.At, c.ID
	return nil
}

func (s *Server) page(r *http.Request, f message.Filter) ([]message.Message, string, error) {
	limit := f.Limit
	f.Limit++
	items, err := s.Store.List(r.Context(), f)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = encodeCursor(items[len(items)-1])
	}
	return items, next, nil
}

func (s *Server) workspaceAPI(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/v1/workspaces":
		projects, err := s.Workspaces.ListProjects(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		inboxes, err := s.Workspaces.ListInboxes(r.Context())
		if err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 200, map[string]any{"projects": projects, "inboxes": inboxes})
	case r.Method == "POST" && r.URL.Path == "/api/v1/projects":
		var in struct {
			Name string `json:"name"`
		}
		if decode(r, &in) != nil || !validName(in.Name) {
			fail(w, 400, "name must have 1–80 characters")
			return true
		}
		p := workspace.Project{ID: "prj_" + rand.Text(), Name: strings.TrimSpace(in.Name)}
		box := workspace.Inbox{ID: "inbox_" + rand.Text(), ProjectID: p.ID, Name: "Inbox"}
		if err := s.Workspaces.CreateProject(r.Context(), p, box); err != nil {
			internalError(w, err)
			return true
		}
		writeJSON(w, 201, map[string]any{"project": p, "inbox": box})
	case r.Method == "POST" && r.URL.Path == "/api/v1/inboxes":
		var in struct {
			Name      string `json:"name"`
			ProjectID string `json:"project_id"`
		}
		if decode(r, &in) != nil || !validName(in.Name) || in.ProjectID == "" {
			fail(w, 400, "name and project_id are required")
			return true
		}
		box := workspace.Inbox{ID: "inbox_" + rand.Text(), ProjectID: in.ProjectID, Name: strings.TrimSpace(in.Name)}
		if err := s.Workspaces.CreateInbox(r.Context(), box); err != nil {
			fail(w, 400, "unable to create inbox: check project_id")
			return true
		}
		writeJSON(w, 201, box)
	case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v1/inboxes/") && strings.HasSuffix(r.URL.Path, "/purge"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/inboxes/"), "/purge")
		n, err := s.Store.Purge(r.Context(), id)
		if err != nil {
			internalError(w, err)
			return true
		}
		s.Hub.Resync()
		writeJSON(w, 200, map[string]int64{"deleted": n})
	case r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/api/v1/messages/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/messages/")
		var patch message.Update
		if decode(r, &patch) != nil || (patch.Favorite == nil && patch.Tags == nil) {
			fail(w, 400, "provide favorite or tags")
			return true
		}
		if patch.Tags != nil {
			if len(*patch.Tags) > 10 {
				fail(w, 400, "maximum 10 tags per message")
				return true
			}
			for i, tag := range *patch.Tags {
				tag = strings.TrimSpace(tag)
				if tag == "" || utf8.RuneCountInString(tag) > 32 {
					fail(w, 400, "tags must have 1–32 characters")
					return true
				}
				(*patch.Tags)[i] = tag
			}
			slices.Sort(*patch.Tags)
			unique := slices.Compact(*patch.Tags)
			patch.Tags = &unique
		}
		m, err := s.Store.Update(r.Context(), id, patch)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404, "message not found")
			return true
		}
		if err != nil {
			internalError(w, err)
			return true
		}
		s.Hub.Changed(m.Inbox, m.To, m.RunID)
		writeJSON(w, 200, m)
	case r.Method == "POST" && r.URL.Path == "/api/v1/messages/delete":
		var in struct {
			IDs []string `json:"ids"`
		}
		if decode(r, &in) != nil || len(in.IDs) == 0 || len(in.IDs) > 200 {
			fail(w, 400, "provide 1–200 message IDs")
			return true
		}
		deleted := 0
		for _, id := range in.IDs {
			m, err := s.Store.Get(r.Context(), id)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				internalError(w, err)
				return true
			}
			ok, err := s.Store.Delete(r.Context(), id)
			if err != nil {
				internalError(w, err)
				return true
			}
			if ok {
				deleted++
				s.Hub.Changed(m.Inbox, m.To, m.RunID)
			}
		}
		writeJSON(w, 200, map[string]int{"deleted": deleted})
	case r.Method == "GET" && r.URL.Path == "/api/v1/export":
		f, err := filter(r)
		if err != nil {
			fail(w, 400, err.Error())
			return true
		}
		items, next, err := s.page(r, f)
		if err != nil {
			internalError(w, err)
			return true
		}
		if r.URL.Query().Get("format") != "csv" {
			w.Header().Set("Content-Disposition", `attachment; filename="textdock.json"`)
			writeJSON(w, 200, map[string]any{"messages": items, "next_cursor": next})
			return true
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="textdock.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"id", "inbox", "to", "from", "body", "run_id", "created_at", "encoding", "segments", "tags", "favorite"})
		for _, m := range items {
			row := []string{m.ID, m.Inbox, m.To, m.From, m.Body, m.RunID, m.CreatedAt.Format(time.RFC3339Nano), m.Analysis.Encoding, strconv.Itoa(m.Analysis.Segments), strings.Join(m.Tags, ","), strconv.FormatBool(m.Favorite)}
			// Neutralize spreadsheet formula prefixes in developer-supplied text.
			for i, value := range row {
				if strings.ContainsAny(first(value), "=+-@\t\r\n") {
					row[i] = "'" + value
				}
			}
			_ = writer.Write(row)
		}
		writer.Flush()
	default:
		return false
	}
	return true
}
func first(s string) string {
	if s == "" {
		return ""
	}
	return s[:1]
}
func validName(s string) bool { return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 80 }
