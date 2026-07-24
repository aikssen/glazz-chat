//go:build integration

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/config"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/settings"
)

func TestPhase6AdministrationAcceptance(t *testing.T) {
	ctx := context.Background()
	runtimeConfig, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	pool, redisClient := isolatedAdministrationStore(t, ctx, runtimeConfig)
	now := time.Date(2026, 7, 24, 18, 0, 0, 0, time.UTC)
	timeSource := clock.NewFake(now)
	runtimeSettings := settings.New(pool, redisClient)
	service := New(pool, ids.NewUUIDv7(), timeSource).WithSettingsInvalidator(runtimeSettings)

	adminA, adminB := uuid.New(), uuid.New()
	deletionPending := uuid.New()
	suffix := uuid.NewString()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO users (id, email, display_name, role)
		VALUES ($1, $2, 'Admin A', 'admin'),
		       ($3, $4, 'Admin B', 'admin'),
		       ($5, $6, 'Pending User', 'user')
	`, adminA, "phase6-a-"+suffix+"@example.com",
		adminB, "phase6-b-"+suffix+"@example.com",
		deletionPending, "phase6-pending-"+suffix+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx,
		`UPDATE users SET status = 'deletion_pending' WHERE id = $1`, deletionPending,
	); err != nil {
		t.Fatal(err)
	}

	assertSettingsAcceptance(t, ctx, service, runtimeSettings, adminA)
	assertModelAcceptance(t, ctx, pool, service, adminA)
	assertRoleAcceptance(t, ctx, pool, service, adminA, adminB, deletionPending)
	assertUsageAndAuditAcceptance(t, ctx, pool, service, adminA, now)
	assertUserPagination(t, ctx, pool, service, suffix)
}

func assertSettingsAcceptance(
	t *testing.T,
	ctx context.Context,
	service *Service,
	runtimeSettings *settings.Service,
	actorID uuid.UUID,
) {
	t.Helper()
	before, err := runtimeSettings.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settingsList, err := service.ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	maintenance := integrationSettingByKey(t, settingsList, "maintenance.enabled")
	updated, err := service.UpdateSetting(
		ctx, actorID, maintenance.Key, json.RawMessage(`true`),
		maintenance.Version, "phase6-maintenance",
	)
	if err != nil || updated.Value != true {
		t.Fatalf("maintenance update = %#v, %v", updated, err)
	}
	after, err := runtimeSettings.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Maintenance || !after.Maintenance {
		t.Fatalf("cached maintenance before=%t after=%t", before.Maintenance, after.Maintenance)
	}
	if _, err := service.UpdateSetting(
		ctx, actorID, maintenance.Key, json.RawMessage(`false`),
		maintenance.Version, "phase6-stale",
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update error = %v", err)
	}
	if _, err := service.UpdateSetting(
		ctx, actorID, "chat.summary_model_id",
		json.RawMessage(`"`+uuid.NewString()+`"`), 1, "phase6-invalid-summary",
	); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid summary model error = %v", err)
	}

	prompt := integrationSettingByKey(t, settingsList, "chat.system_prompt")
	const privatePrompt = "phase6 private system policy"
	if _, err := service.UpdateSetting(
		ctx, actorID, prompt.Key, json.RawMessage(`"`+privatePrompt+`"`),
		prompt.Version, "phase6-prompt",
	); err != nil {
		t.Fatal(err)
	}
	var storedAudit string
	if err := service.database.Raw().QueryRow(ctx, `
		SELECT after_value::text
		FROM admin_audit_log
		WHERE request_id = 'phase6-prompt'
	`).Scan(&storedAudit); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(storedAudit, privatePrompt) || !strings.Contains(storedAudit, "[REDACTED]") {
		t.Fatalf("stored prompt audit was not redacted: %s", storedAudit)
	}
}

func assertModelAcceptance(
	t *testing.T,
	ctx context.Context,
	pool *database.Pool,
	service *Service,
	actorID uuid.UUID,
) {
	t.Helper()
	modelID := uuid.New()
	unavailableID := uuid.New()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO models (
			id, slug, name, context_window, max_output_tokens, capabilities,
			enabled, available, supported, audience, default_for, sort_order
		) VALUES (
			$1, $2, 'Phase 6 model', 8192, 2048, '{"chatCompletions":true}',
			true, true, true, ARRAY['user']::text[], '{}'::text[], 10
		), (
			$3, $4, 'Unavailable model', 8192, 2048, '{"chatCompletions":true}',
			false, false, true, ARRAY['user']::text[], '{}'::text[], 11
		)
	`, modelID, "phase6-"+modelID.String(), unavailableID, "phase6-"+unavailableID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO provider_models (provider_id, model_id, provider_model_id)
		VALUES ('00000000-0000-7000-8000-000000000001', $1, $2)
	`, modelID, "phase6-"+modelID.String()); err != nil {
		t.Fatal(err)
	}
	defaultFor := []string{"user"}
	updated, err := service.UpdateModel(
		ctx, actorID, modelID, ModelUpdate{DefaultFor: &defaultFor}, 1, "phase6-default",
	)
	if err != nil || !slicesContain(updated.DefaultFor, "user") {
		t.Fatalf("replace default = %#v, %v", updated, err)
	}
	var originalDefaults []string
	if err := pool.Raw().QueryRow(ctx, `
		SELECT default_for FROM models
		WHERE id = '00000000-0000-7000-8000-000000000101'
	`).Scan(&originalDefaults); err != nil {
		t.Fatal(err)
	}
	if slicesContain(originalDefaults, "user") || !slicesContain(originalDefaults, "guest") {
		t.Fatalf("original defaults after replacement = %v", originalDefaults)
	}
	disabled := false
	if _, err := service.UpdateModel(
		ctx, actorID, modelID, ModelUpdate{Enabled: &disabled},
		updated.Version, "phase6-remove-only-default",
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("remove only user default error = %v", err)
	}
	enabled := true
	if _, err := service.UpdateModel(
		ctx, actorID, unavailableID, ModelUpdate{Enabled: &enabled},
		1, "phase6-enable-unavailable",
	); !errors.Is(err, ErrInvalid) {
		t.Fatalf("enable unavailable model error = %v", err)
	}
}

func assertRoleAcceptance(
	t *testing.T,
	ctx context.Context,
	pool *database.Pool,
	service *Service,
	adminA uuid.UUID,
	adminB uuid.UUID,
	deletionPending uuid.UUID,
) {
	t.Helper()
	if _, err := service.UpdateUserRole(
		ctx, adminA, adminA, "user", 1, "phase6-self-demotion",
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("self demotion error = %v", err)
	}
	if _, err := service.UpdateUserRole(
		ctx, adminA, deletionPending, "admin", 1, "phase6-pending-promotion",
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("pending promotion error = %v", err)
	}

	var successes atomic.Int32
	var conflicts atomic.Int32
	var wait sync.WaitGroup
	for _, change := range []struct {
		actor  uuid.UUID
		target uuid.UUID
	}{
		{actor: adminA, target: adminB},
		{actor: adminB, target: adminA},
	} {
		change := change
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.UpdateUserRole(
				ctx, change.actor, change.target, "user", 1,
				"phase6-concurrent-role-"+change.target.String(),
			)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrConflict):
				conflicts.Add(1)
			default:
				t.Errorf("concurrent role update error = %v", err)
			}
		}()
	}
	wait.Wait()
	if successes.Load() != 1 || conflicts.Load() != 1 {
		t.Fatalf("role outcomes = %d success, %d conflict", successes.Load(), conflicts.Load())
	}
	var administrators int
	if err := pool.Raw().QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active'`,
	).Scan(&administrators); err != nil {
		t.Fatal(err)
	}
	if administrators != 1 {
		t.Fatalf("active administrators = %d, want 1", administrators)
	}
}

func assertUsageAndAuditAcceptance(
	t *testing.T,
	ctx context.Context,
	pool *database.Pool,
	service *Service,
	actorID uuid.UUID,
	now time.Time,
) {
	t.Helper()
	userID := uuid.New()
	conversationID := uuid.New()
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO users (id, email, display_name)
		VALUES ($1, $2, 'Usage User')
	`, userID, "phase6-usage-"+uuid.NewString()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Raw().Exec(ctx, `
		INSERT INTO conversations (id, user_id, model_id)
		VALUES ($1, $2, '00000000-0000-7000-8000-000000000101')
	`, conversationID, userID); err != nil {
		t.Fatal(err)
	}
	for index, fixture := range []struct {
		status    string
		errorCode *string
		latency   time.Duration
		input     int
		output    int
	}{
		{status: "completed", latency: 2 * time.Second, input: 10, output: 20},
		{status: "failed", errorCode: stringPointer("provider_timeout"), latency: 4 * time.Second, input: 5, output: 7},
	} {
		generationID := uuid.New()
		userMessageID := uuid.New()
		assistantMessageID := uuid.New()
		completedAt := now.Add(time.Duration(index) * time.Second)
		if _, err := pool.Raw().Exec(ctx, `
			INSERT INTO messages (id, conversation_id, role, content, status, sequence)
			VALUES ($1, $3, 'user', 'private user message', 'complete', $4),
			       ($2, $3, 'assistant', 'private response', $5, $6)
		`, userMessageID, assistantMessageID, conversationID, index*2+1,
			map[string]string{"completed": "complete", "failed": "failed"}[fixture.status],
			index*2+2); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Raw().Exec(ctx, `
			INSERT INTO generations (
				id, conversation_id, user_message_id, assistant_message_id,
				model_id, provider_id, idempotency_key, status, finish_reason,
				error_code, input_tokens, output_tokens, accepted_at, completed_at
			) VALUES (
				$1, $2, $3, $4,
				'00000000-0000-7000-8000-000000000101',
				'00000000-0000-7000-8000-000000000001',
				$5, $6, $7, $8, $9, $10, $11, $12
			)
		`, generationID, conversationID, userMessageID, assistantMessageID,
			fmt.Sprintf("phase6-generation-%02d", index),
			fixture.status, map[string]string{"completed": "stop", "failed": "error"}[fixture.status],
			fixture.errorCode, fixture.input, fixture.output,
			completedAt.Add(-fixture.latency), completedAt); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Raw().Exec(ctx, `
			INSERT INTO usage_ledger (
				id, generation_id, actor_type, actor_id, provider_id, model_id,
				input_tokens, output_tokens, occurred_at
			) VALUES (
				$1, $2, 'user', $3,
				'00000000-0000-7000-8000-000000000001',
				'00000000-0000-7000-8000-000000000101',
				$4, $5, $6
			)
		`, uuid.New(), generationID, userID, fixture.input, fixture.output, completedAt); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := service.Usage(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if usage.Generations != 2 || usage.FailedGenerations != 1 ||
		usage.InputTokens != 15 || usage.OutputTokens != 27 ||
		usage.AverageLatencyMs != 3000 || usage.P95LatencyMs < 3800 ||
		len(usage.Errors) != 1 || usage.Errors[0].Code != "provider_timeout" ||
		usage.Errors[0].Count != 1 {
		t.Fatalf("usage aggregate = %#v", usage)
	}

	first, err := service.Audit(ctx, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("first audit page = %#v", first)
	}
	second, err := service.Audit(ctx, first.NextCursor, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("second audit page = %#v", second)
	}
	for _, page := range [][]AuditEvent{first.Items, second.Items} {
		encoded, _ := json.Marshal(page)
		if strings.Contains(string(encoded), "private user message") ||
			strings.Contains(string(encoded), "private response") {
			t.Fatalf("audit leaked chat content: %s", encoded)
		}
	}
	_ = actorID
}

func assertUserPagination(
	t *testing.T,
	ctx context.Context,
	pool *database.Pool,
	service *Service,
	suffix string,
) {
	t.Helper()
	for index := range 3 {
		if _, err := pool.Raw().Exec(ctx, `
			INSERT INTO users (id, email, display_name, created_at)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), fmt.Sprintf("search-%d-%s@example.com", index, suffix),
			"Phase6 Search", time.Now().UTC().Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.ListUsers(ctx, suffix, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("first user page = %#v", first)
	}
	second, err := service.ListUsers(ctx, suffix, first.NextCursor, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("second user page = %#v", second)
	}
	if _, err := service.ListUsers(ctx, "", "invalid", 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func isolatedAdministrationStore(
	t *testing.T,
	ctx context.Context,
	runtimeConfig config.Config,
) (*database.Pool, *redisx.Client) {
	t.Helper()
	base, err := database.Open(ctx, runtimeConfig.Database)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := "phase6_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := base.Raw().Exec(ctx, `CREATE DATABASE `+databaseName); err != nil {
		base.Close()
		t.Fatal(err)
	}
	var pool *database.Pool
	var redisClient *redisx.Client
	t.Cleanup(func() {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		if pool != nil {
			pool.Close()
		}
		_, _ = base.Raw().Exec(
			context.Background(), `DROP DATABASE IF EXISTS `+databaseName+` WITH (FORCE)`,
		)
		base.Close()
	})

	parsed, err := url.Parse(runtimeConfig.Database.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + databaseName
	schemaConfig := runtimeConfig.Database
	schemaConfig.URL = parsed.String()
	migrations, err := database.NewMigrationRunner(schemaConfig.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Close(); err != nil {
		t.Fatal(err)
	}
	pool, err = database.Open(ctx, schemaConfig)
	if err != nil {
		t.Fatal(err)
	}
	redisClient, err = redisx.Open(ctx, config.Redis{
		URL: runtimeConfig.Redis.URL, Prefix: databaseName, HealthTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return pool, redisClient
}

func integrationSettingByKey(t *testing.T, values []Setting, key string) Setting {
	t.Helper()
	for _, value := range values {
		if value.Key == key {
			return value
		}
	}
	t.Fatalf("setting %q not found", key)
	return Setting{}
}

func stringPointer(value string) *string {
	return &value
}
