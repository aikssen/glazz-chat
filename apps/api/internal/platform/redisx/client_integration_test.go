//go:build integration

package redisx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	client, err := Open(context.Background(), config.Redis{
		URL:           runtimeConfig.Redis.URL,
		Prefix:        fmt.Sprintf("glazz-test-%d", time.Now().UnixNano()),
		HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestTakeIsSingleUseAndExpires(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	if err := client.Put(ctx, "ticket", "one", "actor", 100*time.Millisecond); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if value, err := client.Take(ctx, "ticket", "one"); err != nil || value != "actor" {
		t.Fatalf("Take() = %q, %v", value, err)
	}
	if _, err := client.Take(ctx, "ticket", "one"); err != ErrNotFound {
		t.Fatalf("second Take() error = %v", err)
	}
	if err := client.Put(ctx, "ticket", "expiring", "actor", 20*time.Millisecond); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := client.Take(ctx, "ticket", "expiring"); err != ErrNotFound {
		t.Fatalf("expired Take() error = %v", err)
	}
}

func TestLeaseContentionAndOwnerRelease(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	acquired, err := client.AcquireLease(ctx, "generation", "one", "owner-a", time.Second)
	if err != nil || !acquired {
		t.Fatalf("first AcquireLease() = %v, %v", acquired, err)
	}
	acquired, err = client.AcquireLease(ctx, "generation", "one", "owner-b", time.Second)
	if err != nil || acquired {
		t.Fatalf("second AcquireLease() = %v, %v", acquired, err)
	}
	released, err := client.ReleaseLease(ctx, "generation", "one", "owner-b")
	if err != nil || released {
		t.Fatalf("wrong owner ReleaseLease() = %v, %v", released, err)
	}
	released, err = client.ReleaseLease(ctx, "generation", "one", "owner-a")
	if err != nil || !released {
		t.Fatalf("owner ReleaseLease() = %v, %v", released, err)
	}
}

func TestRateLimitBoundary(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	for index := 1; index <= 3; index++ {
		result, err := client.AddRateUsage(ctx, "guest:one", 1, 2, time.Minute)
		if err != nil {
			t.Fatalf("AddRateUsage() error = %v", err)
		}
		if result.Allowed != (index <= 2) {
			t.Fatalf("request %d Allowed = %v", index, result.Allowed)
		}
	}
}
