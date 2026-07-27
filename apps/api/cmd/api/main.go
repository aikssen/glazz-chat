package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/admin"
	"github.com/aikssen/glazz-chat/apps/api/internal/chat"
	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/guests"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/browser"
	identityoauth "github.com/aikssen/glazz-chat/apps/api/internal/identity/oauth"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/logging"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/server"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/telemetry"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
	"github.com/aikssen/glazz-chat/apps/api/internal/quota"
	"github.com/aikssen/glazz-chat/apps/api/internal/realtime"
	"github.com/aikssen/glazz-chat/apps/api/internal/settings"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "api")
	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("api configuration failed", "error_type", errorType(err))
		os.Exit(1)
	}
	logger, err := logging.New(os.Stdout, cfg.Runtime.LogLevel, "api")
	if err != nil {
		bootstrap.Error("api logger configuration failed", "error_type", errorType(err))
		os.Exit(1)
	}
	slog.SetDefault(logger)
	if err := run(logger, cfg); err != nil {
		logger.Error("api stopped", "error_type", errorType(err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg config.Config) error {
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Debug("api initialization started", "environment", cfg.Runtime.Environment)

	if cfg.Database.MigrateOnStartup {
		logger.Info("database migration started")
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
		logger.Info("database migration completed")
	}

	pool, err := database.Open(rootCtx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Debug("database connection ready")
	redisClient, err := redisx.Open(rootCtx, cfg.Redis)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	logger.Debug("redis connection ready")
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
	providerCode, err := models.ConfigureProvider(rootCtx, pool, cfg.Provider.Kind, timeSource)
	if err != nil {
		return err
	}
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
	runtimeSettings := settings.New(pool, redisClient)
	guestService := guests.New(
		pool, idSource, timeSource, cfg.Cookies, cfg.Guests.SessionTTL,
	).WithPolicySource(func(ctx context.Context) (guests.Policy, error) {
		snapshot, err := runtimeSettings.Load(ctx)
		return guests.Policy{
			MessageLimit:     int32(snapshot.GuestMessageLimit),
			OutputTokenLimit: int32(snapshot.GuestOutputTokenLimit),
		}, err
	})
	modelService := models.New(pool)
	adminService := admin.New(pool, idSource, timeSource).WithSettingsInvalidator(runtimeSettings)
	privacyService := privacy.New(pool, idSource, timeSource)
	conversationService := conversations.New(pool, modelService, idSource, timeSource)
	tickets := realtime.NewTickets(redisClient, timeSource, 30*time.Second)
	broker := realtime.NewBroker(redisClient, idSource, timeSource)
	quotaService := quota.New(
		pool, redisClient, idSource, timeSource, quota.DefaultPolicy(), cfg.Cookies.SigningKey,
	).WithPolicySource(func(ctx context.Context) (quota.Policy, error) {
		snapshot, err := runtimeSettings.Load(ctx)
		if err != nil {
			return quota.Policy{}, err
		}
		policy := quota.DefaultPolicy()
		policy.GuestMessageLimit = snapshot.GuestMessageLimit
		policy.GuestOutputTokenLimit = snapshot.GuestOutputTokenLimit
		policy.UserDailyMessageLimit = snapshot.UserMessageLimit
		policy.UserDailyOutputLimit = snapshot.UserOutputTokenLimit
		policy.GlobalDailyOutputCap = snapshot.GlobalOutputTokenLimit
		policy.GlobalConcurrentLimit = snapshot.GlobalConcurrentStreams
		return policy, nil
	})
	fakeOptions := provider.FakeOptions{
		Usage: provider.Usage{InputTokens: 8, OutputTokens: int(cfg.Provider.FakeOutputTokens)},
	}
	if cfg.Runtime.Environment == "test" {
		fakeOptions.Latency = 100 * time.Millisecond
	}
	gateways := chat.Gateways{models.FakeProviderCode: provider.NewFake(fakeOptions)}
	if cfg.Provider.Kind != "fake" {
		gateway, err := provider.NewOpenAICompatible(
			cfg.Provider.BaseURL, cfg.Provider.APIKey, nil, provider.DefaultOptions(),
		)
		if err != nil {
			return err
		}
		gateways[providerCode] = provider.NewResilient(
			gateway, provider.DefaultResilienceOptions(),
		).WithLogger(logger)
	}
	chatService := chat.New(
		rootCtx, pool, conversationService, modelService, quotaService, broker,
		gateways, idSource, timeSource,
	).WithLogger(logger).WithSystemPrompt(func(ctx context.Context) (string, error) {
		snapshot, err := runtimeSettings.Load(ctx)
		return snapshot.SystemPrompt, err
	}).WithSummaryModel(func(ctx context.Context) (models.Selection, error) {
		snapshot, err := runtimeSettings.Load(ctx)
		if err != nil {
			return models.Selection{}, err
		}
		modelID, err := uuid.Parse(snapshot.SummaryModelID)
		if err != nil {
			return models.Selection{}, fmt.Errorf("parse summary model ID: %w", err)
		}
		return modelService.Select(ctx, modelID, "user")
	}).WithSafety(
		chat.NewRuleSafetyPolicy(),
		func(ctx context.Context) ([]string, []string, error) {
			snapshot, err := runtimeSettings.Load(ctx)
			return snapshot.InputSafetyCategories, snapshot.OutputSafetyCategories, err
		},
		chat.SafetyReporterFunc(func(_ context.Context, report chat.SafetyReport) error {
			logger.Warn("chat content blocked",
				"stage", report.Stage,
				"category", report.Category,
				"request_id", report.RequestID,
				"correlation_id", report.RequestID,
			)
			return nil
		}),
	).WithAvailability(func(ctx context.Context) (bool, error) {
		snapshot, err := runtimeSettings.Load(ctx)
		return !cfg.Runtime.Maintenance && !snapshot.Maintenance, err
	})
	realtimeHandler := realtime.NewHandler(
		tickets, broker, chatService, timeSource, cfg.Runtime.AllowedOrigins,
	).WithLogger(logger)
	browserManager := browser.New(
		cfg.Cookies, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL,
	)

	var oauthService *identityoauth.Service
	if cfg.OAuth.Enabled {
		var oauthProvider identityoauth.Provider
		if cfg.OAuth.TestMode {
			oauthProvider, err = identityoauth.NewDeterministic(cfg.OAuth)
		} else {
			oauthProvider, err = identityoauth.NewGoogle(rootCtx, cfg.OAuth)
		}
		if err != nil {
			return err
		}
		oauthService = identityoauth.New(
			redisClient, oauthProvider, userService, sessionService, 10*time.Minute,
		)
	}

	handler := server.New(server.Dependencies{
		Config: cfg, Database: pool, Redis: redisClient, Guests: guestService,
		OAuth: oauthService, Sessions: sessionService, Browser: browserManager,
		Auth: browser.Authenticate(keyRing, sessionService), Telemetry: telemetryRuntime,
		Logger: logger, IDs: idSource, Clock: timeSource,
		Models: modelService, Chats: conversationService,
		Tickets: tickets, Realtime: realtimeHandler,
		ChatEngine:   chatService,
		Admin:        adminService,
		Privacy:      privacyService,
		Settings:     runtimeSettings,
		ProviderCode: providerCode,
		ResolveUser: func(request *http.Request) (browser.Actor, error) {
			return browser.Resolve(request, keyRing, sessionService)
		},
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
		logger.Info("api shutdown started")
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
