package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
)

type replayStoreStub struct {
	current int64
	events  []string
}

func (store *replayStoreStub) NextSequence(
	context.Context, string, string, time.Duration,
) (int64, error) {
	return 0, nil
}

func (store *replayStoreStub) CurrentSequence(
	context.Context, string, string,
) (int64, error) {
	return store.current, nil
}

func (store *replayStoreStub) AppendReplay(
	context.Context, string, string, int64, string, int64, time.Duration,
) error {
	return nil
}

func (store *replayStoreStub) ReplayAfter(
	context.Context, string, string, int64, int64,
) ([]string, error) {
	return store.events, nil
}

func (store *replayStoreStub) Publish(context.Context, string, string) error {
	return nil
}

func (store *replayStoreStub) Subscribe(context.Context, string) (*redis.PubSub, error) {
	return nil, nil
}

func TestReplayDetectsMissedWindow(t *testing.T) {
	event, err := json.Marshal(Event{Sequence: 5})
	if err != nil {
		t.Fatal(err)
	}
	store := &replayStoreStub{current: 8, events: []string{string(event)}}
	broker := &Broker{store: store}
	actor := conversations.Actor{Type: conversations.ActorUser, ID: uuid.New()}

	result, err := broker.Replay(context.Background(), actor, 1)
	if err != nil || !result.ResyncRequired {
		t.Fatalf("missed replay = %#v, err = %v", result, err)
	}
	result, err = broker.Replay(context.Background(), actor, 4)
	if err != nil || result.ResyncRequired || len(result.Events) != 1 {
		t.Fatalf("contiguous replay = %#v, err = %v", result, err)
	}
	store.events = nil
	result, err = broker.Replay(context.Background(), actor, 7)
	if err != nil || !result.ResyncRequired {
		t.Fatalf("missing latest replay = %#v, err = %v", result, err)
	}
}

func TestEnqueueRejectsFullQueue(t *testing.T) {
	queue := make(chan string, 1)
	if !enqueue(queue, "first") {
		t.Fatal("first enqueue failed")
	}
	if enqueue(queue, "second") {
		t.Fatal("full queue accepted another event")
	}
}
