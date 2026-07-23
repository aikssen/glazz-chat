package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/outbox"
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
	workerID, err := ids.NewUUIDv7().New()
	if err != nil {
		return err
	}
	runner, err := outbox.New(
		pool,
		clock.UTC{},
		logger,
		map[string]outbox.Handler{},
		outbox.Options{
			WorkerID: workerID.String(), BatchSize: 20, MaxAttempts: 8,
			LockTTL: 5 * time.Minute, PollEvery: time.Second,
		},
	)
	if err != nil {
		return err
	}
	logger.Info("worker ready", "worker_id", workerID.String())
	return runner.Run(ctx)
}
