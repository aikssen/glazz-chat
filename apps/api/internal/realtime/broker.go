package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
)

const (
	ProtocolVersion = 1
	replayLimit     = 256
	replayTTL       = 10 * time.Minute
)

type Event struct {
	Version    int       `json:"version"`
	Type       string    `json:"type"`
	EventID    string    `json:"eventId"`
	RequestID  string    `json:"requestId"`
	Sequence   int64     `json:"sequence"`
	OccurredAt time.Time `json:"occurredAt"`
	Payload    any       `json:"payload"`
}

type RawEvent struct {
	Version        int             `json:"version"`
	Type           string          `json:"type"`
	EventID        string          `json:"eventId"`
	RequestID      string          `json:"requestId"`
	IdempotencyKey string          `json:"idempotencyKey"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Payload        json.RawMessage `json:"payload"`
}

type ReplayStore interface {
	NextSequence(context.Context, string, string, time.Duration) (int64, error)
	AppendReplay(context.Context, string, string, int64, string, int64, time.Duration) error
	ReplayAfter(context.Context, string, string, int64, int64) ([]string, error)
	Publish(context.Context, string, string) error
	Subscribe(context.Context, string) (*redis.PubSub, error)
}

type Broker struct {
	store ReplayStore
	ids   ids.Source
	clock clock.Clock
}

func NewBroker(store ReplayStore, idSource ids.Source, timeSource clock.Clock) *Broker {
	return &Broker{store: store, ids: idSource, clock: timeSource}
}

func (broker *Broker) Emit(
	ctx context.Context,
	actor conversations.Actor,
	eventType, requestID string,
	payload any,
) (Event, error) {
	actorKey := actor.TypeString() + ":" + actor.ID.String()
	sequence, err := broker.store.NextSequence(ctx, "actor", actorKey, replayTTL)
	if err != nil {
		return Event{}, err
	}
	eventID, err := broker.ids.New()
	if err != nil {
		return Event{}, err
	}
	event := Event{
		Version: ProtocolVersion, Type: eventType, EventID: "evt_" + eventID.String(),
		RequestID: requestID, Sequence: sequence, OccurredAt: broker.clock.Now().UTC(),
		Payload: payload,
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	if err := broker.store.AppendReplay(
		ctx, "actor", actorKey, sequence, string(encoded), replayLimit, replayTTL,
	); err != nil {
		return Event{}, fmt.Errorf("append realtime replay: %w", err)
	}
	if err := broker.store.Publish(ctx, topic(actor), string(encoded)); err != nil {
		return Event{}, fmt.Errorf("publish realtime event: %w", err)
	}
	return event, nil
}

func (broker *Broker) Replay(
	ctx context.Context,
	actor conversations.Actor,
	after int64,
) ([]string, error) {
	return broker.store.ReplayAfter(
		ctx, "actor", actor.TypeString()+":"+actor.ID.String(), after, replayLimit,
	)
}

func (broker *Broker) Subscribe(
	ctx context.Context,
	actor conversations.Actor,
) (*redis.PubSub, error) {
	return broker.store.Subscribe(ctx, topic(actor))
}

func topic(actor conversations.Actor) string {
	return "actor:" + actor.TypeString() + ":" + actor.ID.String()
}

func NewRequestID(id uuid.UUID) string {
	return "req_" + id.String()
}
