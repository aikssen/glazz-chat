//go:build integration

package models

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
)

func TestCatalogEnforcesExposureSelectionAndProviderHealth(t *testing.T) {
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
	orphanID := uuid.New()
	disabledID := uuid.New()
	unsupportedID := uuid.New()
	unavailableID := uuid.New()
	defaultModelID := uuid.MustParse("00000000-0000-7000-8000-000000000101")
	code := "catalog-" + providerID.String()[:8]
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM providers WHERE id = $1`, providerID)
		_, _ = pool.Raw().Exec(
			context.Background(), `DELETE FROM models WHERE id IN ($1, $2, $3, $4, $5)`,
			modelID, orphanID, disabledID, unsupportedID, unavailableID,
		)
	})
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO providers (id, code, display_name, adapter, enabled, health_status)
		VALUES ($1, $2, 'Catalog test', 'openai_compatible', true, 'healthy')
	`, providerID, code); err != nil {
		t.Fatal(err)
	}
	insertModel := `
		INSERT INTO models (
			id, slug, name, context_window, max_output_tokens, capabilities,
			enabled, available, supported, audience, default_for
		) VALUES ($1, $2, $3, 8192, 1024, '{"chatCompletions": true}',
		          $4, $5, $6, $7, $8)`
	if _, err := pool.Raw().Exec(
		ctx, insertModel, modelID, "catalog-"+modelID.String(), "Catalog visible",
		true, true, true, []string{"guest", "user"}, []string{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO provider_models (provider_id, model_id, provider_model_id, available)
		VALUES ($1, $2, $3, true)
	`, providerID, modelID, "upstream-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(
		ctx, insertModel, orphanID, "catalog-"+orphanID.String(), "Catalog orphan",
		true, true, true, []string{"user"}, []string{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(
		ctx, insertModel, disabledID, "catalog-"+disabledID.String(), "Catalog disabled",
		false, true, true, []string{"user"}, []string{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(
		ctx, insertModel, unsupportedID, "catalog-"+unsupportedID.String(), "Catalog unsupported",
		false, true, false, []string{"user"}, []string{},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(
		ctx, insertModel, unavailableID, "catalog-"+unavailableID.String(), "Catalog unavailable",
		true, false, true, []string{"user"}, []string{},
	); err != nil {
		t.Fatalf("enabled models must be able to become unavailable: %v", err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO provider_models (provider_id, model_id, provider_model_id, available)
		VALUES ($1, $2, $3, false)
	`, providerID, unavailableID, "upstream-"+unavailableID.String()); err != nil {
		t.Fatal(err)
	}

	service := New(pool)
	guestModels, err := service.List(ctx, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if len(guestModels) != 1 || !containsModel(guestModels, defaultModelID) {
		t.Fatalf("guest catalog must expose only the default model: %#v", guestModels)
	}
	if _, err := service.Select(ctx, modelID, "guest"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("guest selected a non-default model: %v", err)
	}
	userModels, err := service.List(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	if !containsModel(userModels, modelID) || containsModel(userModels, orphanID) ||
		containsModel(userModels, disabledID) || containsModel(userModels, unsupportedID) ||
		containsModel(userModels, unavailableID) {
		t.Fatalf("user catalog did not enforce mapping/support: %#v", userModels)
	}
	selection, err := service.Select(ctx, modelID, "user")
	if err != nil {
		t.Fatal(err)
	}
	publicJSON, err := json.Marshal(selection.Model)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "provider") || strings.Contains(string(publicJSON), "upstream") {
		t.Fatalf("public model leaked provider details: %s", publicJSON)
	}
	if _, err := service.Select(ctx, orphanID, "user"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("orphan model was selectable: %v", err)
	}
	if _, err := service.Select(ctx, unavailableID, "user"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("unavailable model was selectable: %v", err)
	}
	if _, err := service.Select(ctx, disabledID, "user"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("disabled model was selectable: %v", err)
	}

	if _, err := pool.Raw().Exec(
		ctx, `UPDATE providers SET health_status = 'unavailable' WHERE id = $1`, providerID,
	); err != nil {
		t.Fatal(err)
	}
	userModels, err = service.List(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	if containsModel(userModels, modelID) {
		t.Fatal("catalog exposed a model whose provider is unavailable")
	}
	if _, err := service.Select(ctx, modelID, "user"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("unavailable provider model was selectable: %v", err)
	}
	if _, err := pool.Raw().Exec(
		ctx, `UPDATE providers SET health_status = 'healthy', enabled = false WHERE id = $1`, providerID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Select(ctx, modelID, "user"); !errors.Is(err, ErrNotSelectable) {
		t.Fatalf("disabled provider model was selectable: %v", err)
	}
}

func TestModelSchemaRejectsInvalidExposure(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 2, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	insert := `
		INSERT INTO models (
			id, slug, name, context_window, max_output_tokens, capabilities,
			enabled, available, supported, audience, default_for
		) VALUES ($1, $2, 'Invalid exposure', 8192, 1024, '{"chatCompletions": true}',
		          true, $3, $4, $5, $6)`
	tests := []struct {
		name       string
		available  bool
		supported  bool
		audience   []string
		defaultFor []string
	}{
		{
			name: "unsupported", available: true, supported: false,
			audience: []string{"user"},
		},
		{
			name: "empty audience", available: true, supported: true,
			audience: []string{},
		},
		{
			name: "default outside audience", available: true, supported: true,
			audience: []string{"user"}, defaultFor: []string{"guest"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id := uuid.New()
			_, err := pool.Raw().Exec(
				ctx, insert, id, "invalid-"+id.String(), test.available, test.supported,
				test.audience, test.defaultFor,
			)
			if err == nil {
				_, _ = pool.Raw().Exec(ctx, `DELETE FROM models WHERE id = $1`, id)
				t.Fatal("invalid model exposure satisfied database constraints")
			}
		})
	}
}

func containsModel(items []Model, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
