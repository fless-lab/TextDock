package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fless-lab/TextDock/internal/httpapi"
	"github.com/fless-lab/TextDock/internal/storage"
	"github.com/fless-lab/TextDock/internal/ui"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("TextDock stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("listen", env("TEXTDOCK_LISTEN", "127.0.0.1:18257"), "HTTP listen address")
	dbPath := flag.String("db", env("TEXTDOCK_DB", "data/textdock.db"), "SQLite path or :memory:")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("TextDock " + version)
		return nil
	}
	token := os.Getenv("TEXTDOCK_TOKEN")
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if (ip == nil || !ip.IsLoopback()) && len(token) < 16 {
		return errors.New("network access requires TEXTDOCK_TOKEN with at least 16 characters; use 127.0.0.1:18257 for zero-config local use")
	}
	if *dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(*dbPath), 0700); err != nil {
			return err
		}
	}
	store, err := storage.Open(*dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	server := &http.Server{
		Addr: *addr, Handler: (&httpapi.Server{Store: store, Token: token, Version: version, UI: ui.Files()}).Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 40 * time.Second, IdleTimeout: 60 * time.Second,
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	slog.Info("TextDock ready", "url", "http://"+listener.Addr().String(), "version", version, "mode", "capture")
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
