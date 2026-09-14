package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fless-lab/TextDock/internal/cli"
	"github.com/fless-lab/TextDock/internal/config"
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
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		return cli.Run(context.Background(), os.Args[1:], os.Stdout)
	}
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:18257"
	}
	if cfg.DB == "" {
		cfg.DB = "data/textdock.db"
	}
	if cfg.Retention == "" {
		cfg.Retention = "0"
	}
	flag.String("config", "", "JSON configuration file")
	addr := flag.String("listen", env("TEXTDOCK_LISTEN", cfg.Listen), "HTTP listen address")
	dbPath := flag.String("db", env("TEXTDOCK_DB", cfg.DB), "SQLite path or :memory:")
	publicURL := flag.String("public-url", env("TEXTDOCK_PUBLIC_URL", cfg.PublicURL), "phone-facing HTTP(S) origin for QR pairing")
	retention := flag.Duration("retention", 0, "automatically delete messages older than this duration; 0 disables")
	otpPattern := flag.String("otp-pattern", env("TEXTDOCK_OTP_PATTERN", cfg.OTPPattern), "custom RE2 OTP pattern; first capture group is the code")
	if value := env("TEXTDOCK_RETENTION", cfg.Retention); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("retention: %w", err)
		}
		*retention = parsed
	}
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	var otpRegex *regexp.Regexp
	if *otpPattern != "" {
		otpRegex, err = regexp.Compile(*otpPattern)
		if err != nil {
			return fmt.Errorf("otp-pattern: %w", err)
		}
		if otpRegex.NumSubexp() < 1 {
			return errors.New("otp-pattern must contain a capture group")
		}
	}
	if *retention < 0 {
		return errors.New("retention must not be negative")
	}
	if *showVersion {
		fmt.Println("TextDock " + version)
		return nil
	}
	token := os.Getenv("TEXTDOCK_TOKEN")
	if *publicURL != "" {
		u, err := url.Parse(*publicURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("public-url must be an HTTP(S) origin without credentials, query or fragment")
		}
		if len(token) < 16 {
			return errors.New("public-url requires TEXTDOCK_TOKEN with at least 16 characters")
		}
	}
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
	api := &httpapi.Server{Store: store, Devices: store, Workspaces: store, Token: token, Version: version, UI: ui.Files(), Listen: *addr, PublicURL: *publicURL, OTPPattern: otpRegex}
	server := &http.Server{
		Addr: *addr, Handler: api.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 40 * time.Second, IdleTimeout: 60 * time.Second,
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *retention > 0 {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				n, err := store.Prune(ctx, time.Now().Add(-*retention))
				if err != nil && ctx.Err() == nil {
					slog.Error("retention cleanup", "error", err)
				}
				if n > 0 {
					api.Hub.Resync()
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
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
