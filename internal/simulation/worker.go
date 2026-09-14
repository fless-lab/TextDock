package simulation

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Worker struct {
	Store   Repository
	Secret  string
	Changed func(string, string, string)
}

func (w Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		for range 50 {
			err := w.Step(ctx, time.Now().UTC())
			if errors.Is(err, sql.ErrNoRows) || ctx.Err() != nil {
				break
			}
			if err != nil {
				slog.Error("simulation worker", "error", err)
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w Worker) Step(ctx context.Context, now time.Time) error {
	job, err := w.Store.NextJob(ctx, now)
	if err != nil {
		return err
	}
	if job.Kind == "transition" {
		m, err := w.Store.CompleteTransition(ctx, job, now)
		if err == nil && w.Changed != nil {
			w.Changed(m.Inbox, m.To, m.RunID)
		}
		return err
	}
	var delivery Delivery
	if err := json.Unmarshal([]byte(job.Payload), &delivery); err != nil {
		return err
	}
	data, contentType, signature := FormatDelivery(delivery, w.Secret)
	attempt := Attempt{JobID: job.ID, MessageID: job.MessageID, At: now, URL: delivery.URL, Request: data, ContentType: contentType}
	started := time.Now()
	r, err := http.NewRequestWithContext(ctx, "POST", delivery.URL, strings.NewReader(data))
	if err == nil {
		r.Header.Set("Content-Type", contentType)
		r.Header.Set("X-TextDock-Event-ID", job.ID)
		if w.Secret != "" {
			if delivery.Format == "twilio" {
				r.Header.Set("X-Twilio-Signature", signature)
			} else {
				r.Header.Set("X-TextDock-Signature", signature)
			}
		}
		attempt.Headers = make(map[string]string)
		for key := range r.Header {
			attempt.Headers[key] = r.Header.Get(key)
		}
		client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, requestErr := client.Do(r)
		err = requestErr
		if response != nil {
			attempt.Status = response.StatusCode
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 4097))
			response.Body.Close()
			if len(body) > 4096 {
				body = body[:4096]
				attempt.ResponseTruncated = true
			}
			attempt.Response = string(body)
			if readErr != nil && err == nil {
				err = readErr
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	} // Lease recovery handles interrupted deliveries.
	if err != nil {
		attempt.Error = err.Error()
	}
	attempt.DurationMS = time.Since(started).Milliseconds()
	success := err == nil && attempt.Status >= 200 && attempt.Status < 300
	err = w.Store.FinishWebhook(ctx, job, attempt, success, now.Add(time.Duration(1<<min(job.Attempts, 6))*time.Second))
	if err == nil && w.Changed != nil {
		w.Changed(delivery.Message.Inbox, delivery.Message.To, delivery.Message.RunID)
	}
	return err
}

func FormatDelivery(d Delivery, secret string) (string, string, string) {
	if d.Format == "twilio" {
		sid := ProviderSID(d.Message.ID)
		values := url.Values{"MessageSid": {sid}, "SmsSid": {sid}, "MessageStatus": {d.Message.Status}, "SmsStatus": {d.Message.Status}, "To": {d.Message.To}, "From": {d.Message.From}}
		if d.Message.Direction == "inbound" {
			values.Set("Body", d.Message.Body)
		}
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		canonical := d.URL
		for _, key := range keys {
			canonical += key + values.Get(key)
		}
		mac := hmac.New(sha1.New, []byte(secret))
		_, _ = mac.Write([]byte(canonical))
		return values.Encode(), "application/x-www-form-urlencoded", base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	data, _ := json.Marshal(map[string]any{"event_id": strconv.FormatInt(d.EventID, 10), "type": "sms." + d.Message.Status, "message": d.Message})
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(data)
	return string(data), "application/json", "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
func ProviderSID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "SM" + hex.EncodeToString(sum[:16])
}
