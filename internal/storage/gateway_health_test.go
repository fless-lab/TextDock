package storage

import (
	"context"
	"testing"
	"time"

	"github.com/fless-lab/TextDock/internal/relay"
)

func TestGatewayHealthFreshnessAndRevocation(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := relay.Gateway{ID: "test-gateway", Name: "Phone", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateGateway(ctx, g, "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.GatewayHeartbeat(ctx, g.ID, relay.Health{Model: "Emulated phone", AppVersion: "preview", SubscriptionID: -1}); err != nil {
		t.Fatal(err)
	}
	items, err := s.Gateways(ctx)
	if err != nil || len(items) != 1 || !items[0].Online {
		t.Fatalf("fresh health: %+v %v", items, err)
	}
	if _, err := s.db.Exec(`UPDATE gateway_health SET seen_at=?`, time.Now().Add(-time.Minute).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	items, _ = s.Gateways(ctx)
	if items[0].Online || items[0].LastSeen == nil {
		t.Fatal("stale heartbeat still online")
	}
	s.GatewayHeartbeat(ctx, g.ID, relay.Health{})
	s.RevokeGateway(ctx, g.ID)
	items, _ = s.Gateways(ctx)
	if items[0].Online {
		t.Fatal("revoked gateway still online")
	}
}
