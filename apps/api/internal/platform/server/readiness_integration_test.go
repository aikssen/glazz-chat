//go:build integration

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
)

func TestReadinessReportsUnavailableDependencies(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("postgres", func(t *testing.T) {
		pool, redisClient := openReadinessDependencies(t, runtimeConfig)
		pool.Close()
		defer redisClient.Close()

		assertReadinessDependency(t, Dependencies{
			Database: pool,
			Redis:    redisClient,
		}, "postgres")
	})

	t.Run("redis", func(t *testing.T) {
		pool, redisClient := openReadinessDependencies(t, runtimeConfig)
		defer pool.Close()
		if err := redisClient.Close(); err != nil {
			t.Fatal(err)
		}

		assertReadinessDependency(t, Dependencies{
			Database: pool,
			Redis:    redisClient,
		}, "redis")
	})
}

func openReadinessDependencies(
	t *testing.T,
	runtimeConfig config.Config,
) (*database.Pool, *redisx.Client) {
	t.Helper()
	ctx := context.Background()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 2, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	redisClient, err := redisx.Open(ctx, config.Redis{
		URL:           runtimeConfig.Redis.URL,
		Prefix:        "glazz-readiness-integration-" + uuid.NewString(),
		HealthTimeout: time.Second,
	})
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool, redisClient
}

func assertReadinessDependency(t *testing.T, deps Dependencies, unavailable string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)
	response := httptest.NewRecorder()
	deps.readiness(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	var body struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "not_ready" || body.Dependencies[unavailable] != "down" {
		t.Fatalf("readiness body = %#v", body)
	}
}
