//go:build integration

package integration_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/guests"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/sessions"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/tokens"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/outbox"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
	"github.com/aikssen/glazz-chat/apps/api/internal/quota"
)

func TestM2IdentityGuestQuotaAndOutbox(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	migrations, err := database.NewMigrationRunner(runtimeConfig.Database.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Close(); err != nil {
		t.Fatal(err)
	}

	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 10, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redisClient, err := redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: "glazz-m2-integration-" + uuid.NewString(), HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer redisClient.Close()

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	timeSource := clock.NewFake(now)
	idSource := ids.NewUUIDv7()
	authConfig := config.Auth{
		Issuer: "https://api.example", Audience: "glazz-web", ActiveKeyID: "test-1",
		AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 24 * time.Hour,
	}
	ring, err := tokens.NewEphemeral(authConfig, timeSource)
	if err != nil {
		t.Fatal(err)
	}
	testID := uuid.NewString()
	adminEmail := "person-" + testID + "@example.com"
	userService := users.New(pool, idSource, timeSource, "terms-1", "privacy-1", map[string]struct{}{
		adminEmail: {},
	})
	user, created, err := userService.ProvisionGoogle(ctx, users.ProvisionInput{
		Profile: users.GoogleProfile{
			Subject: "google-subject-" + testID, Email: adminEmail,
			EmailVerified: true, DisplayName: "Person",
		},
		TermsAccepted: true, PrivacyAccepted: true,
	})
	if err != nil || !created || user.Role != "admin" {
		t.Fatalf("first provision = created %v, error %v", created, err)
	}
	replayed, created, err := userService.ProvisionGoogle(ctx, users.ProvisionInput{
		Profile: users.GoogleProfile{
			Subject: "google-subject-" + testID, Email: adminEmail,
			EmailVerified: true, DisplayName: "Person",
		},
		TermsAccepted: true, PrivacyAccepted: true,
	})
	if err != nil || created || replayed.ID != user.ID {
		t.Fatalf("replayed provision = %#v, created %v, error %v", replayed, created, err)
	}

	sessionService := sessions.New(pool, idSource, timeSource, ring, 24*time.Hour)
	credentials, err := sessionService.Create(ctx, user.ID, "integration")
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var reuses atomic.Int32
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, rotateErr := sessionService.Rotate(ctx, credentials.RefreshToken)
			if rotateErr == nil {
				successes.Add(1)
			}
			if errors.Is(rotateErr, sessions.ErrRefreshReuse) {
				reuses.Add(1)
			}
		}()
	}
	wait.Wait()
	if successes.Load() != 1 || reuses.Load() != 1 {
		t.Fatalf("refresh outcomes = %d success, %d reuse", successes.Load(), reuses.Load())
	}

	cookies := config.Cookies{
		SigningKey: []byte("01234567890123456789012345678901"), SameSite: "lax",
	}
	guestService := guests.New(pool, idSource, timeSource, cookies, time.Hour)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/guest-sessions", nil)
	_, created, err = guestService.CreateOrResume(ctx, request, recorder)
	if err != nil || !created {
		t.Fatalf("create guest = %v, %v", created, err)
	}
	response := recorder.Result()
	request = httptest.NewRequest("GET", "/api/v1/guest-sessions/current", nil)
	request.AddCookie(response.Cookies()[0])
	guestID, err := guestService.ID(ctx, request)
	if err != nil {
		t.Fatal(err)
	}

	quotaService := quota.New(
		pool, redisClient, idSource, timeSource, quota.DefaultPolicy(), cookies.SigningKey,
	)
	reservation, err := quotaService.Reserve(ctx, quota.Actor{
		Type: quota.Guest, ID: *guestID, IP: netip.MustParseAddr("203.0.113.8"),
	}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if err := quotaService.Settle(ctx, reservation, 125); err != nil {
		t.Fatal(err)
	}
	allowance, err := guestService.Current(ctx, request)
	if err != nil || allowance.MessagesUsed != 1 || allowance.OutputTokensUsed != 125 {
		t.Fatalf("settled allowance = %#v, %v", allowance, err)
	}
	conversationID, _ := idSource.New()
	if _, err := pool.Raw().Exec(
		ctx,
		`INSERT INTO conversations (id, guest_session_id, model_id)
		 VALUES ($1, $2, '00000000-0000-7000-8000-000000000101')`,
		conversationID,
		*guestID,
	); err != nil {
		t.Fatal(err)
	}
	migrationInput := users.ProvisionInput{
		Profile: users.GoogleProfile{
			Subject: "google-migrated-" + testID, Email: "migrated-" + testID + "@example.com",
			EmailVerified: true, DisplayName: "Migrated",
		},
		TermsAccepted: true, PrivacyAccepted: true, GuestID: guestID,
	}
	type migrationResult struct {
		user    store.User
		created bool
		err     error
	}
	migrationResults := make(chan migrationResult, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			migratedUser, migratedCreated, migrateErr := userService.ProvisionGoogle(ctx, migrationInput)
			migrationResults <- migrationResult{
				user: migratedUser, created: migratedCreated, err: migrateErr,
			}
		}()
	}
	wait.Wait()
	close(migrationResults)
	var migratedUser store.User
	migrationCreatedCount := 0
	for result := range migrationResults {
		if result.err != nil {
			t.Fatalf("concurrent guest migration error = %v", result.err)
		}
		if migratedUser.ID != uuid.Nil && migratedUser.ID != result.user.ID {
			t.Fatalf("concurrent migration created different users: %s and %s", migratedUser.ID, result.user.ID)
		}
		migratedUser = result.user
		if result.created {
			migrationCreatedCount++
		}
	}
	if migrationCreatedCount != 1 {
		t.Fatalf("concurrent guest migration created %d users, want 1", migrationCreatedCount)
	}
	var conversationOwner uuid.UUID
	if err := pool.Raw().QueryRow(
		ctx,
		`SELECT user_id FROM conversations WHERE id = $1 AND guest_session_id IS NULL`,
		conversationID,
	).Scan(&conversationOwner); err != nil || conversationOwner != migratedUser.ID {
		t.Fatalf("migrated conversation owner = %s, error %v", conversationOwner, err)
	}
	if _, err := guestService.Current(ctx, request); !errors.Is(err, guests.ErrGuestUnauthenticated) {
		t.Fatalf("migrated guest remains active: %v", err)
	}

	expiredRecorder := httptest.NewRecorder()
	expiredRequest := httptest.NewRequest("POST", "/api/v1/guest-sessions", nil)
	if _, _, err := guestService.CreateOrResume(ctx, expiredRequest, expiredRecorder); err != nil {
		t.Fatal(err)
	}
	expiredRequest = httptest.NewRequest("GET", "/api/v1/guest-sessions/current", nil)
	expiredRequest.AddCookie(expiredRecorder.Result().Cookies()[0])
	timeSource.Advance(2 * time.Hour)
	if _, err := guestService.Current(ctx, expiredRequest); !errors.Is(err, guests.ErrGuestUnauthenticated) {
		t.Fatalf("expired guest remains active: %v", err)
	}

	eventID, _ := idSource.New()
	if err := pool.Queries().EnqueueOutboxEvent(ctx, store.EnqueueOutboxEventParams{
		ID: eventID, EventType: "integration.event", Payload: []byte(`{"safe":true}`),
		IdempotencyKey: "integration-" + eventID.String(),
		AvailableAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	var handled atomic.Int32
	handler := outbox.HandlerFunc(func(context.Context, outbox.Event) error {
		handled.Add(1)
		return nil
	})
	runner, err := outbox.New(
		pool, timeSource, testLogger(), map[string]outbox.Handler{"integration.event": handler},
		outbox.Options{
			WorkerID: "worker-1", BatchSize: 10, MaxAttempts: 3,
			LockTTL: time.Minute, PollEvery: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	runnerTwo, err := outbox.New(
		pool, timeSource, testLogger(), map[string]outbox.Handler{"integration.event": handler},
		outbox.Options{
			WorkerID: "worker-2", BatchSize: 10, MaxAttempts: 3,
			LockTTL: time.Minute, PollEvery: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan int, 2)
	failures := make(chan error, 2)
	for _, candidate := range []*outbox.Runner{runner, runnerTwo} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			processed, runErr := candidate.RunOnce(ctx)
			results <- processed
			failures <- runErr
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	total := 0
	for processed := range results {
		total += processed
	}
	for runErr := range failures {
		if runErr != nil {
			t.Fatal(runErr)
		}
	}
	if total != 1 {
		t.Fatalf("concurrent outbox claims = %d, want 1", total)
	}
	if processed, err := runner.RunOnce(ctx); err != nil || processed != 0 || handled.Load() != 1 {
		t.Fatalf("outbox replay = processed %d, handled %d, error %v", processed, handled.Load(), err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
