//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aikssen/glazz-chat/apps/api/internal/admin"
	"github.com/aikssen/glazz-chat/apps/api/internal/identity/users"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/privacy"
)

func TestM4AdministrationAndPrivacyLifecycle(t *testing.T) {
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	migrations, err := database.NewMigrationRunner(runtimeConfig.Database.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Close(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, config.Database{
		URL: runtimeConfig.Database.URL, MaxConnections: 8, MinConnections: 1,
		MaxLifetime: time.Hour, MaxIdleTime: time.Minute, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	now := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	timeSource := clock.NewFake(now)
	idSource := ids.NewUUIDv7()
	administratorID := uuid.New()
	deletionUserID := uuid.New()
	suffix := uuid.NewString()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO users (id, email, display_name, role)
		VALUES ($1, $2, 'M4 Administrator', 'admin'),
		       ($3, $4, 'Deletion User', 'user')
	`, administratorID, "m4-admin-"+suffix+"@example.com",
		deletionUserID, "m4-delete-"+suffix+"@example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1)`,
			[]uuid.UUID{administratorID, deletionUserID})
	})

	adminService := admin.New(pool, idSource, timeSource)
	settings, err := adminService.ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	maintenance := settingByKey(t, settings, "maintenance.enabled")
	updated, err := adminService.UpdateSetting(
		ctx, administratorID, maintenance.Key, json.RawMessage(`true`),
		maintenance.Version, "m4-setting-update",
	)
	if err != nil || updated.Value != true {
		t.Fatalf("update maintenance = %#v, %v", updated, err)
	}
	if _, err := adminService.UpdateSetting(
		ctx, administratorID, maintenance.Key, json.RawMessage(`false`),
		maintenance.Version, "m4-setting-stale",
	); !errors.Is(err, admin.ErrConflict) {
		t.Fatalf("stale setting update error = %v", err)
	}
	if _, err := adminService.UpdateSetting(
		ctx, administratorID, maintenance.Key, json.RawMessage(`false`),
		updated.Version, "m4-setting-restore",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adminService.UpdateUserRole(
		ctx, administratorID, administratorID, "user", 1, "m4-self-demotion",
	); !errors.Is(err, admin.ErrConflict) {
		t.Fatalf("self demotion error = %v", err)
	}
	auditPage, err := adminService.Audit(ctx, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAudit(auditPage.Items, "setting.updated", maintenance.Key) {
		t.Fatal("setting update was not audited")
	}

	identityID := uuid.New()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO user_identities (
			id, user_id, provider, provider_subject, verified_email
		) VALUES ($1, $2, 'google', $3, $4)
	`, identityID, deletionUserID, "m4-subject-"+suffix,
		"m4-delete-"+suffix+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO auth_sessions (
			id, user_id, family_id, refresh_expires_at, recent_auth_at
		) VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), deletionUserID, uuid.New(), now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO conversations (id, user_id, model_id)
		VALUES ($1, $2, '00000000-0000-7000-8000-000000000101')
	`, uuid.New(), deletionUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO daily_usage (
			actor_type, actor_id, usage_date, messages_used, output_tokens_used
		) VALUES ('user', $1, $2, 2, 300)
	`, deletionUserID, now); err != nil {
		t.Fatal(err)
	}

	privacyService := privacy.New(pool, idSource, timeSource)
	first, err := privacyService.Request(ctx, deletionUserID)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := privacyService.Request(ctx, deletionUserID)
	if err != nil || repeated.ID != first.ID {
		t.Fatalf("repeated deletion = %#v, %v", repeated, err)
	}
	userService := users.New(pool, idSource, timeSource, "terms-1", "privacy-1", nil)
	if _, _, err := userService.ProvisionGoogle(ctx, users.ProvisionInput{
		Profile: users.GoogleProfile{
			Subject: "m4-subject-" + suffix, Email: "m4-delete-" + suffix + "@example.com",
			EmailVerified: true,
		},
		TermsAccepted: true, PrivacyAccepted: true,
	}); !errors.Is(err, users.ErrAccountUnavailable) {
		t.Fatalf("login after deletion request error = %v", err)
	}
	var activeSessions int
	if err := pool.Raw().QueryRow(ctx,
		`SELECT COUNT(*) FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NULL`,
		deletionUserID,
	).Scan(&activeSessions); err != nil || activeSessions != 0 {
		t.Fatalf("active sessions after deletion = %d, %v", activeSessions, err)
	}
	if _, err := privacyService.PurgeDue(ctx, 0, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Queries().FindUserByID(ctx, deletionUserID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("purged user lookup error = %v", err)
	}
	var conversationCount, retainedAggregate int
	if err := pool.Raw().QueryRow(ctx,
		`SELECT COUNT(*) FROM conversations WHERE user_id = $1`, deletionUserID,
	).Scan(&conversationCount); err != nil || conversationCount != 0 {
		t.Fatalf("personal conversations = %d, %v", conversationCount, err)
	}
	if err := pool.Raw().QueryRow(ctx,
		`SELECT COUNT(*) FROM daily_usage WHERE actor_type = 'user' AND actor_id = $1`,
		deletionUserID,
	).Scan(&retainedAggregate); err != nil || retainedAggregate != 1 {
		t.Fatalf("anonymous aggregate count = %d, %v", retainedAggregate, err)
	}
	var jobStatus string
	if err := pool.Raw().QueryRow(ctx,
		`SELECT status FROM account_deletion_jobs WHERE id = $1`, first.ID,
	).Scan(&jobStatus); err != nil || jobStatus != "completed" {
		t.Fatalf("deletion job status = %q, %v", jobStatus, err)
	}

	assertGuestCleanup(t, ctx, pool, privacyService, now)
}

func assertGuestCleanup(
	t *testing.T,
	ctx context.Context,
	pool *database.Pool,
	service *privacy.Service,
	now time.Time,
) {
	t.Helper()
	expiredID, activeID, migratedID := uuid.New(), uuid.New(), uuid.New()
	migratedUserID := uuid.New()
	suffix := uuid.NewString()
	expiredHash := sha256.Sum256([]byte("expired-" + suffix))
	activeHash := sha256.Sum256([]byte("active-" + suffix))
	migratedHash := sha256.Sum256([]byte("migrated-" + suffix))
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO users (id, email, display_name)
		VALUES ($1, $2, 'Migrated Guest')
	`, migratedUserID, "m4-migrated-"+suffix+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO guest_sessions (id, identity_hash, expires_at)
		VALUES ($1, $2, $3),
		       ($4, $5, $3),
		       ($6, $7, $3)
	`, expiredID, expiredHash[:], now.Add(-time.Hour),
		activeID, activeHash[:], migratedID, migratedHash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		UPDATE guest_sessions
		SET migrated_user_id = $1, migrated_at = $2
		WHERE id = $3
	`, migratedUserID, now, migratedID); err != nil {
		t.Fatal(err)
	}
	conversationID := uuid.New()
	userMessageID := uuid.New()
	assistantMessageID := uuid.New()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO conversations (id, guest_session_id, model_id, generation_state)
		VALUES ($1, $2, '00000000-0000-7000-8000-000000000101', 'streaming')
	`, conversationID, activeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, status, sequence)
		VALUES ($1, $2, 'user', 'active', 'complete', 1),
		       ($3, $2, 'assistant', '', 'pending', 2)
	`, userMessageID, conversationID, assistantMessageID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO generations (
			id, conversation_id, user_message_id, assistant_message_id,
			model_id, provider_id, idempotency_key, status
		) VALUES (
			$1, $2, $3, $4,
			'00000000-0000-7000-8000-000000000101',
			'00000000-0000-7000-8000-000000000001',
			'm4-active-generation', 'streaming'
		)
	`, uuid.New(), conversationID, userMessageID, assistantMessageID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id = $1`, migratedUserID)
		_, _ = pool.Raw().Exec(context.Background(), `DELETE FROM guest_sessions WHERE id = ANY($1)`,
			[]uuid.UUID{expiredID, activeID, migratedID})
	})

	if _, err := service.CleanupGuests(ctx); err != nil {
		t.Fatal(err)
	}
	var expiredExists, activeExists, migratedExists bool
	if err := pool.Raw().QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM guest_sessions WHERE id = $1),
		       EXISTS(SELECT 1 FROM guest_sessions WHERE id = $2),
		       EXISTS(SELECT 1 FROM guest_sessions WHERE id = $3)
	`, expiredID, activeID, migratedID).Scan(
		&expiredExists, &activeExists, &migratedExists,
	); err != nil {
		t.Fatal(err)
	}
	if expiredExists || !activeExists || !migratedExists {
		t.Fatalf("guest cleanup states = expired %t, active %t, migrated %t",
			expiredExists, activeExists, migratedExists)
	}
}

func settingByKey(t *testing.T, items []admin.Setting, key string) admin.Setting {
	t.Helper()
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	t.Fatalf("setting %q was not found", key)
	return admin.Setting{}
}

func containsAudit(items []admin.AuditEvent, action, targetID string) bool {
	for _, item := range items {
		if item.Action == action && item.TargetID == targetID {
			return true
		}
	}
	return false
}
