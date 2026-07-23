package redisx

import (
	"context"
	"testing"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func TestOpenUnavailableRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := Open(ctx, config.Redis{
		URL:           "redis://127.0.0.1:1/0",
		Prefix:        "test",
		HealthTimeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Open() error = nil")
	}
}
