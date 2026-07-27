//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/chat"
	"github.com/aikssen/glazz-chat/apps/api/internal/conversations"
	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
	"github.com/aikssen/glazz-chat/apps/api/internal/provider"
	"github.com/aikssen/glazz-chat/apps/api/internal/quota"
	"github.com/aikssen/glazz-chat/apps/api/internal/realtime"
)

func TestM3DurableIdempotentChat(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 10, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redisClient, err := redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: "glazz-m3-integration-" + uuid.NewString(),
		HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer redisClient.Close()

	idSource := ids.NewUUIDv7()
	timeSource := clock.UTC{}
	cookieConfig := config.Cookies{
		SigningKey: []byte("01234567890123456789012345678901"), SameSite: "lax",
	}
	guestID, err := idSource.New()
	if err != nil {
		t.Fatal(err)
	}
	identity := []byte(uuid.NewString())
	identityHash := sha256.Sum256(identity)
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO guest_sessions (id, identity_hash, expires_at)
		VALUES ($1, $2, now() + interval '1 day')
	`, guestID, identityHash[:]); err != nil {
		t.Fatal(err)
	}
	modelService := models.New(pool)
	conversationService := conversations.New(pool, modelService, idSource, timeSource)
	actor := conversations.Actor{Type: conversations.ActorGuest, ID: guestID}
	conversation, err := conversationService.Create(ctx, actor, conversations.CreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	quotaService := quota.New(
		pool, redisClient, idSource, timeSource, quota.DefaultPolicy(), cookieConfig.SigningKey,
	)
	fake := provider.NewFake(provider.FakeOptions{Chunks: []string{"hello ", "world"}})
	broker := realtime.NewBroker(redisClient, idSource, timeSource)
	service := chat.New(
		ctx, pool, conversationService, modelService, quotaService, broker,
		chat.Gateways{"fake": fake}, idSource, timeSource,
	)
	payload, _ := json.Marshal(map[string]any{
		"conversationId": conversation.ID, "content": "Say hello.",
	})
	event := realtime.RawEvent{
		Type: "chat.generate", EventID: "evt_integration_01",
		RequestID: "req_integration_01", IdempotencyKey: "idem-integration-0001",
		Payload: payload,
	}
	start, err := service.Prepare(ctx, actor, event)
	if err != nil || start == nil {
		t.Fatalf("prepare = start %v, error %v", start != nil, err)
	}
	start()
	var generation store.Generation
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		generation, err = pool.Queries().GetGenerationByIdempotencyKey(
			ctx, store.GetGenerationByIdempotencyKeyParams{
				ConversationID: conversation.ID, IdempotencyKey: event.IdempotencyKey,
			},
		)
		if err == nil && generation.Status == "completed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || generation.Status != "completed" || generation.OutputTokens != 6 {
		t.Fatalf("generation = %#v, error %v", generation, err)
	}
	messages, err := pool.Queries().ListConversationMessages(
		ctx, store.ListConversationMessagesParams{
			ConversationID: conversation.ID, PageSize: 10,
		},
	)
	if err != nil || len(messages) != 2 || messages[0].Content != "hello world" ||
		messages[0].ModelID == nil || *messages[0].ModelID != generation.ModelID ||
		messages[0].ModelName == nil || *messages[0].ModelName == "" {
		t.Fatalf("messages = %#v, error %v", messages, err)
	}
	replayStart, err := service.Prepare(ctx, actor, event)
	if err != nil || replayStart != nil || fake.Calls() != 1 {
		t.Fatalf("duplicate = start %v, calls %d, error %v", replayStart != nil, fake.Calls(), err)
	}
}
