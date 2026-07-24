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
	t.Cleanup(pool.Close)

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
		_, _ = pool.Raw().Exec(
			context.Background(),
			`DELETE FROM admin_audit_log
			 WHERE action = 'model.provider_availability_changed' AND target_id = $1`,
			modelID.String(),
		)
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM providers WHERE id = $1`, providerID)
		_, _ = pool.Raw().Exec(
			context.Background(),
			`DELETE FROM models WHERE id = $1 OR slug IN ('unknown-upstream', 'different-upstream')`,
			modelID,
		)
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
	if result.Mapped != 2 || result.Ignored != 0 || result.Changed != 1 {
		t.Fatalf("first result = %#v", result)
	}
	assertModelState(t, pool, ctx, modelID, false, true)
	assertDiscoveredModel(t, pool, ctx, "unknown-upstream")
	assertSyncAuditCount(t, pool, ctx, modelID, 1)

	result, err = synchronizer.Sync(ctx, providerCode, availableCatalog, "req-model-sync-repeat")
	if err != nil || result.Changed != 0 {
		t.Fatalf("repeat result = %#v, err = %v", result, err)
	}
	assertSyncAuditCount(t, pool, ctx, modelID, 1)

	missingCatalog := provider.NewFake(provider.FakeOptions{Models: []provider.Model{
		{ID: "different-upstream", ChatCompletions: true},
	}})
	result, err = synchronizer.Sync(ctx, providerCode, missingCatalog, "req-model-sync-missing")
	if err != nil || result.Changed != 2 {
		t.Fatalf("missing result = %#v, err = %v", result, err)
	}
	assertModelState(t, pool, ctx, modelID, false, false)
	assertSyncAuditCount(t, pool, ctx, modelID, 2)

	result, err = synchronizer.Sync(ctx, providerCode, missingCatalog, "req-model-sync-missing-repeat")
	if err != nil || result.Changed != 0 {
		t.Fatalf("missing repeat result = %#v, err = %v", result, err)
	}
	assertSyncAuditCount(t, pool, ctx, modelID, 2)
}

func assertDiscoveredModel(
	t *testing.T,
	pool *database.Pool,
	ctx context.Context,
	slug string,
) {
	t.Helper()
	var name string
	var enabled, available, supported bool
	if err := pool.Raw().QueryRow(ctx, `
		SELECT name, enabled, available, supported
		FROM models
		WHERE slug = $1
	`, slug).Scan(&name, &enabled, &available, &supported); err != nil {
		t.Fatal(err)
	}
	if name == "" || enabled || !available || !supported {
		t.Fatalf(
			"discovered model = name %q, enabled %t, available %t, supported %t",
			name, enabled, available, supported,
		)
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

func assertSyncAuditCount(
	t *testing.T,
	pool *database.Pool,
	ctx context.Context,
	modelID uuid.UUID,
	want int,
) {
	t.Helper()
	var count int
	if err := pool.Raw().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM admin_audit_log
		WHERE action = 'model.provider_availability_changed'
		  AND target_type = 'model'
		  AND target_id = $1
	`, modelID.String()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("sync audit count = %d, want %d", count, want)
	}
}
