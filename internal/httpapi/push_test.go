package httpapi

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/fless-lab/TextDock/internal/push"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestOwnPushPreferencesDoNotGrantInboxWrites(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keys, err := db.PushKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer((&Server{Store: db, Workspaces: db, Devices: db, Simulation: db, Push: &push.Service{Store: db, Config: push.Config{Enabled: true}, Keys: keys}, Token: "admin-token-for-push", UI: fstest.MapFS{}}).Handler())
	defer s.Close()
	token, id := pairPhone(t, s, "admin-token-for-push")
	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]any{"endpoint": "https://fcm.googleapis.com/subscription", "expirationTime": nil, "keys": map[string]string{"auth": base64.RawURLEncoding.EncodeToString(make([]byte, 16)), "p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())}})
	r := request(t, s, "PUT", "/connect/v1/push/subscription", string(payload), token)
	if r.StatusCode != 200 {
		t.Fatalf("own subscription: %d", r.StatusCode)
	}
	if r := request(t, s, "POST", "/connect/v1/messages", `{}`, token); r.StatusCode != 403 {
		t.Fatal("notification permission granted message writes")
	}
	if r := request(t, s, "PUT", "/connect/v1/push/subscription", string(payload), "td_gateway_fake"); r.StatusCode != 401 {
		t.Fatal("gateway credential created a phone subscription")
	}
	if r := request(t, s, "POST", "/connect/v1/push/test", `{}`, token); r.StatusCode != 202 {
		t.Fatalf("diagnostic alert: %d", r.StatusCode)
	}
	request(t, s, "DELETE", "/api/v1/devices/"+id, "", "admin-token-for-push")
	if r := request(t, s, "GET", "/connect/v1/push/state", "", token); r.StatusCode != 401 {
		t.Fatal("revoked phone read notification state")
	}
}
