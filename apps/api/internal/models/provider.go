package models

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

const (
	FakeProviderCode       = "fake"
	ConfiguredProviderCode = "configured"
)

var configuredProviderID = uuid.MustParse("00000000-0000-7000-8000-000000000002")

func ProviderCode(kind string) string {
	if kind == FakeProviderCode {
		return FakeProviderCode
	}
	return ConfiguredProviderCode
}

func ConfigureProvider(
	ctx context.Context,
	pool *database.Pool,
	kind string,
	timeSource clock.Clock,
) (string, error) {
	code := ProviderCode(kind)
	now := pgtype.Timestamptz{Time: timeSource.Now().UTC(), Valid: true}
	err := pool.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		if code == FakeProviderCode {
			return queries.SetProviderEnabledByCode(ctx, store.SetProviderEnabledByCodeParams{
				Enabled: true, NowAt: now, Code: FakeProviderCode,
			})
		}
		settings, err := json.Marshal(map[string]bool{"runtimeConfigured": true})
		if err != nil {
			return err
		}
		if _, err := queries.UpsertProvider(ctx, store.UpsertProviderParams{
			ID: configuredProviderID, Code: ConfiguredProviderCode,
			DisplayName: "Configured LLM provider", Adapter: "openai_compatible",
			Enabled: true, HealthStatus: "healthy", Settings: settings, NowAt: now,
		}); err != nil {
			return fmt.Errorf("register configured model provider: %w", err)
		}
		if err := queries.SetProviderEnabledByCode(ctx, store.SetProviderEnabledByCodeParams{
			Enabled: false, NowAt: now, Code: FakeProviderCode,
		}); err != nil {
			return fmt.Errorf("disable fake model provider: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return code, nil
}
