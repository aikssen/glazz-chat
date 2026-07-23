package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.Info("worker ready")
	<-ctx.Done()
	slog.Info("worker stopped")
}
