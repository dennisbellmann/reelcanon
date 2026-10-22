package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/adminui"
	"github.com/reelcanon/reelcanon/backend/internal/config"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/sources"
	"github.com/reelcanon/reelcanon/backend/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := config.LoadDotEnv(); err != nil {
		logger.Warn("could not load .env", "error", err)
	}
	username, password := strings.TrimSpace(os.Getenv("ADMIN_USER")), os.Getenv("ADMIN_PASSWORD")
	if username == "" || password == "" || password == "change-me-before-use" {
		logger.Error("ADMIN_USER and a non-default ADMIN_PASSWORD are required")
		os.Exit(1)
	}
	databaseURL := os.Getenv("ADMIN_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(ctx, databaseURL)
	cancel()
	if err != nil {
		logger.Error("start admin application", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	modules, err := sources.Load(envOr("CANON_CRAWLER_MODULES_FILE", "../data/crawler-modules.json"))
	if err != nil {
		logger.Error("load cockpit source modules", "error", err)
		os.Exit(1)
	}

	addr := envOr("ADMIN_ADDR", ":8081")
	server := &http.Server{Addr: addr, Handler: adminui.New(db, username, password, logger, modules...), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("admin application listening", "address", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("admin application stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopCancel()
	<-stop.Done()
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("shut down admin application", "error", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
