package storage

import (
	"context"
	"time"

	"github.com/fless-lab/TextDock/internal/relay"
)

func (s *SQLite) migrateGatewayHealth() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version == 6 {
		_, err := tx.Exec(`CREATE TABLE gateway_health(gateway_id TEXT PRIMARY KEY REFERENCES gateways(id) ON DELETE CASCADE, seen_at INTEGER NOT NULL, model TEXT NOT NULL, app_version TEXT NOT NULL, subscription_id INTEGER NOT NULL);
		INSERT INTO schema_migrations VALUES (7);`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) GatewayHeartbeat(ctx context.Context, id string, health relay.Health) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO gateway_health VALUES(?,?,?,?,?) ON CONFLICT(gateway_id) DO UPDATE SET seen_at=excluded.seen_at,model=excluded.model,app_version=excluded.app_version,subscription_id=excluded.subscription_id`, id, time.Now().UnixMilli(), health.Model, health.AppVersion, health.SubscriptionID)
	return err
}
