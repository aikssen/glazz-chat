package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("migration command failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: migrate <up|down|validate|reset>")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	runner, err := database.NewMigrationRunner(cfg.Database.URL)
	if err != nil {
		return err
	}
	defer runner.Close()

	switch args[0] {
	case "up":
		return runner.Up(ctx)
	case "down":
		return runner.Down(ctx)
	case "validate":
		return runner.Validate(ctx)
	case "reset":
		if cfg.Runtime.Environment == "production" {
			return fmt.Errorf("reset is disabled in production")
		}
		return runner.Reset(ctx)
	default:
		return fmt.Errorf("unknown migration command %q", args[0])
	}
}
