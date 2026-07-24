package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
)

type Snapshot struct {
	Maintenance             bool
	GuestMessageLimit       int64
	GuestOutputTokenLimit   int64
	UserMessageLimit        int64
	UserOutputTokenLimit    int64
	GlobalOutputTokenLimit  int64
	GlobalConcurrentStreams int64
	SystemPrompt            string
	InputSafetyCategories   []string
	OutputSafetyCategories  []string
}

type Service struct {
	database *database.Pool
}

func New(pool *database.Pool) *Service {
	return &Service{database: pool}
}

func (service *Service) Load(ctx context.Context) (Snapshot, error) {
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
	if err := decode(values, "safety.input_categories", &snapshot.InputSafetyCategories); err != nil {
		return Snapshot{}, err
	}
	if err := decode(values, "safety.output_categories", &snapshot.OutputSafetyCategories); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
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
