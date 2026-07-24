package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
)

var (
	ErrTicketInvalid = errors.New("WebSocket ticket is invalid")
	ErrActorMismatch = errors.New("WebSocket ticket actor mismatch")
)

type Ticket struct {
	Value     string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ticketPayload struct {
	ActorType conversations.ActorType `json:"actorType"`
	ActorID   string                  `json:"actorId"`
	ExpiresAt time.Time               `json:"expiresAt"`
}

type TicketStore interface {
	Put(context.Context, string, string, string, time.Duration) error
	Take(context.Context, string, string) (string, error)
}

type Tickets struct {
	store TicketStore
	clock clock.Clock
	ttl   time.Duration
}

func NewTickets(store TicketStore, timeSource clock.Clock, ttl time.Duration) *Tickets {
	return &Tickets{store: store, clock: timeSource, ttl: ttl}
}

func (service *Tickets) Issue(
	ctx context.Context,
	actor conversations.Actor,
) (Ticket, error) {
	if actor.ID == uuid.Nil ||
		(actor.Type != conversations.ActorUser && actor.Type != conversations.ActorGuest) ||
		service.ttl <= 0 {
		return Ticket{}, ErrTicketInvalid
	}
	value, err := ids.SecureToken(32)
	if err != nil {
		return Ticket{}, err
	}
	expiresAt := service.clock.Now().UTC().Add(service.ttl)
	payload, err := json.Marshal(ticketPayload{
		ActorType: actor.Type, ActorID: actor.ID.String(), ExpiresAt: expiresAt,
	})
	if err != nil {
		return Ticket{}, err
	}
	if err := service.store.Put(
		ctx, "ws-ticket", ticketID(value), string(payload), service.ttl,
	); err != nil {
		return Ticket{}, fmt.Errorf("store WebSocket ticket: %w", err)
	}
	return Ticket{Value: value, ExpiresAt: expiresAt}, nil
}

func (service *Tickets) Consume(
	ctx context.Context,
	value string,
	actor conversations.Actor,
) error {
	if len(value) < 32 {
		return ErrTicketInvalid
	}
	raw, err := service.store.Take(ctx, "ws-ticket", ticketID(value))
	if errors.Is(err, redisx.ErrNotFound) {
		return ErrTicketInvalid
	}
	if err != nil {
		return fmt.Errorf("consume WebSocket ticket: %w", err)
	}
	var payload ticketPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ErrTicketInvalid
	}
	if !payload.ExpiresAt.After(service.clock.Now().UTC()) {
		return ErrTicketInvalid
	}
	if payload.ActorType != actor.Type || payload.ActorID != actor.ID.String() {
		return ErrActorMismatch
	}
	return nil
}

func ticketID(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
