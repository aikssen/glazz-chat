package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var ErrNotSelectable = errors.New("model is not selectable")

type Capabilities struct {
	ChatCompletions bool `json:"chatCompletions"`
	Markdown        bool `json:"markdown"`
	Code            bool `json:"code"`
}

type Model struct {
	ID              uuid.UUID    `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	ContextWindow   int32        `json:"-"`
	MaxOutputTokens int32        `json:"-"`
	Capabilities    Capabilities `json:"capabilities"`
}

type Selection struct {
	Model           Model
	ProviderID      uuid.UUID
	ProviderCode    string
	Adapter         string
	ProviderModelID string
}

type Service struct {
	database *database.Pool
}

func New(pool *database.Pool) *Service {
	return &Service{database: pool}
}

func (service *Service) List(ctx context.Context, actorType string) ([]Model, error) {
	records, err := service.database.Queries().ListPublicModels(ctx, actorType)
	if err != nil {
		return nil, fmt.Errorf("list public models: %w", err)
	}
	result := make([]Model, 0, len(records))
	for _, record := range records {
		model, err := mapModel(record)
		if err != nil {
			return nil, err
		}
		result = append(result, model)
	}
	return result, nil
}

func (service *Service) Default(ctx context.Context, actorType string) (Selection, error) {
	record, err := service.database.Queries().GetDefaultModel(ctx, actorType)
	if errors.Is(err, pgx.ErrNoRows) {
		return Selection{}, ErrNotSelectable
	}
	if err != nil {
		return Selection{}, fmt.Errorf("get default model: %w", err)
	}
	return service.selection(ctx, record)
}

func (service *Service) Select(
	ctx context.Context,
	id uuid.UUID,
	actorType string,
) (Selection, error) {
	record, err := service.database.Queries().GetSelectableModel(
		ctx, store.GetSelectableModelParams{ModelID: id, ActorType: actorType},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Selection{}, ErrNotSelectable
	}
	if err != nil {
		return Selection{}, fmt.Errorf("get selectable model: %w", err)
	}
	return service.selection(ctx, record)
}

func (service *Service) selection(ctx context.Context, record store.Model) (Selection, error) {
	model, err := mapModel(record)
	if err != nil {
		return Selection{}, err
	}
	mapping, err := service.database.Queries().GetProviderForModel(ctx, record.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Selection{}, ErrNotSelectable
	}
	if err != nil {
		return Selection{}, fmt.Errorf("get provider mapping: %w", err)
	}
	return Selection{
		Model: model, ProviderID: mapping.ProviderID, ProviderCode: mapping.ProviderCode,
		Adapter: mapping.Adapter, ProviderModelID: mapping.ProviderModelID,
	}, nil
}

func mapModel(record store.Model) (Model, error) {
	var capabilities Capabilities
	if err := json.Unmarshal(record.Capabilities, &capabilities); err != nil {
		return Model{}, fmt.Errorf("decode model capabilities: %w", err)
	}
	if !capabilities.ChatCompletions {
		return Model{}, ErrNotSelectable
	}
	return Model{
		ID: record.ID, Name: record.Name, Description: record.Description,
		ContextWindow: record.ContextWindow, MaxOutputTokens: record.MaxOutputTokens,
		Capabilities: capabilities,
	}, nil
}
