package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type Service struct {
	Config Config
	Keys   Keys
	Store  Repository
	Client *http.Client
}

func (s *Service) Run(ctx context.Context) {
	if !s.Config.Enabled {
		<-ctx.Done()
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for range 20 {
			err := s.Step(ctx)
			if errors.Is(err, sql.ErrNoRows) || ctx.Err() != nil {
				break
			}
			if err != nil {
				slog.Error("push worker", "error", err)
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
func (s *Service) Step(ctx context.Context) error {
	if !s.Config.Enabled {
		return sql.ErrNoRows
	}
	job, err := s.Store.ClaimPush(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	active, err := s.Store.PushJobActive(ctx, job)
	if err != nil {
		return err
	}
	if !active {
		return s.Store.FinishPush(ctx, job, Outcome{State: "cancelled", Detail: "Subscription or session changed"})
	}
	if err := s.Config.Validate(&job.Subscription); err != nil {
		return s.Store.FinishPush(ctx, job, Outcome{State: "failed", Detail: "Subscription no longer passes endpoint validation"})
	}
	// Never include message body, recipient, OTP or credentials in a push payload.
	body, _ := json.Marshal(map[string]string{"device_id": job.DeviceID, "generation": job.Generation, "kind": job.Kind})
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	send, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := webpush.SendNotificationWithContext(send, body, &webpush.Subscription{Endpoint: job.Subscription.Endpoint, Keys: webpush.Keys{Auth: job.Subscription.Keys.Auth, P256dh: job.Subscription.Keys.P256dh}}, &webpush.Options{HTTPClient: client, Subscriber: s.Config.Subject, VAPIDPublicKey: s.Keys.Public, VAPIDPrivateKey: s.Keys.Private, TTL: 60, Topic: "textdock-inbox", Urgency: webpush.UrgencyNormal, RecordSize: 512})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	outcome := Outcome{State: "retrying", Detail: "Push service connection failed", RetryAfter: time.Duration(1<<min(job.Attempts, 6)) * time.Second}
	if response != nil {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
		outcome.HTTPStatus = response.StatusCode
		switch {
		case err == nil && response.StatusCode >= 200 && response.StatusCode < 300:
			outcome.State, outcome.Detail = "accepted", "Push service accepted the alert; device display is not confirmed"
		case response.StatusCode == 404 || response.StatusCode == 410:
			outcome.State, outcome.Detail = "expired", "Push subscription expired; enable notifications again"
		case response.StatusCode == 429 || response.StatusCode >= 500:
			outcome.Detail = "Push service temporarily rejected the alert"
			if seconds, parseErr := strconv.Atoi(response.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				outcome.RetryAfter = time.Duration(min(seconds, 60)) * time.Second
			}
		default:
			outcome.State, outcome.Detail = "failed", "Push service rejected the alert; check VAPID contact and subscription"
		}
	}
	if outcome.State == "retrying" && job.Attempts >= 3 {
		outcome.State = "failed"
	}
	return s.Store.FinishPush(ctx, job, outcome)
}
