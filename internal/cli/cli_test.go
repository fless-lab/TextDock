package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/fless-lab/TextDock/internal/httpapi"
	"github.com/fless-lab/TextDock/internal/storage"
)

func TestCLIWorkflowAndBackupRestore(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cli.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer((&httpapi.Server{Store: db, Workspaces: db, Devices: db, UI: fstest.MapFS{}, Token: "cli-test-token-123"}).Handler())
	defer server.Close()
	t.Setenv("TEXTDOCK_TOKEN", "cli-test-token-123")
	t.Setenv("TEXTDOCK_URL", server.URL)
	for _, args := range [][]string{
		{"send", "--to", "+33612345678", "--body", "Code 482193", "--run-id", "cli-run"},
		{"list", "--run-id", "cli-run"},
		{"wait", "--to", "+33612345678", "--run-id", "cli-run", "--timeout", "0"},
		{"export", "--format", "json", "--limit", "1"},
	} {
		var out bytes.Buffer
		if err := Run(ctx, args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("invalid JSON %v: %s", args, out.String())
		}
	}
	backup := filepath.Join(t.TempDir(), "snapshot.db")
	var out bytes.Buffer
	if err := Run(ctx, []string{"backup", "--db", dbPath, "--out", backup}, &out); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored.db")
	if err := Run(ctx, []string{"restore", "--source", backup, "--db", restored}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"restore", "--source", backup, "--db", restored}, &out); err == nil {
		t.Fatal("restore overwrote existing database")
	}
	if err := storage.ValidateSnapshot(ctx, restored); err != nil {
		t.Fatal(err)
	}
}
