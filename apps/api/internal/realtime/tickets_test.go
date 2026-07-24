package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
)

type memoryTickets struct {
	values map[string]string
}

func (store *memoryTickets) Put(
	_ context.Context, namespace, id, value string, _ time.Duration,
) error {
	store.values[namespace+id] = value
	return nil
}

func (store *memoryTickets) Take(
	_ context.Context, namespace, id string,
) (string, error) {
	key := namespace + id
	value, ok := store.values[key]
	if !ok {
		return "", redisx.ErrNotFound
	}
	delete(store.values, key)
	return value, nil
}

func TestTicketIsSingleUseAndActorBound(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	service := NewTickets(
		&memoryTickets{values: map[string]string{}}, clock.NewFake(now), 30*time.Second,
	)
	actor := conversations.Actor{Type: conversations.ActorUser, ID: uuid.New()}
	ticket, err := service.Issue(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	other := conversations.Actor{Type: conversations.ActorUser, ID: uuid.New()}
	if err := service.Consume(context.Background(), ticket.Value, other); !errors.Is(err, ErrActorMismatch) {
		t.Fatalf("mismatch err = %v", err)
	}
	if err := service.Consume(context.Background(), ticket.Value, actor); !errors.Is(err, ErrTicketInvalid) {
		t.Fatalf("replay err = %v", err)
	}
}

func TestTicketExpires(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	timeSource := clock.NewFake(now)
	service := NewTickets(
		&memoryTickets{values: map[string]string{}}, timeSource, 30*time.Second,
	)
	actor := conversations.Actor{Type: conversations.ActorGuest, ID: uuid.New()}
	ticket, err := service.Issue(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	timeSource.Advance(31 * time.Second)
	if err := service.Consume(context.Background(), ticket.Value, actor); !errors.Is(err, ErrTicketInvalid) {
		t.Fatalf("expired ticket err = %v", err)
	}
}
