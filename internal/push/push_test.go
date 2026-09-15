package push_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/connect"
	"github.com/fless-lab/TextDock/internal/message"
	"github.com/fless-lab/TextDock/internal/push"
	"github.com/fless-lab/TextDock/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/hkdf"
)

func subscription(t *testing.T, endpoint string) (push.Subscription, *ecdh.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	var sub push.Subscription
	sub.Endpoint = endpoint
	sub.Keys.P256dh = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	sub.Keys.Auth = base64.RawURLEncoding.EncodeToString(auth)
	return sub, key, auth
}
func device(t *testing.T, db *storage.SQLite, id, inbox, to, run string) connect.Device {
	t.Helper()
	ctx := context.Background()
	if err := db.CreatePair(ctx, id, connect.Scope{Inbox: inbox, To: to, RunID: run}, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	d, err := db.ClaimPair(ctx, id, connect.Hash(id), connect.Device{ID: id, Name: id, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSubscriptionValidation(t *testing.T) {
	sub, _, _ := subscription(t, "https://fcm.googleapis.com/push/test")
	config := push.Config{}
	if err := config.Validate(&sub); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"http://fcm.googleapis.com/test", "https://fcm.googleapis.com.evil.test/x", "https://127.0.0.1/x", "https://user:password@fcm.googleapis.com/x", "https://fcm.googleapis.com:1234/x"} {
		copy := sub
		copy.Endpoint = endpoint
		if config.Validate(&copy) == nil {
			t.Fatalf("untrusted push endpoint accepted: %s", endpoint)
		}
	}
	sub.Keys.Auth = "bad"
	if !errors.Is(config.Validate(&sub), push.ErrSubscription) {
		t.Fatal("invalid encryption key accepted")
	}
}

func TestEncryptedGenericPayloadAndExpiredSubscription(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetPushEnabled(true)
	d := device(t, db, "phone", "local", "+12025550123", "push-run")
	keys, err := db.PushKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sub, recipient, auth := subscription(t, "https://localhost/placeholder")
	var expired atomic.Bool
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expired.Load() {
			w.WriteHeader(410)
			return
		}
		if r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("TTL") != "60" {
			t.Error("incorrect Web Push headers")
		}
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "vapid t="), ", k=")
		if len(parts) != 2 {
			t.Error("missing VAPID authorization")
			w.WriteHeader(400)
			return
		}
		public, _ := base64.RawURLEncoding.DecodeString(keys.Public)
		x, y := elliptic.Unmarshal(elliptic.P256(), public)
		_, err := jwt.Parse(parts[0], func(*jwt.Token) (any, error) { return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil }, jwt.WithValidMethods([]string{"ES256"}), jwt.WithAudience("https://"+r.Host), jwt.WithSubject("https://textdock.test"))
		if err != nil {
			t.Errorf("invalid VAPID JWT: %v", err)
		}
		wire, _ := io.ReadAll(r.Body)
		if len(wire) < 102 {
			t.Error("missing encrypted content")
			return
		}
		salt := wire[:16]
		length := int(wire[20])
		sender, err := ecdh.P256().NewPublicKey(wire[21 : 21+length])
		if err != nil {
			t.Error(err)
			return
		}
		shared, err := recipient.ECDH(sender)
		if err != nil {
			t.Error(err)
			return
		}
		info := append([]byte("WebPush: info\x00"), recipient.PublicKey().Bytes()...)
		info = append(info, sender.Bytes()...)
		derive := func(secret, salt, info []byte, size int) []byte {
			result := make([]byte, size)
			if _, err := io.ReadFull(hkdf.New(sha256.New, secret, salt, info), result); err != nil {
				t.Error(err)
			}
			return result
		}
		ikm := derive(shared, auth, info, 32)
		cek := derive(ikm, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)
		nonce := derive(ikm, salt, []byte("Content-Encoding: nonce\x00"), 12)
		block, _ := aes.NewCipher(cek)
		gcm, _ := cipher.NewGCM(block)
		plain, err := gcm.Open(nil, nonce, wire[21+length:], nil)
		if err != nil {
			t.Error(err)
			return
		}
		plain = bytes.TrimRight(plain, "\x00")
		if plain[len(plain)-1] != 2 {
			t.Error("missing padding delimiter")
			return
		}
		plain = plain[:len(plain)-1]
		var payload map[string]string
		if err := json.Unmarshal(plain, &payload); err != nil {
			t.Error(err)
		}
		if len(payload) != 3 || payload["device_id"] != d.ID || payload["kind"] != "inbox" {
			t.Errorf("unexpected push content: %s", plain)
		}
		if bytes.Contains(plain, []byte("918273")) || bytes.Contains(plain, []byte(d.Scope.To)) {
			t.Error("SMS content leaked into lock-screen payload")
		}
		w.WriteHeader(201)
	}))
	defer receiver.Close()
	sub.Endpoint = receiver.URL + "/subscription"
	if _, err := db.PutPushSubscription(ctx, d.ID, sub); err != nil {
		t.Fatal(err)
	}
	m, _ := message.New(message.Input{To: d.Scope.To, From: "Acme", Body: "Private OTP 918273", RunID: "push-run"}, "api")
	if err := db.Save(ctx, m); err != nil {
		t.Fatal(err)
	}
	client := receiver.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	service := push.Service{Config: push.Config{Enabled: true, Subject: "https://textdock.test", AllowedHosts: []string{"127.0.0.1"}}, Keys: keys, Store: db, Client: client}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := service.Step(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, sql.ErrNoRows) || time.Now().After(deadline) {
			t.Fatalf("push not processed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	state, _ := db.PushState(ctx, d.ID)
	if state.Status != "accepted" || !state.Subscribed {
		t.Fatalf("acceptance: %+v", state)
	}
	expired.Store(true)
	if err := db.QueuePushTest(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Step(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = db.PushState(ctx, d.ID)
	if state.Subscribed || state.Status != "expired" {
		t.Fatalf("expired endpoint retained: %+v", state)
	}
}
