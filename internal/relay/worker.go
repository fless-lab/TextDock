package relay

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Worker struct {
	Store   Repository
	Config  Config
	Changed func(string, string, string)
	Resync  func()
}

func (w Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if n, err := w.Store.ExpireRelays(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
			slog.Error("relay lease recovery", "error", err)
		} else if n > 0 && w.Resync != nil {
			w.Resync()
		}
		if w.Config.Driver == "twilio" {
			if err := w.Step(ctx); err != nil && !errors.Is(err, sql.ErrNoRows) && ctx.Err() == nil {
				slog.Error("relay worker", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w Worker) Step(ctx context.Context) error {
	job, err := w.Store.ClaimRelay(ctx, "twilio", "server", time.Now().UTC(), w.Config.Limit)
	if err != nil {
		return err
	}
	if w.Changed != nil {
		w.Changed(job.Inbox, job.To, job.RunID)
	}
	result := w.send(ctx, job)
	// Shutdown can happen after provider acceptance. Lease recovery records
	// unknown rather than making a second carrier request.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	m, err := w.Store.CompleteRelay(ctx, job.ID, job.LeaseToken, "server", result)
	if err == nil && w.Changed != nil {
		w.Changed(m.Inbox, m.To, m.RunID)
	}
	return err
}
func (w Worker) send(ctx context.Context, job Job) Result {
	form := url.Values{"To": {job.To}, "From": {job.From}, "Body": {job.Body}}
	if w.Config.PublicURL != "" {
		form.Set("StatusCallback", w.Config.PublicURL+"/relay/twilio/status?message_id="+url.QueryEscape(job.MessageID))
	}
	target := strings.TrimSuffix(w.Config.Endpoint, "/") + "/2010-04-01/Accounts/" + url.PathEscape(w.Config.AccountSID) + "/Messages.json"
	r, err := http.NewRequestWithContext(ctx, "POST", target, strings.NewReader(form.Encode()))
	if err != nil {
		return Result{State: "failed", Error: "invalid provider endpoint"}
	}
	r.SetBasicAuth(w.Config.AccountSID, w.Config.AuthToken)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(r)
	if err != nil {
		return Result{State: "unknown", Error: "provider request interrupted; acceptance is unknown"}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 {
		return Result{State: "unknown", Error: "unable to read provider response"}
	}
	var body struct {
		SID     string `json:"sid"`
		Message string `json:"message"`
	}
	decodeErr := json.Unmarshal(data, &body)
	if response.StatusCode >= 200 && response.StatusCode < 300 && decodeErr == nil && body.SID != "" {
		return Result{State: "accepted", ProviderID: body.SID}
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		text := "provider rejected the request"
		if body.Message != "" {
			text = body.Message
		}
		if len(text) > 512 {
			text = text[:512]
		}
		return Result{State: "failed", Error: text}
	}
	return Result{State: "unknown", Error: "unexpected provider response; do not automatically resend"}
}

func VerifyTwilioSignature(secret, target, signature string, values url.Values) bool {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	canonical := target
	for _, key := range keys {
		entries := append([]string(nil), values[key]...)
		sort.Strings(entries)
		for _, value := range entries {
			canonical += key + value
		}
	}
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(canonical))
	got, err := base64.StdEncoding.DecodeString(signature)
	return err == nil && hmac.Equal(got, mac.Sum(nil))
}
