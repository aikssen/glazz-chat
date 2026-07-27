package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/logging"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/outbox"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "worker")
	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("worker configuration failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	logger, err := logging.New(os.Stdout, cfg.Runtime.LogLevel, "worker")
	if err != nil {
		bootstrap.Error("worker logger configuration failed", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
	slog.SetDefault(logger)
	if err := run(logger, cfg); err != nil {
		logger.Error("worker stopped", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Debug("worker initialization started", "environment", cfg.Runtime.Environment)
	pool, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Debug("worker database connection ready")
	idSource := ids.NewUUIDv7()
	timeSource := clock.UTC{}
	providerCode, err := models.ConfigureProvider(ctx, pool, cfg.Provider.Kind, timeSource)
	if err != nil {
		return err
	}
	workerID, err := idSource.New()
	if err != nil {
		return err
	}
	gateways := map[string]provider.Gateway{
		models.FakeProviderCode: provider.NewFake(provider.FakeOptions{
			Usage: provider.Usage{
				InputTokens: 8, OutputTokens: int(cfg.Provider.FakeOutputTokens),
			},
		}),
	}
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
	synchronizer := models.NewSynchronizer(pool, idSource, timeSource)
	if providerCode != models.FakeProviderCode {
		logger.Info("startup model sync started", "provider_code", providerCode)
		if _, err := synchronizer.Sync(
			ctx, providerCode, gateways[providerCode], "startup-model-sync",
		); err != nil {
			logger.ErrorContext(
				ctx, "startup model sync failed", "error_type", fmt.Sprintf("%T", err),
			)
		} else {
			logger.Info("startup model sync completed", "provider_code", providerCode)
		}
	}
	privacyService := privacy.New(pool, idSource, timeSource)
	runner, err := outbox.New(
		pool,
		timeSource,
		logger,
		map[string]outbox.Handler{
			"models.sync": outbox.HandlerFunc(func(ctx context.Context, event outbox.Event) error {
				eventLogger := logging.Context(logger, ctx).With(
					"event_id", event.ID, "event_type", event.Type,
				)
				eventLogger.Debug("model sync event decoded")
				var payload struct {
					ProviderCode string `json:"providerCode"`
				}
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return fmt.Errorf("decode model sync event: %w", err)
				}
				gateway := gateways[payload.ProviderCode]
				if gateway == nil {
					return errors.New("model sync gateway is not configured")
				}
				eventLogger.Info("model sync started", "provider_code", payload.ProviderCode)
				_, err := synchronizer.Sync(ctx, payload.ProviderCode, gateway, event.ID)
				if err == nil {
					eventLogger.Info("model sync completed", "provider_code", payload.ProviderCode)
				}
				return err
			}),
		},
		outbox.Options{
			WorkerID: workerID.String(), BatchSize: 20, MaxAttempts: 8,
			LockTTL: 5 * time.Minute, PollEvery: time.Second,
		},
	)
	if err != nil {
		return err
	}
	logger.Info("worker ready", "worker_id", workerID.String())
	runnerError := make(chan error, 1)
	go func() { runnerError <- runner.Run(ctx) }()
	maintenance := time.NewTicker(time.Hour)
	defer maintenance.Stop()
	runMaintenance := func() {
		logger.DebugContext(ctx, "maintenance cycle started")
		if purged, err := privacyService.PurgeDue(ctx, 0, 20); err != nil {
			logger.ErrorContext(ctx, "account purge cycle failed", "error_type", fmt.Sprintf("%T", err))
		} else {
			logger.InfoContext(ctx, "account purge cycle completed", "accounts_purged", purged)
		}
		if deleted, err := privacyService.CleanupGuests(ctx); err != nil {
			logger.ErrorContext(ctx, "guest cleanup cycle failed", "error_type", fmt.Sprintf("%T", err))
		} else {
			logger.InfoContext(ctx, "guest cleanup cycle completed", "guests_deleted", deleted)
		}
		logger.DebugContext(ctx, "maintenance cycle completed")
	}
	runMaintenance()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutdown started")
			return <-runnerError
		case err := <-runnerError:
			return err
		case <-maintenance.C:
			runMaintenance()
		}
	}
}
