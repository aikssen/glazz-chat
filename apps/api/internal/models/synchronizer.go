package models

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

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

const (
	discoveredContextWindow   = 131072
	discoveredMaxOutputTokens = 8192
)

var validModelSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,127}$`)

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
		mappings := make(map[string]store.ProviderModel, len(before))
		for _, mapping := range before {
			mappings[mapping.ProviderModelID] = mapping
		}
		upstreamIDs := make([]string, 0, len(discovered))
		for upstreamID := range discovered {
			upstreamIDs = append(upstreamIDs, upstreamID)
		}
		sort.Strings(upstreamIDs)
		present := make([]string, 0, len(before))
		for index, upstreamID := range upstreamIDs {
			upstream := discovered[upstreamID]
			mapping, ok := mappings[upstreamID]
			if !ok {
				model, err := synchronizer.discoverModel(
					ctx, queries, upstream, int32(index+100), now, requestID,
				)
				if err != nil {
					return err
				}
				mapping = store.ProviderModel{
					ProviderID: record.ID, ModelID: model.ID, ProviderModelID: upstreamID,
				}
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
				ProviderModelID: upstreamID, Metadata: metadata, SyncedAt: now,
			}); err != nil {
				return fmt.Errorf("refresh provider mapping: %w", err)
			}
			present = append(present, upstreamID)
			result.Mapped++
		}
		result.Ignored = len(catalog) - len(discovered)
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
			if !ok || previous == mapping.Available {
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

func (synchronizer *Synchronizer) discoverModel(
	ctx context.Context,
	queries *store.Queries,
	upstream provider.Model,
	sortOrder int32,
	now pgtype.Timestamptz,
	requestID string,
) (store.Model, error) {
	slug := modelSlug(upstream.ID)
	model, err := queries.GetModelBySlug(ctx, slug)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.Model{}, fmt.Errorf("find discovered model: %w", err)
	}
	modelID, err := synchronizer.ids.New()
	if err != nil {
		return store.Model{}, err
	}
	contextWindow := int32(upstream.ContextWindow)
	if contextWindow < 1024 {
		contextWindow = discoveredContextWindow
	}
	maxOutputTokens := int32(upstream.MaxOutputTokens)
	if maxOutputTokens < 1 || maxOutputTokens > contextWindow {
		maxOutputTokens = min(discoveredMaxOutputTokens, contextWindow)
	}
	capabilities, err := json.Marshal(map[string]bool{
		"chatCompletions": true, "markdown": true, "code": true,
	})
	if err != nil {
		return store.Model{}, err
	}
	model, err = queries.CreateDiscoveredModel(ctx, store.CreateDiscoveredModelParams{
		ID: modelID, Slug: slug, Name: modelDisplayName(upstream.ID),
		Description:   "Discovered from the configured LLM provider.",
		ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens,
		Capabilities: capabilities, SortOrder: sortOrder, NowAt: now,
	})
	if err != nil {
		return store.Model{}, fmt.Errorf("create discovered model: %w", err)
	}
	if err := synchronizer.auditDiscovery(ctx, queries, model, upstream.ID, requestID); err != nil {
		return store.Model{}, err
	}
	return model, nil
}

func (synchronizer *Synchronizer) auditDiscovery(
	ctx context.Context,
	queries *store.Queries,
	model store.Model,
	upstreamID string,
	requestID string,
) error {
	auditID, err := synchronizer.ids.New()
	if err != nil {
		return err
	}
	after, _ := json.Marshal(map[string]any{
		"name": model.Name, "providerModelId": upstreamID, "enabled": false,
	})
	if err := queries.InsertAdminAudit(ctx, store.InsertAdminAuditParams{
		ID: auditID, Action: "model.discovered", TargetType: "model",
		TargetID: model.ID.String(), AfterValue: after, RequestID: nullable(requestID),
	}); err != nil {
		return fmt.Errorf("audit discovered model: %w", err)
	}
	return nil
}

func modelSlug(upstreamID string) string {
	normalized := strings.ToLower(strings.TrimSpace(upstreamID))
	if validModelSlug.MatchString(normalized) {
		return normalized
	}
	var builder strings.Builder
	for _, character := range normalized {
		if unicode.IsLetter(character) || unicode.IsDigit(character) ||
			character == '.' || character == '_' || character == '-' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	base := strings.Trim(builder.String(), "-._")
	if len(base) > 100 {
		base = base[:100]
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(upstreamID)))[:12]
	if base == "" {
		base = "model"
	}
	return "model-" + base + "-" + sum
}

func modelDisplayName(upstreamID string) string {
	words := strings.FieldsFunc(upstreamID, func(character rune) bool {
		return character == '-' || character == '_' || character == '/'
	})
	for index, word := range words {
		lower := strings.ToLower(word)
		switch lower {
		case "glm":
			words[index] = "GLM"
		case "qwen":
			words[index] = "Qwen"
		case "deepseek":
			words[index] = "DeepSeek"
		case "kimi":
			words[index] = "Kimi"
		case "minimax":
			words[index] = "MiniMax"
		case "mimo":
			words[index] = "MiMo"
		default:
			if len(word) > 0 {
				words[index] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
	}
	if len(words) == 0 {
		return "Discovered model"
	}
	return strings.Join(words, " ")
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
