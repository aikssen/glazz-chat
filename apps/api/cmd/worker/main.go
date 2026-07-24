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
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/outbox"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error_type", fmt.Sprintf("%T", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()
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
		models.FakeProviderCode: provider.NewFake(provider.FakeOptions{}),
	}
	if cfg.Provider.Kind != "fake" {
		gateway, err := provider.NewOpenAICompatible(
			cfg.Provider.BaseURL, cfg.Provider.APIKey, nil, provider.DefaultOptions(),
		)
		if err != nil {
			return err
		}
		gateways[providerCode] = provider.NewResilient(gateway, provider.DefaultResilienceOptions())
	}
	synchronizer := models.NewSynchronizer(pool, idSource, timeSource)
	if providerCode != models.FakeProviderCode {
		if _, err := synchronizer.Sync(
			ctx, providerCode, gateways[providerCode], "startup-model-sync",
		); err != nil {
			logger.ErrorContext(
				ctx, "startup model sync failed", "error_type", fmt.Sprintf("%T", err),
			)
		}
	}
	privacyService := privacy.New(pool, idSource, timeSource)
	runner, err := outbox.New(
		pool,
		timeSource,
		logger,
		map[string]outbox.Handler{
			"models.sync": outbox.HandlerFunc(func(ctx context.Context, event outbox.Event) error {
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
				_, err := synchronizer.Sync(ctx, payload.ProviderCode, gateway, event.ID)
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
		if _, err := privacyService.PurgeDue(ctx, 0, 20); err != nil {
			logger.ErrorContext(ctx, "account purge cycle failed", "error_type", fmt.Sprintf("%T", err))
		}
		if _, err := privacyService.CleanupGuests(ctx); err != nil {
			logger.ErrorContext(ctx, "guest cleanup cycle failed", "error_type", fmt.Sprintf("%T", err))
		}
	}
	runMaintenance()
	for {
		select {
		case <-ctx.Done():
			return <-runnerError
		case err := <-runnerError:
			return err
		case <-maintenance.C:
			runMaintenance()
		}
	}
}
