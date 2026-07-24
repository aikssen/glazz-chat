package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
)

const (
	cacheNamespace = "runtime-settings"
	cacheID        = "snapshot"
	cacheTTL       = 10 * time.Second
)

type Cache interface {
	Get(ctx context.Context, namespace, id string) (string, error)
	Put(ctx context.Context, namespace, id, value string, ttl time.Duration) error
	Delete(ctx context.Context, namespace, id string) error
}

type Snapshot struct {
	Maintenance             bool     `json:"maintenance"`
	GuestMessageLimit       int64    `json:"guestMessageLimit"`
	GuestOutputTokenLimit   int64    `json:"guestOutputTokenLimit"`
	UserMessageLimit        int64    `json:"userMessageLimit"`
	UserOutputTokenLimit    int64    `json:"userOutputTokenLimit"`
	GlobalOutputTokenLimit  int64    `json:"globalOutputTokenLimit"`
	GlobalConcurrentStreams int64    `json:"globalConcurrentStreams"`
	SystemPrompt            string   `json:"systemPrompt"`
	SummaryModelID          string   `json:"summaryModelId"`
	InputSafetyCategories   []string `json:"inputSafetyCategories"`
	OutputSafetyCategories  []string `json:"outputSafetyCategories"`
}

type Service struct {
	database *database.Pool
	cache    Cache
}

func New(pool *database.Pool, caches ...Cache) *Service {
	service := &Service{database: pool}
	if len(caches) > 0 {
		service.cache = caches[0]
	}
	return service
}

func (service *Service) Load(ctx context.Context) (Snapshot, error) {
	if service.cache != nil {
		if cached, err := service.cache.Get(ctx, cacheNamespace, cacheID); err == nil {
			var snapshot Snapshot
			if json.Unmarshal([]byte(cached), &snapshot) == nil {
				return snapshot, nil
			}
			_ = service.cache.Delete(ctx, cacheNamespace, cacheID)
		}
	}
	records, err := service.database.Queries().ListRuntimeSettings(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list runtime settings: %w", err)
	}
	values := make(map[string]json.RawMessage, len(records))
	for _, record := range records {
		values[record.Key] = record.Value
	}
	var snapshot Snapshot
	if err := decode(values, "maintenance.enabled", &snapshot.Maintenance); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.guest.messages", &snapshot.GuestMessageLimit); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.guest.output_tokens", &snapshot.GuestOutputTokenLimit); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.user.messages", &snapshot.UserMessageLimit); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.user.output_tokens", &snapshot.UserOutputTokenLimit); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.global.output_tokens", &snapshot.GlobalOutputTokenLimit); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "quota.global.concurrent_generations", &snapshot.GlobalConcurrentStreams); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "chat.system_prompt", &snapshot.SystemPrompt); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "chat.summary_model_id", &snapshot.SummaryModelID); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "safety.input_categories", &snapshot.InputSafetyCategories); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "safety.output_categories", &snapshot.OutputSafetyCategories); err != nil {
		return Snapshot{}, err
	}
	if service.cache != nil {
		if encoded, err := json.Marshal(snapshot); err == nil {
			_ = service.cache.Put(ctx, cacheNamespace, cacheID, string(encoded), cacheTTL)
		}
	}
	return snapshot, nil
}

func (service *Service) Invalidate(ctx context.Context) error {
	if service.cache == nil {
		return nil
	}
	if err := service.cache.Delete(ctx, cacheNamespace, cacheID); err != nil {
		return fmt.Errorf("invalidate runtime settings cache: %w", err)
	}
	return nil
}

func decode[T any](values map[string]json.RawMessage, key string, target *T) error {
	raw, ok := values[key]
	if !ok {
		return fmt.Errorf("runtime setting %q is missing", key)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode runtime setting %q: %w", key, err)
	}
	return nil
}
