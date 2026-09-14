// Package cli provides scriptable clients and local snapshot commands.
package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/storage"
)

func Run(ctx context.Context, args []string, out io.Writer) error {
	command := args[0]
	valid := map[string]bool{"send": true, "list": true, "wait": true, "purge": true, "export": true, "backup": true, "restore": true}
	if !valid[command] {
		return errors.New("commands: send, list, wait, purge, export, backup, restore; run with --help for flags")
	}
	f := flag.NewFlagSet("textdock "+command, flag.ContinueOnError)
	f.SetOutput(out)
	base := f.String("url", env("TEXTDOCK_URL", "http://127.0.0.1:18257"), "TextDock server origin")
	inbox := f.String("inbox", "local", "inbox ID")
	to := f.String("to", "", "recipient")
	from := f.String("from", "TextDock", "sender")
	mode := f.String("mode", "capture", "capture or simulate")
	direction := f.String("direction", "outbound", "outbound or inbound")
	scenario := f.String("scenario", "", "simulation scenario ID")
	callback := f.String("callback-url", "", "simulation webhook URL")
	body := f.String("body", "", "message body")
	run := f.String("run-id", "", "unique test run ID")
	query := f.String("q", "", "literal search")
	limit := f.Int("limit", 100, "page size (1–200)")
	cursor := f.String("cursor", "", "next-page cursor from list")
	tag := f.String("tag", "", "exact tag")
	favorite := f.Bool("favorite", false, "only favorites")
	otp := f.Bool("otp", false, "only detected codes")
	status := f.String("status", "", "message status filter")
	wait := f.Int("timeout", 30, "OTP wait in seconds (0–30)")
	format := f.String("format", "jsonl", "export format: jsonl, json, csv")
	dbPath := f.String("db", env("TEXTDOCK_DB", "data/textdock.db"), "database path (backup/restore only)")
	output := f.String("out", "", "new snapshot file (backup only)")
	source := f.String("source", "", "SQLite snapshot file (restore only)")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected argument: %s", f.Arg(0))
	}
	if command == "backup" || command == "restore" {
		if command == "restore" {
			return restore(*source, *dbPath)
		}
		if *output == "" {
			return errors.New("backup requires --out")
		}
		if _, err := os.Stat(*output); !errors.Is(err, os.ErrNotExist) {
			return errors.New("backup destination must be a new file")
		}
		if _, err := os.Stat(*dbPath); err != nil {
			return err
		}
		db, err := storage.Open(*dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.Backup(ctx, *output); err != nil {
			return err
		}
		return os.Chmod(*output, 0600)
	}
	u, err := url.Parse(*base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("url must be an HTTP(S) server origin")
	}
	client := &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, path string, payload any) ([]byte, error) {
		var input io.Reader
		if payload != nil {
			data, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			input = strings.NewReader(string(data))
		}
		r, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(*base, "/")+path, input)
		if err != nil {
			return nil, err
		}
		r.Header.Set("Content-Type", "application/json")
		if token := os.Getenv("TEXTDOCK_TOKEN"); token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(r)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
		if err != nil {
			return nil, err
		}
		if response.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
		}
		return data, nil
	}
	q := url.Values{"inbox": {*inbox}, "to": {*to}, "run_id": {*run}, "q": {*query}, "limit": {strconv.Itoa(*limit)}, "cursor": {*cursor}, "tag": {*tag}, "favorite": {strconv.FormatBool(*favorite)}, "otp": {strconv.FormatBool(*otp)}}
	var data []byte
	q.Set("status", *status)
	switch command {
	case "send":
		data, err = call("POST", "/api/v1/messages", message.Input{Inbox: *inbox, To: *to, From: *from, Body: *body, RunID: *run, Mode: *mode, Direction: *direction, ScenarioID: *scenario, CallbackURL: *callback})
	case "list":
		data, err = call("GET", "/api/v1/messages?"+q.Encode(), nil)
	case "wait":
		q.Set("timeout", strconv.Itoa(*wait))
		data, err = call("GET", "/api/v1/otp?"+q.Encode(), nil)
	case "purge":
		data, err = call("POST", "/api/v1/inboxes/"+url.PathEscape(*inbox)+"/purge", nil)
	case "export":
		if *format != "jsonl" && *format != "json" && *format != "csv" {
			return errors.New("format must be jsonl, json or csv")
		}
		writer := csv.NewWriter(out)
		if *format == "csv" {
			if err := writer.Write([]string{"id", "inbox", "to", "from", "body", "run_id", "created_at"}); err != nil {
				return err
			}
		}
		if *format == "json" {
			if _, err := io.WriteString(out, "["); err != nil {
				return err
			}
		}
		first := true
		for {
			data, err := call("GET", "/api/v1/messages?"+q.Encode(), nil)
			if err != nil {
				return err
			}
			var page struct {
				Messages []message.Message `json:"messages"`
				Next     string            `json:"next_cursor"`
			}
			if err := json.Unmarshal(data, &page); err != nil {
				return err
			}
			for _, m := range page.Messages {
				if *format == "csv" {
					row := []string{m.ID, m.Inbox, m.To, m.From, m.Body, m.RunID, m.CreatedAt.Format(time.RFC3339Nano)}
					for i, v := range row {
						if v != "" && strings.ContainsAny(v[:1], "=+-@\t\r\n") {
							row[i] = "'" + v
						}
					}
					if err := writer.Write(row); err != nil {
						return err
					}
				} else {
					if *format == "json" && !first {
						if _, err := io.WriteString(out, ","); err != nil {
							return err
						}
					}
					if err := json.NewEncoder(out).Encode(m); err != nil {
						return err
					}
				}
				first = false
			}
			if page.Next == "" {
				break
			}
			q.Set("cursor", page.Next)
		}
		if *format == "csv" {
			writer.Flush()
			return writer.Error()
		}
		if *format == "json" {
			_, err := io.WriteString(out, "]\n")
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func restore(source, destination string) error {
	if source == "" {
		return errors.New("restore requires --source; destination --db must not exist")
	}
	if err := storage.ValidateSnapshot(context.Background(), source); err != nil {
		return fmt.Errorf("validate snapshot: %w", err)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	var header [16]byte
	if _, err := io.ReadFull(input, header[:]); err != nil || string(header[:]) != "SQLite format 3\x00" {
		return errors.New("source is not a SQLite snapshot")
	}
	if _, err := input.Seek(0, 0); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
