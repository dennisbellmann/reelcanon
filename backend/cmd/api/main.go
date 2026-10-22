package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/accounts"
	"github.com/reelcanon/reelcanon/backend/internal/api"
	"github.com/reelcanon/reelcanon/backend/internal/catalog"
	"github.com/reelcanon/reelcanon/backend/internal/config"
	"github.com/reelcanon/reelcanon/backend/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := config.LoadDotEnv(); err != nil {
		logger.Warn("could not load .env", "error", err)
	}
	databaseURL := firstEnv("API_DATABASE_URL", "DATABASE_URL")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(ctx, databaseURL)
	cancel()
	if err != nil {
		logger.Error("start public API", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	addr := envOr("API_ADDR", ":8080")
	var handler http.Handler = api.NewHandler(catalog.NewRepository(db), db.PingContext, logger)
	if os.Getenv("AUTH_ENABLED") == "true" {
		userURL := os.Getenv("USER_DATABASE_URL")
		if userURL == "" {
			logger.Error("USER_DATABASE_URL is required when authentication is enabled")
			os.Exit(1)
		}
		userCtx, userCancel := context.WithTimeout(context.Background(), 10*time.Second)
		userDB, openErr := database.Open(userCtx, userURL)
		userCancel()
		if openErr != nil {
			logger.Error("account database is unavailable")
			os.Exit(1)
		}
		defer userDB.Close()
		accountService, accountErr := accounts.New(userDB, logger)
		if accountErr != nil {
			logger.Error("configure accounts", "error", accountErr)
			os.Exit(1)
		}
		handler = accountService.Handler(handler)
		cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
		defer cleanupCancel()
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				ctx, cancel := context.WithTimeout(cleanupCtx, 5*time.Second)
				_, sessionErr := userDB.ExecContext(ctx, "DELETE FROM sessions WHERE expiry < now()")
				_, requestErr := userDB.ExecContext(ctx, "DELETE FROM auth_login_requests WHERE expires_at < now()")
				_, mobileErr := userDB.ExecContext(ctx, "DELETE FROM mobile_sessions WHERE expires_at < now()")
				_, challengeErr := userDB.ExecContext(ctx, "DELETE FROM mobile_login_challenges WHERE expires_at < now()")
				cancel()
				if mobileErr != nil || challengeErr != nil || sessionErr != nil || requestErr != nil {
					logger.Warn("expired account data cleanup failed")
				}
				select {
				case <-cleanupCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	} else {
		handler = accounts.Disabled(handler)
	}
	if directory := os.Getenv("FRONTEND_DIST"); directory != "" {
		if err := api.ValidateFrontend(directory); err != nil {
			logger.Error("frontend production bundle is unavailable", "error", err)
			os.Exit(1)
		}
		handler = api.WithFrontend(handler, directory)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("public API listening", "address", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("public API stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopCancel()
	<-stop.Done()
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("shut down public API", "error", err)
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
