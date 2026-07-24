//go:build integration

package models

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
)

func TestSynchronizerIsIdempotentAndNeverAutoEnables(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 4, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	providerID := uuid.New()
	modelID := uuid.New()
	providerCode := "sync-" + providerID.String()[:8]
	upstreamID := "upstream-" + modelID.String()
	if _, err := pool.Raw().Exec(
		ctx,
		`INSERT INTO providers (id, code, display_name, adapter, enabled)
		 VALUES ($1, $2, 'Sync test', 'openai_compatible', true)`,
		providerID,
		providerCode,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO models (
			id, slug, name, context_window, max_output_tokens, capabilities,
			enabled, available, supported, audience
		) VALUES (
			$1, $2, 'Sync model', 8192, 1024, '{"chatCompletions": true}',
			false, false, true, ARRAY['user']
		)
	`, modelID, "model-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(
		ctx,
		`INSERT INTO provider_models (
			provider_id, model_id, provider_model_id, available
		 ) VALUES ($1, $2, $3, false)`,
		providerID,
		modelID,
		upstreamID,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM providers WHERE id = $1`, providerID)
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM models WHERE id = $1`, modelID)
	})

	synchronizer := NewSynchronizer(pool, ids.NewUUIDv7(), clock.UTC{})
	availableCatalog := provider.NewFake(provider.FakeOptions{Models: []provider.Model{
		{ID: upstreamID, ContextWindow: 8192, MaxOutputTokens: 1024, ChatCompletions: true},
		{ID: "unknown-upstream", ChatCompletions: true},
	}})
	result, err := synchronizer.Sync(ctx, providerCode, availableCatalog, "req-model-sync")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mapped != 1 || result.Ignored != 1 || result.Changed != 1 {
		t.Fatalf("first result = %#v", result)
	}
	assertModelState(t, pool, ctx, modelID, false, true)

	result, err = synchronizer.Sync(ctx, providerCode, availableCatalog, "req-model-sync-repeat")
	if err != nil || result.Changed != 0 {
		t.Fatalf("repeat result = %#v, err = %v", result, err)
	}

	missingCatalog := provider.NewFake(provider.FakeOptions{Models: []provider.Model{
		{ID: "different-upstream", ChatCompletions: true},
	}})
	result, err = synchronizer.Sync(ctx, providerCode, missingCatalog, "req-model-sync-missing")
	if err != nil || result.Changed != 1 {
		t.Fatalf("missing result = %#v, err = %v", result, err)
	}
	assertModelState(t, pool, ctx, modelID, false, false)

	result, err = synchronizer.Sync(ctx, providerCode, missingCatalog, "req-model-sync-missing-repeat")
	if err != nil || result.Changed != 0 {
		t.Fatalf("missing repeat result = %#v, err = %v", result, err)
	}
}

func assertModelState(
	t *testing.T,
	pool *database.Pool,
	ctx context.Context,
	modelID uuid.UUID,
	enabled bool,
	available bool,
) {
	t.Helper()
	var gotEnabled, gotAvailable bool
	if err := pool.Raw().QueryRow(
		ctx, `SELECT enabled, available FROM models WHERE id = $1`, modelID,
	).Scan(&gotEnabled, &gotAvailable); err != nil {
		t.Fatal(err)
	}
	if gotEnabled != enabled || gotAvailable != available {
		t.Fatalf(
			"model state = enabled %t, available %t; want enabled %t, available %t",
			gotEnabled, gotAvailable, enabled, available,
		)
	}
}
