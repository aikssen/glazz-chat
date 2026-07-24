package conversations

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrNotFound   = errors.New("conversation not found")
	ErrConflict   = errors.New("conversation conflicts with current state")
	ErrInvalid    = errors.New("conversation input is invalid")
	ErrGuestScope = errors.New("operation is unavailable to guest")
)

type ActorType string

const (
	ActorUser  ActorType = "user"
	ActorGuest ActorType = "guest"
)

type Actor struct {
	Type ActorType
	ID   uuid.UUID
}

func (actor Actor) TypeString() string {
	return string(actor.Type)
}

type Conversation struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	ModelID         uuid.UUID `json:"modelId"`
	GenerationState string    `json:"generationState"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Status         string    `json:"status"`
	Sequence       int32     `json:"sequence"`
	CreatedAt      time.Time `json:"createdAt"`
}

type CreateInput struct {
	Title   string
	ModelID *uuid.UUID
}

type UpdateInput struct {
	Title    *string
	Archived *bool
	ModelID  *uuid.UUID
}

type ListInput struct {
	Limit           int32
	Cursor          string
	Search          string
	IncludeArchived bool
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ModelCatalog interface {
	Default(context.Context, string) (models.Selection, error)
	Select(context.Context, uuid.UUID, string) (models.Selection, error)
}

type Service struct {
	database *database.Pool
	models   ModelCatalog
	ids      ids.Source
	clock    clock.Clock
}

func New(
	pool *database.Pool,
	modelCatalog ModelCatalog,
	idSource ids.Source,
	timeSource clock.Clock,
) *Service {
	return &Service{database: pool, models: modelCatalog, ids: idSource, clock: timeSource}
}

func (service *Service) Create(
	ctx context.Context,
	actor Actor,
	input CreateInput,
) (Conversation, error) {
	if err := validActor(actor); err != nil {
		return Conversation{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New conversation"
	}
	if len(title) > 120 {
		return Conversation{}, ErrInvalid
	}
	var selection models.Selection
	var err error
	if input.ModelID == nil {
		selection, err = service.models.Default(ctx, string(actor.Type))
	} else {
		selection, err = service.models.Select(ctx, *input.ModelID, string(actor.Type))
	}
	if errors.Is(err, models.ErrNotSelectable) {
		return Conversation{}, ErrInvalid
	}
	if err != nil {
		return Conversation{}, err
	}
	id, err := service.ids.New()
	if err != nil {
		return Conversation{}, err
	}
	now := timestamp(service.clock.Now())
	var record store.Conversation
	if actor.Type == ActorUser {
		record, err = service.database.Queries().CreateUserConversation(
			ctx, store.CreateUserConversationParams{
				ID: id, UserID: &actor.ID, Title: title,
				ModelID: selection.Model.ID, NowAt: now,
			},
		)
	} else {
		record, err = service.database.Queries().CreateGuestConversation(
			ctx, store.CreateGuestConversationParams{
				ID: id, GuestSessionID: &actor.ID, Title: title,
				ModelID: selection.Model.ID, NowAt: now,
			},
		)
	}
	if uniqueViolation(err) {
		return Conversation{}, ErrConflict
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return mapConversation(record), nil
}

func (service *Service) Get(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
) (Conversation, error) {
	record, err := service.owned(ctx, actor, conversationID)
	if err != nil {
		return Conversation{}, err
	}
	return mapConversation(record), nil
}

func (service *Service) List(
	ctx context.Context,
	actor Actor,
	input ListInput,
) (Page[Conversation], error) {
	if err := validActor(actor); err != nil {
		return Page[Conversation]{}, err
	}
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		return Page[Conversation]{}, ErrInvalid
	}
	if len(input.Search) > 200 {
		return Page[Conversation]{}, ErrInvalid
	}
	if actor.Type == ActorGuest {
		record, err := service.database.Queries().GetGuestConversationByOwner(ctx, &actor.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Page[Conversation]{Items: []Conversation{}}, nil
		}
		if err != nil {
			return Page[Conversation]{}, fmt.Errorf("list guest conversation: %w", err)
		}
		return Page[Conversation]{Items: []Conversation{mapConversation(record)}}, nil
	}
	cursor, err := decodeConversationCursor(input.Cursor)
	if err != nil {
		return Page[Conversation]{}, ErrInvalid
	}
	var search *string
	if trimmed := strings.TrimSpace(input.Search); trimmed != "" {
		search = &trimmed
	}
	records, err := service.database.Queries().ListUserConversations(
		ctx, store.ListUserConversationsParams{
			UserID: &actor.ID, IncludeArchived: input.IncludeArchived, Search: search,
			BeforeUpdatedAt: cursor.UpdatedAt, BeforeID: cursor.ID, PageSize: input.Limit + 1,
		},
	)
	if err != nil {
		return Page[Conversation]{}, fmt.Errorf("list conversations: %w", err)
	}
	page := Page[Conversation]{Items: make([]Conversation, 0, min(len(records), int(input.Limit)))}
	for index, record := range records {
		if index == int(input.Limit) {
			last := records[index-1]
			page.NextCursor = encodeConversationCursor(last.UpdatedAt.Time, last.ID)
			break
		}
		page.Items = append(page.Items, mapConversation(record))
	}
	return page, nil
}

func (service *Service) Update(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
	input UpdateInput,
) (Conversation, error) {
	if _, err := service.owned(ctx, actor, conversationID); err != nil {
		return Conversation{}, err
	}
	var title *string
	if input.Title != nil {
		trimmed := strings.TrimSpace(*input.Title)
		if trimmed == "" || len(trimmed) > 120 {
			return Conversation{}, ErrInvalid
		}
		title = &trimmed
	}
	if actor.Type == ActorGuest && input.Archived != nil {
		return Conversation{}, ErrGuestScope
	}
	if input.ModelID != nil {
		if _, err := service.models.Select(ctx, *input.ModelID, string(actor.Type)); err != nil {
			if errors.Is(err, models.ErrNotSelectable) {
				return Conversation{}, ErrInvalid
			}
			return Conversation{}, err
		}
	}
	now := timestamp(service.clock.Now())
	var record store.Conversation
	var err error
	if actor.Type == ActorGuest {
		record, err = service.database.Queries().UpdateGuestConversation(
			ctx, store.UpdateGuestConversationParams{
				Title: title, ModelID: input.ModelID, NowAt: now,
				ID: conversationID, GuestSessionID: &actor.ID,
			},
		)
	} else {
		var status *string
		if input.Archived != nil {
			value := "active"
			if *input.Archived {
				value = "archived"
			}
			status = &value
		}
		record, err = service.database.Queries().UpdateUserConversation(
			ctx, store.UpdateUserConversationParams{
				Title: title, Status: status, ModelID: input.ModelID, NowAt: now,
				ID: conversationID, UserID: &actor.ID,
			},
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, ErrConflict
	}
	if err != nil {
		return Conversation{}, fmt.Errorf("update conversation: %w", err)
	}
	return mapConversation(record), nil
}

func (service *Service) Delete(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
) error {
	if _, err := service.owned(ctx, actor, conversationID); err != nil {
		return err
	}
	now := timestamp(service.clock.Now())
	var affected int64
	var err error
	if actor.Type == ActorGuest {
		affected, err = service.database.Queries().SoftDeleteGuestConversation(
			ctx, store.SoftDeleteGuestConversationParams{
				NowAt: now, ID: conversationID, GuestSessionID: &actor.ID,
			},
		)
	} else {
		affected, err = service.database.Queries().SoftDeleteUserConversation(
			ctx, store.SoftDeleteUserConversationParams{
				NowAt: now, ID: conversationID, UserID: &actor.ID,
			},
		)
	}
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func (service *Service) Messages(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
	limit int32,
	beforeSequence *int32,
) (Page[Message], error) {
	if _, err := service.owned(ctx, actor, conversationID); err != nil {
		return Page[Message]{}, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		return Page[Message]{}, ErrInvalid
	}
	records, err := service.database.Queries().ListConversationMessages(
		ctx, store.ListConversationMessagesParams{
			ConversationID: conversationID, BeforeSequence: beforeSequence, PageSize: limit + 1,
		},
	)
	if err != nil {
		return Page[Message]{}, fmt.Errorf("list conversation messages: %w", err)
	}
	page := Page[Message]{Items: make([]Message, 0, min(len(records), int(limit)))}
	for index, record := range records {
		if index == int(limit) {
			page.NextCursor = fmt.Sprintf("%d", records[index-1].Sequence)
			break
		}
		page.Items = append(page.Items, mapMessage(record))
	}
	return page, nil
}

func (service *Service) OwnedRecord(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
) (store.Conversation, error) {
	return service.owned(ctx, actor, conversationID)
}

func (service *Service) owned(
	ctx context.Context,
	actor Actor,
	conversationID uuid.UUID,
) (store.Conversation, error) {
	if err := validActor(actor); err != nil || conversationID == uuid.Nil {
		return store.Conversation{}, ErrInvalid
	}
	var record store.Conversation
	var err error
	if actor.Type == ActorUser {
		record, err = service.database.Queries().GetUserConversation(
			ctx, store.GetUserConversationParams{ID: conversationID, UserID: &actor.ID},
		)
	} else {
		record, err = service.database.Queries().GetGuestConversation(
			ctx, store.GetGuestConversationParams{ID: conversationID, GuestSessionID: &actor.ID},
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Conversation{}, ErrNotFound
	}
	if err != nil {
		return store.Conversation{}, fmt.Errorf("get owned conversation: %w", err)
	}
	return record, nil
}

type conversationCursor struct {
	UpdatedAt pgtype.Timestamptz
	ID        *uuid.UUID
}

func encodeConversationCursor(updatedAt time.Time, id uuid.UUID) string {
	payload, _ := json.Marshal(struct {
		UpdatedAt time.Time `json:"updatedAt"`
		ID        uuid.UUID `json:"id"`
	}{UpdatedAt: updatedAt.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeConversationCursor(value string) (conversationCursor, error) {
	if value == "" {
		return conversationCursor{}, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return conversationCursor{}, err
	}
	var cursor struct {
		UpdatedAt time.Time `json:"updatedAt"`
		ID        uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(payload, &cursor); err != nil ||
		cursor.UpdatedAt.IsZero() || cursor.ID == uuid.Nil {
		return conversationCursor{}, ErrInvalid
	}
	return conversationCursor{
		UpdatedAt: timestamp(cursor.UpdatedAt), ID: &cursor.ID,
	}, nil
}

func mapConversation(record store.Conversation) Conversation {
	return Conversation{
		ID: record.ID, Title: record.Title, Status: record.Status,
		ModelID: record.ModelID, GenerationState: record.GenerationState,
		CreatedAt: record.CreatedAt.Time.UTC(), UpdatedAt: record.UpdatedAt.Time.UTC(),
	}
}

func mapMessage(record store.Message) Message {
	return Message{
		ID: record.ID, ConversationID: record.ConversationID, Role: record.Role,
		Content: record.Content, Status: record.Status, Sequence: record.Sequence,
		CreatedAt: record.CreatedAt.Time.UTC(),
	}
}

func validActor(actor Actor) error {
	if actor.ID == uuid.Nil || (actor.Type != ActorUser && actor.Type != ActorGuest) {
		return ErrInvalid
	}
	return nil
}

func uniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}
