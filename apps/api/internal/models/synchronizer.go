package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
)

var ErrProviderNotFound = errors.New("model provider not found")

type SyncResult struct {
	Discovered int
	Mapped     int
	Ignored    int
	Changed    int
}

type Synchronizer struct {
	database *database.Pool
	ids      ids.Source
	clock    clock.Clock
}

func NewSynchronizer(
	pool *database.Pool,
	idSource ids.Source,
	timeSource clock.Clock,
) *Synchronizer {
	return &Synchronizer{database: pool, ids: idSource, clock: timeSource}
}

func (synchronizer *Synchronizer) Sync(
	ctx context.Context,
	providerCode string,
	gateway provider.Gateway,
	requestID string,
) (SyncResult, error) {
	if providerCode == "" || gateway == nil {
		return SyncResult{}, errors.New("provider code and gateway are required")
	}
	catalog, err := gateway.Catalog(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("fetch provider catalog: %w", err)
	}
	discovered := make(map[string]provider.Model, len(catalog))
	for _, item := range catalog {
		if item.ID != "" && item.ChatCompletions {
			discovered[item.ID] = item
		}
	}
	result := SyncResult{Discovered: len(catalog)}
	now := pgtype.Timestamptz{Time: synchronizer.clock.Now().UTC(), Valid: true}
	err = synchronizer.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		record, err := queries.GetProviderByCode(ctx, providerCode)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProviderNotFound
		}
		if err != nil {
			return fmt.Errorf("read provider: %w", err)
		}
		before, err := queries.ListProviderModelMappings(ctx, record.ID)
		if err != nil {
			return fmt.Errorf("list provider mappings: %w", err)
		}
		present := make([]string, 0, len(before))
		for _, mapping := range before {
			upstream, ok := discovered[mapping.ProviderModelID]
			if !ok {
				continue
			}
			metadata, err := json.Marshal(map[string]any{
				"contextWindow":   upstream.ContextWindow,
				"maxOutputTokens": upstream.MaxOutputTokens,
				"chatCompletions": upstream.ChatCompletions,
			})
			if err != nil {
				return err
			}
			if err := queries.UpsertProviderModel(ctx, store.UpsertProviderModelParams{
				ProviderID: record.ID, ModelID: mapping.ModelID,
				ProviderModelID: mapping.ProviderModelID, Metadata: metadata, SyncedAt: now,
			}); err != nil {
				return fmt.Errorf("refresh provider mapping: %w", err)
			}
			present = append(present, mapping.ProviderModelID)
			result.Mapped++
		}
		result.Ignored = len(catalog) - result.Mapped
		if err := queries.MarkProviderModelsUnavailable(ctx, store.MarkProviderModelsUnavailableParams{
			SyncedAt: now, ProviderID: record.ID, PresentModelIds: present,
		}); err != nil {
			return fmt.Errorf("mark missing provider mappings: %w", err)
		}
		if _, err := queries.RefreshModelAvailability(ctx, now); err != nil {
			return fmt.Errorf("refresh model availability: %w", err)
		}
		after, err := queries.ListProviderModelMappings(ctx, record.ID)
		if err != nil {
			return fmt.Errorf("reload provider mappings: %w", err)
		}
		beforeAvailability := make(map[uuid.UUID]bool, len(before))
		for _, mapping := range before {
			beforeAvailability[mapping.ModelID] = mapping.Available
		}
		for _, mapping := range after {
			previous, ok := beforeAvailability[mapping.ModelID]
			if ok && previous == mapping.Available {
				continue
			}
			auditID, err := synchronizer.ids.New()
			if err != nil {
				return err
			}
			beforeValue, _ := json.Marshal(map[string]bool{"available": previous})
			afterValue, _ := json.Marshal(map[string]bool{"available": mapping.Available})
			if err := queries.InsertAdminAudit(ctx, store.InsertAdminAuditParams{
				ID: auditID, Action: "model.provider_availability_changed",
				TargetType: "model", TargetID: mapping.ModelID.String(),
				BeforeValue: beforeValue, AfterValue: afterValue,
				RequestID: nullable(requestID),
			}); err != nil {
				return fmt.Errorf("audit provider mapping: %w", err)
			}
			result.Changed++
		}
		return nil
	})
	if err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
