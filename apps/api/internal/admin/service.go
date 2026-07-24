package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrNotFound = errors.New("admin resource not found")
	ErrConflict = errors.New("admin resource version conflict")
	ErrInvalid  = errors.New("admin input is invalid")
)

type Setting struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Version   int32     `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Model struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Capabilities map[string]any `json:"capabilities"`
	Enabled      bool           `json:"enabled"`
	Available    bool           `json:"available"`
	Supported    bool           `json:"supported"`
	Audience     []string       `json:"audience"`
	DefaultFor   []string       `json:"defaultFor"`
	Order        int32          `json:"order"`
	Version      int32          `json:"version"`
}

type ModelUpdate struct {
	Enabled    *bool
	Audience   *[]string
	DefaultFor *[]string
	Order      *int32
}

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	Version     int32     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type Usage struct {
	PeriodStart   time.Time `json:"periodStart"`
	PeriodEnd     time.Time `json:"periodEnd"`
	Generations   int64     `json:"generations"`
	InputTokens   int64     `json:"inputTokens"`
	OutputTokens  int64     `json:"outputTokens"`
	EstimatedCost float64   `json:"estimatedCost"`
	Currency      string    `json:"currency"`
}

type AuditEvent struct {
	ID         uuid.UUID      `json:"id"`
	ActorID    *uuid.UUID     `json:"actorId"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
}

type Service struct {
	database *database.Pool
	ids      ids.Source
	clock    clock.Clock
}

func New(pool *database.Pool, idSource ids.Source, timeSource clock.Clock) *Service {
	return &Service{database: pool, ids: idSource, clock: timeSource}
}

func (service *Service) ListSettings(ctx context.Context) ([]Setting, error) {
	records, err := service.database.Queries().ListRuntimeSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list runtime settings: %w", err)
	}
	result := make([]Setting, 0, len(records))
	for _, record := range records {
		mapped, err := mapSetting(record)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (service *Service) UpdateSetting(
	ctx context.Context,
	actorID uuid.UUID,
	key string,
	value json.RawMessage,
	expectedVersion int32,
	requestID string,
) (Setting, error) {
	if actorID == uuid.Nil || expectedVersion <= 0 || !validSettingValue(key, value) {
		return Setting{}, ErrInvalid
	}
	var updated store.RuntimeSetting
	err := service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		before, err := queries.GetRuntimeSetting(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		updated, err = queries.UpdateRuntimeSetting(ctx, store.UpdateRuntimeSettingParams{
			Value: value, UpdatedByUserID: &actorID, NowAt: timestamp(service.clock.Now()),
			Key: key, ExpectedVersion: expectedVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return service.audit(ctx, queries, actorID, "setting.updated", "runtime_setting", key,
			map[string]any{"value": json.RawMessage(before.Value), "version": before.Version},
			map[string]any{"value": json.RawMessage(updated.Value), "version": updated.Version},
			requestID,
		)
	})
	if err != nil {
		return Setting{}, err
	}
	return mapSetting(updated)
}

func (service *Service) ListModels(ctx context.Context) ([]Model, error) {
	records, err := service.database.Queries().ListAdminModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list admin models: %w", err)
	}
	result := make([]Model, 0, len(records))
	for _, record := range records {
		mapped, err := mapModel(record)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (service *Service) UpdateModel(
	ctx context.Context,
	actorID uuid.UUID,
	modelID uuid.UUID,
	input ModelUpdate,
	expectedVersion int32,
	requestID string,
) (Model, error) {
	if actorID == uuid.Nil || modelID == uuid.Nil || expectedVersion <= 0 ||
		(input.Enabled == nil && input.Audience == nil && input.DefaultFor == nil && input.Order == nil) {
		return Model{}, ErrInvalid
	}
	var updated store.Model
	err := service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		before, err := queries.GetAdminModel(ctx, modelID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		enabled := before.Enabled
		audience := before.Audience
		defaultFor := before.DefaultFor
		order := before.SortOrder
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		if input.Audience != nil {
			audience = *input.Audience
		}
		if input.DefaultFor != nil {
			defaultFor = *input.DefaultFor
		}
		if input.Order != nil {
			order = *input.Order
		}
		if !validAudience(audience, defaultFor) || order < 0 ||
			(enabled && (!before.Available || !before.Supported || len(audience) == 0)) {
			return ErrInvalid
		}
		updated, err = queries.UpdateAdminModel(ctx, store.UpdateAdminModelParams{
			Enabled: input.Enabled, Audience: optionalList(input.Audience),
			DefaultFor: optionalList(input.DefaultFor), SortOrder: input.Order,
			NowAt: timestamp(service.clock.Now()), ID: modelID, ExpectedVersion: expectedVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		for _, actorType := range []string{"guest", "user"} {
			count, err := queries.CountSelectableDefaults(ctx, actorType)
			if err != nil {
				return err
			}
			if count == 0 {
				return ErrConflict
			}
		}
		return service.audit(ctx, queries, actorID, "model.updated", "model", modelID.String(),
			modelAuditValue(before), modelAuditValue(updated), requestID)
	})
	if err != nil {
		return Model{}, err
	}
	return mapModel(updated)
}

func (service *Service) ListUsers(
	ctx context.Context,
	query string,
	cursor string,
	limit int32,
) (Page[User], error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 || len(query) > 200 {
		return Page[User]{}, ErrInvalid
	}
	createdAt, id, err := decodeCursor(cursor)
	if err != nil {
		return Page[User]{}, ErrInvalid
	}
	var search *string
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		search = &trimmed
	}
	records, err := service.database.Queries().ListAdminUsers(ctx, store.ListAdminUsersParams{
		Query: search, AfterCreatedAt: createdAt, AfterID: id, PageSize: limit + 1,
	})
	if err != nil {
		return Page[User]{}, fmt.Errorf("list admin users: %w", err)
	}
	page := Page[User]{Items: make([]User, 0, min(len(records), int(limit)))}
	for index, record := range records {
		if index == int(limit) {
			last := records[index-1]
			page.NextCursor = encodeCursor(last.CreatedAt.Time, last.ID)
			break
		}
		page.Items = append(page.Items, mapUser(record))
	}
	return page, nil
}

func (service *Service) UpdateUserRole(
	ctx context.Context,
	actorID uuid.UUID,
	userID uuid.UUID,
	role string,
	expectedVersion int32,
	requestID string,
) (User, error) {
	if actorID == uuid.Nil || userID == uuid.Nil || expectedVersion <= 0 ||
		(role != "user" && role != "admin") {
		return User{}, ErrInvalid
	}
	var updated store.User
	err := service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		before, err := queries.GetAdminUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if before.Role == "admin" && role == "user" {
			if actorID == userID {
				return ErrConflict
			}
			count, err := queries.CountAdministrators(ctx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrConflict
			}
		}
		updated, err = queries.UpdateUserRole(ctx, store.UpdateUserRoleParams{
			Role: role, NowAt: timestamp(service.clock.Now()), ID: userID,
			ExpectedVersion: expectedVersion,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return service.audit(ctx, queries, actorID, "user.role_updated", "user", userID.String(),
			map[string]any{"role": before.Role, "version": before.Version},
			map[string]any{"role": updated.Role, "version": updated.Version},
			requestID,
		)
	})
	if err != nil {
		return User{}, err
	}
	return mapUser(updated), nil
}

func (service *Service) Usage(
	ctx context.Context,
	start time.Time,
	end time.Time,
) (Usage, error) {
	if start.IsZero() || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return Usage{}, ErrInvalid
	}
	record, err := service.database.Queries().AggregateAdminUsage(ctx, store.AggregateAdminUsageParams{
		PeriodStart: timestamp(start), PeriodEnd: timestamp(end),
	})
	if err != nil {
		return Usage{}, fmt.Errorf("aggregate admin usage: %w", err)
	}
	return Usage{
		PeriodStart: start.UTC(), PeriodEnd: end.UTC(), Generations: record.Generations,
		InputTokens: record.InputTokens, OutputTokens: record.OutputTokens,
		EstimatedCost: float64(record.EstimatedCostMicrounits) / 1_000_000,
		Currency:      "USD",
	}, nil
}

func (service *Service) Audit(
	ctx context.Context,
	cursor string,
	limit int32,
) (Page[AuditEvent], error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		return Page[AuditEvent]{}, ErrInvalid
	}
	occurredAt, id, err := decodeCursor(cursor)
	if err != nil {
		return Page[AuditEvent]{}, ErrInvalid
	}
	records, err := service.database.Queries().ListAdminAuditEvents(ctx, store.ListAdminAuditEventsParams{
		BeforeOccurredAt: occurredAt, BeforeID: id, PageSize: limit + 1,
	})
	if err != nil {
		return Page[AuditEvent]{}, fmt.Errorf("list admin audit: %w", err)
	}
	page := Page[AuditEvent]{Items: make([]AuditEvent, 0, min(len(records), int(limit)))}
	for index, record := range records {
		if index == int(limit) {
			last := records[index-1]
			page.NextCursor = encodeCursor(last.OccurredAt.Time, last.ID)
			break
		}
		page.Items = append(page.Items, mapAudit(record))
	}
	return page, nil
}

func (service *Service) audit(
	ctx context.Context,
	queries *store.Queries,
	actorID uuid.UUID,
	action string,
	targetType string,
	targetID string,
	before any,
	after any,
	requestID string,
) error {
	id, err := service.ids.New()
	if err != nil {
		return err
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return queries.InsertAdminAudit(ctx, store.InsertAdminAuditParams{
		ID: id, ActorUserID: &actorID, Action: action, TargetType: targetType,
		TargetID: targetID, BeforeValue: beforeJSON, AfterValue: afterJSON,
		RequestID: optionalString(requestID),
	})
}

func mapSetting(record store.RuntimeSetting) (Setting, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(record.Value))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return Setting{}, fmt.Errorf("decode setting %s: %w", record.Key, err)
	}
	return Setting{
		Key: record.Key, Value: value, Version: record.Version,
		UpdatedAt: record.UpdatedAt.Time.UTC(),
	}, nil
}

func mapModel(record store.Model) (Model, error) {
	var capabilities map[string]any
	if err := json.Unmarshal(record.Capabilities, &capabilities); err != nil {
		return Model{}, fmt.Errorf("decode model capabilities: %w", err)
	}
	return Model{
		ID: record.ID, Name: record.Name, Description: record.Description,
		Capabilities: capabilities, Enabled: record.Enabled, Available: record.Available,
		Supported: record.Supported, Audience: record.Audience, DefaultFor: record.DefaultFor,
		Order: record.SortOrder, Version: record.Version,
	}, nil
}

func mapUser(record store.User) User {
	return User{
		ID: record.ID, Email: record.Email, DisplayName: record.DisplayName,
		Role: record.Role, Status: record.Status, Version: record.Version,
		CreatedAt: record.CreatedAt.Time.UTC(),
	}
}

func mapAudit(record store.AdminAuditLog) AuditEvent {
	event := AuditEvent{
		ID: record.ID, ActorID: record.ActorUserID, Action: record.Action,
		TargetType: record.TargetType, TargetID: record.TargetID,
		OccurredAt: record.OccurredAt.Time.UTC(),
	}
	_ = json.Unmarshal(record.BeforeValue, &event.Before)
	_ = json.Unmarshal(record.AfterValue, &event.After)
	return event
}

func validSettingValue(key string, raw json.RawMessage) bool {
	definitions := map[string]struct {
		kind string
		min  int64
		max  int64
	}{
		"maintenance.enabled":                 {kind: "boolean"},
		"quota.guest.messages":                {kind: "integer", min: 1, max: 20},
		"quota.guest.output_tokens":           {kind: "integer", min: 128, max: 10000},
		"quota.user.messages":                 {kind: "integer", min: 1, max: 1000},
		"quota.user.output_tokens":            {kind: "integer", min: 128, max: 1000000},
		"quota.global.output_tokens":          {kind: "integer", min: 128, max: 1000000000000},
		"quota.global.concurrent_generations": {kind: "integer", min: 1, max: 10000},
		"chat.system_prompt":                  {kind: "string", min: 1, max: 10000},
		"chat.summary_model_id":               {kind: "string", min: 36, max: 36},
		"safety.input_categories":             {kind: "string_array", max: 50},
		"safety.output_categories":            {kind: "string_array", max: 50},
	}
	definition, ok := definitions[key]
	if !ok || len(raw) == 0 {
		return false
	}
	switch definition.kind {
	case "boolean":
		var value bool
		return json.Unmarshal(raw, &value) == nil && string(raw) != "null"
	case "integer":
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			return false
		}
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		parsed, err := strconv.ParseInt(string(number), 10, 64)
		return err == nil && parsed >= definition.min && parsed <= definition.max
	case "string":
		var value string
		valid := json.Unmarshal(raw, &value) == nil &&
			int64(len(strings.TrimSpace(value))) >= definition.min &&
			int64(len(value)) <= definition.max
		if valid && key == "chat.summary_model_id" {
			_, err := uuid.Parse(value)
			return err == nil
		}
		return valid
	case "string_array":
		var values []string
		if json.Unmarshal(raw, &values) != nil || int64(len(values)) > definition.max {
			return false
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len(value) > 80 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validAudience(audience []string, defaults []string) bool {
	seenAudience := map[string]bool{}
	for _, value := range audience {
		if (value != "guest" && value != "user") || seenAudience[value] {
			return false
		}
		seenAudience[value] = true
	}
	seenDefaults := map[string]bool{}
	for _, value := range defaults {
		if !seenAudience[value] || seenDefaults[value] {
			return false
		}
		seenDefaults[value] = true
	}
	return true
}

func modelAuditValue(record store.Model) map[string]any {
	return map[string]any{
		"enabled": record.Enabled, "audience": record.Audience,
		"defaultFor": record.DefaultFor, "order": record.SortOrder, "version": record.Version,
	}
}

func optionalList(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func encodeCursor(at time.Time, id uuid.UUID) string {
	payload, _ := json.Marshal(struct {
		At time.Time `json:"at"`
		ID uuid.UUID `json:"id"`
	}{At: at.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCursor(value string) (pgtype.Timestamptz, *uuid.UUID, error) {
	if value == "" {
		return pgtype.Timestamptz{}, nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return pgtype.Timestamptz{}, nil, err
	}
	var cursor struct {
		At time.Time `json:"at"`
		ID uuid.UUID `json:"id"`
	}
	if json.Unmarshal(raw, &cursor) != nil || cursor.At.IsZero() || cursor.ID == uuid.Nil {
		return pgtype.Timestamptz{}, nil, ErrInvalid
	}
	return timestamp(cursor.At), &cursor.ID, nil
}
