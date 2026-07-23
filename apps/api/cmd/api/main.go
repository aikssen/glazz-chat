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

	"github.com/aikssen/glazz-chat/apps/api/internal/guests"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	identityoauth "github.com/aikssen/glazz-chat/apps/api/internal/identity/oauth"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/server"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/telemetry"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error_type", errorType(err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Database.MigrateOnStartup {
		runner, err := database.NewMigrationRunner(cfg.Database.URL)
		if err != nil {
			return err
		}
		if err := runner.Up(rootCtx); err != nil {
			_ = runner.Close()
			return err
		}
		if err := runner.Close(); err != nil {
			return err
		}
	}

	pool, err := database.Open(rootCtx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	redisClient, err := redisx.Open(rootCtx, cfg.Redis)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	telemetryRuntime, err := telemetry.New(rootCtx, cfg.Telemetry)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Runtime.ShutdownTimeout)
		defer cancel()
		_ = telemetryRuntime.Shutdown(shutdownCtx)
	}()

	timeSource := clock.UTC{}
	idSource := ids.NewUUIDv7()
	keyRing, err := tokens.Load(cfg.Auth, timeSource)
	if err != nil {
		return err
	}
	sessionService := sessions.New(
		pool, idSource, timeSource, keyRing, cfg.Auth.RefreshTokenTTL,
	)
	userService := users.New(
		pool, idSource, timeSource, cfg.Auth.TermsVersion,
		cfg.Auth.PrivacyVersion, cfg.Admin.BootstrapEmails,
	)
	guestService := guests.New(
		pool, idSource, timeSource, cfg.Cookies, 30*24*time.Hour,
	)
	browserManager := browser.New(
		cfg.Cookies, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL,
	)

	var oauthService *identityoauth.Service
	if cfg.OAuth.Enabled {
		google, err := identityoauth.NewGoogle(rootCtx, cfg.OAuth)
		if err != nil {
			return err
		}
		oauthService = identityoauth.New(
			redisClient, google, userService, sessionService, 10*time.Minute,
		)
	}

	handler := server.New(server.Dependencies{
		Config: cfg, Database: pool, Redis: redisClient, Guests: guestService,
		OAuth: oauthService, Sessions: sessionService, Browser: browserManager,
		Auth: browser.Authenticate(keyRing, sessionService), Telemetry: telemetryRuntime,
		Logger: logger, IDs: idSource,
	})
	httpServer := &http.Server{
		Addr: cfg.Runtime.APIAddress, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	serverError := make(chan error, 1)
	go func() {
		logger.Info(
			"api listening", "address", httpServer.Addr,
			"environment", cfg.Runtime.Environment,
		)
		serverError <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Runtime.ShutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

func errorType(err error) string {
	if err == nil {
		return "<nil>"
	}
	return "startup_or_runtime_error"
}
