package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigExplicitPathAndStrictShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "textdock.json")
	if err := os.WriteFile(path, []byte(`{"listen":"127.0.0.1:18259","retention":"24h","otp_pattern":"code=([A-Z0-9]+)"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXTDOCK_CONFIG", "/missing-config")
	cfg, err := Load([]string{"--config", path})
	if err != nil || cfg.Listen != "127.0.0.1:18259" || cfg.Retention != "24h" {
		t.Fatalf("explicit config: %+v %v", cfg, err)
	}
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{"--config=" + path}); err == nil {
		t.Fatal("unknown config key accepted")
	}
	if err := os.WriteFile(path, []byte(`{} {}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{"--config", path}); err == nil {
		t.Fatal("multiple JSON documents accepted")
	}
}
