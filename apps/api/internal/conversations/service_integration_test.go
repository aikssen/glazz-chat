//go:build integration

package conversations

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/models"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

func TestConversationOwnershipPaginationStateAndIdempotency(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 6, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	userID := uuid.New()
	otherUserID := uuid.New()
	guestID := uuid.New()
	identityHash := sha256.Sum256([]byte(guestID.String()))
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, userID, otherUserID)
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM guest_sessions WHERE id = $1`, guestID)
	})
	for id, email := range map[uuid.UUID]string{
		userID:      "phase5-owner-" + userID.String() + "@example.test",
		otherUserID: "phase5-other-" + otherUserID.String() + "@example.test",
	} {
		if _, err := pool.Raw().Exec(ctx, `
			INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'Phase 5')
		`, id, email); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO guest_sessions (id, identity_hash, expires_at)
		VALUES ($1, $2, now() + interval '1 day')
	`, guestID, identityHash[:]); err != nil {
		t.Fatal(err)
	}

	timeSource := clock.NewFake(time.Date(2026, time.July, 24, 15, 0, 0, 0, time.UTC))
	service := New(pool, models.New(pool), ids.NewUUIDv7(), timeSource)
	owner := Actor{Type: ActorUser, ID: userID}
	other := Actor{Type: ActorUser, ID: otherUserID}
	guest := Actor{Type: ActorGuest, ID: guestID}

	alpha := createConversation(t, ctx, service, owner, "Alpha project", "create-alpha-0001")
	duplicate, err := service.Create(ctx, owner, CreateInput{
		Title: "Ignored replay title", IdempotencyKey: "create-alpha-0001",
	})
	if err != nil || duplicate.ID != alpha.ID || duplicate.Title != alpha.Title {
		t.Fatalf("idempotent create = %#v, err = %v", duplicate, err)
	}
	timeSource.Advance(time.Second)
	beta := createConversation(t, ctx, service, owner, "Beta project", "create-beta-00001")
	timeSource.Advance(time.Second)
	gamma := createConversation(t, ctx, service, owner, "Gamma archive", "create-gamma-0001")

	archived := true
	if _, err := service.Update(ctx, owner, gamma.ID, UpdateInput{Archived: &archived}); err != nil {
		t.Fatal(err)
	}
	active, err := service.List(ctx, owner, ListInput{Limit: 10})
	if err != nil || containsConversation(active.Items, gamma.ID) {
		t.Fatalf("active list = %#v, err = %v", active, err)
	}
	all, err := service.List(ctx, owner, ListInput{Limit: 10, IncludeArchived: true})
	if err != nil || !containsConversation(all.Items, gamma.ID) {
		t.Fatalf("archived list = %#v, err = %v", all, err)
	}
	search, err := service.List(ctx, owner, ListInput{Limit: 10, Search: "Alpha"})
	if err != nil || len(search.Items) != 1 || search.Items[0].ID != alpha.ID {
		t.Fatalf("search page = %#v, err = %v", search, err)
	}

	firstPage, err := service.List(ctx, owner, ListInput{Limit: 1})
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first page = %#v, err = %v", firstPage, err)
	}
	timeSource.Advance(time.Second)
	newest := createConversation(t, ctx, service, owner, "Newest project", "create-newest-001")
	secondPage, err := service.List(ctx, owner, ListInput{
		Limit: 1, Cursor: firstPage.NextCursor,
	})
	if err != nil || len(secondPage.Items) != 1 ||
		secondPage.Items[0].ID == firstPage.Items[0].ID ||
		secondPage.Items[0].ID == newest.ID {
		t.Fatalf("stable second page = %#v, err = %v", secondPage, err)
	}

	if _, err := service.Get(ctx, other, alpha.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user get err = %v", err)
	}
	title := "Stolen"
	if _, err := service.Update(
		ctx, other, alpha.ID, UpdateInput{Title: &title},
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user update err = %v", err)
	}
	if err := service.Delete(
		ctx, other, alpha.ID, "delete-cross-user",
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user delete err = %v", err)
	}

	if _, err := pool.Queries().SetConversationGenerationState(
		ctx, store.SetConversationGenerationStateParams{
			State: "streaming", NowAt: timestamp(timeSource.Now()), ID: beta.ID,
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(
		ctx, owner, beta.ID, UpdateInput{ModelID: &beta.ModelID},
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("active model change err = %v", err)
	}
	if err := service.Delete(
		ctx, owner, beta.ID, "delete-active-0001",
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("active delete err = %v", err)
	}
	if _, err := pool.Queries().SetConversationGenerationState(
		ctx, store.SetConversationGenerationStateParams{
			State: "idle", NowAt: timestamp(timeSource.Now()), ID: beta.ID,
		},
	); err != nil {
		t.Fatal(err)
	}

	for sequence := int32(1); sequence <= 3; sequence++ {
		if _, err := pool.Queries().CreateMessage(ctx, store.CreateMessageParams{
			ID: uuid.New(), ConversationID: beta.ID, Role: "user",
			Content: fmt.Sprintf("message-%d", sequence), Status: "complete",
			Sequence: sequence, NowAt: timestamp(timeSource.Now()),
		}); err != nil {
			t.Fatal(err)
		}
	}
	messages, err := service.Messages(ctx, owner, beta.ID, 2, nil)
	if err != nil || len(messages.Items) != 2 || messages.Items[0].Sequence != 3 ||
		messages.Items[1].Sequence != 2 || messages.NextCursor != "2" {
		t.Fatalf("message first page = %#v, err = %v", messages, err)
	}
	before := int32(2)
	messages, err = service.Messages(ctx, owner, beta.ID, 2, &before)
	if err != nil || len(messages.Items) != 1 || messages.Items[0].Sequence != 1 {
		t.Fatalf("message second page = %#v, err = %v", messages, err)
	}
	if _, err := service.Messages(ctx, other, beta.ID, 2, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user messages err = %v", err)
	}

	guestConversation := createConversation(
		t, ctx, service, guest, "Guest project", "create-guest-0001",
	)
	guestReplay, err := service.Create(ctx, guest, CreateInput{
		Title: "Ignored", IdempotencyKey: "create-guest-0001",
	})
	if err != nil || guestReplay.ID != guestConversation.ID {
		t.Fatalf("guest replay = %#v, err = %v", guestReplay, err)
	}
	if _, err := service.Create(ctx, guest, CreateInput{
		Title: "Second guest", IdempotencyKey: "create-guest-0002",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("second guest conversation err = %v", err)
	}
	if _, err := service.Update(
		ctx, guest, guestConversation.ID, UpdateInput{Archived: &archived},
	); !errors.Is(err, ErrGuestScope) {
		t.Fatalf("guest archive err = %v", err)
	}

	deleteKey := "delete-alpha-0001"
	if err := service.Delete(ctx, owner, alpha.ID, deleteKey); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, owner, alpha.ID, deleteKey); err != nil {
		t.Fatalf("idempotent delete err = %v", err)
	}
	if err := service.Delete(
		ctx, owner, alpha.ID, "delete-alpha-other",
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("different delete key err = %v", err)
	}
}

func TestConversationSchemaRequiresExactlyOneOwner(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, runtimeConfig.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	_, err = pool.Raw().Exec(ctx, `
		INSERT INTO conversations (id, title, model_id)
		VALUES ($1, 'Invalid owner', '00000000-0000-7000-8000-000000000101')
	`, uuid.New())
	if err == nil {
		t.Fatal("conversation without an owner satisfied the database constraint")
	}
}

func createConversation(
	t *testing.T,
	ctx context.Context,
	service *Service,
	actor Actor,
	title string,
	idempotencyKey string,
) Conversation {
	t.Helper()
	conversation, err := service.Create(ctx, actor, CreateInput{
		Title: title, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	return conversation
}

func containsConversation(items []Conversation, id uuid.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
