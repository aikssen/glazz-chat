package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type correlationKey struct{}

func New(output io.Writer, level, service string) (*slog.Logger, error) {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(level)))); err != nil {
		return nil, fmt.Errorf("parse LOG_LEVEL: must be debug, info, warn, or error")
	}
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: parsed})).
		With("service", service), nil
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationKey{}, correlationID)
}

func CorrelationID(ctx context.Context) string {
	value, _ := ctx.Value(correlationKey{}).(string)
	return value
}

func Context(logger *slog.Logger, ctx context.Context) *slog.Logger {
	if correlationID := CorrelationID(ctx); correlationID != "" {
		return logger.With("correlation_id", correlationID)
	}
	return logger
}
