-- name: Ping :one
SELECT 1::integer;

-- name: EnqueueOutboxEvent :exec
INSERT INTO outbox_events (id, event_type, payload, idempotency_key, available_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (idempotency_key) DO NOTHING;

-- name: GetOutboxEventByIdempotencyKey :one
SELECT * FROM outbox_events WHERE idempotency_key = sqlc.arg(idempotency_key);

-- name: LockGlobalGenerationAdmission :exec
SELECT pg_advisory_xact_lock(hashtextextended('global-generation-admission', 0));

-- name: CountGlobalActiveReservations :one
SELECT COUNT(*)::bigint
FROM quota_reservations
WHERE status = 'reserved';

-- name: ClaimOutboxEvents :many
WITH candidates AS (
    SELECT candidate.id
    FROM outbox_events AS candidate
    WHERE candidate.processed_at IS NULL
      AND candidate.dead_lettered_at IS NULL
      AND candidate.available_at <= sqlc.arg(now_at)
      AND (candidate.locked_at IS NULL OR candidate.locked_at < sqlc.arg(lock_expired_before))
    ORDER BY candidate.available_at, candidate.created_at
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events AS event
SET locked_at = sqlc.arg(now_at),
    locked_by = sqlc.arg(worker_id),
    attempts = event.attempts + 1
FROM candidates
WHERE event.id = candidates.id
  AND event.processed_at IS NULL
  AND event.dead_lettered_at IS NULL
  AND (event.locked_at IS NULL OR event.locked_at < sqlc.arg(lock_expired_before))
RETURNING event.*;

-- name: CompleteOutboxEvent :execrows
UPDATE outbox_events
SET processed_at = $3, locked_at = NULL, locked_by = NULL, last_error_class = NULL
WHERE id = $1 AND locked_by = $2 AND processed_at IS NULL;

-- name: RetryOutboxEvent :execrows
UPDATE outbox_events
SET available_at = $3, locked_at = NULL, locked_by = NULL, last_error_class = $4
WHERE id = $1 AND locked_by = $2 AND processed_at IS NULL;

-- name: DeadLetterOutboxEvent :execrows
UPDATE outbox_events
SET dead_lettered_at = $3, locked_at = NULL, locked_by = NULL, last_error_class = $4
WHERE id = $1 AND locked_by = $2 AND processed_at IS NULL;

-- name: RecordOutboxReceipt :exec
INSERT INTO outbox_receipts (event_id, handler_name)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: HasOutboxReceipt :one
SELECT EXISTS (
    SELECT 1 FROM outbox_receipts WHERE event_id = $1 AND handler_name = $2
);

-- name: AddDailyUsage :one
INSERT INTO daily_usage (
    actor_type, actor_id, usage_date, messages_used, output_tokens_used, updated_at
)
VALUES (
    sqlc.arg(actor_type), sqlc.arg(actor_id), sqlc.arg(usage_date),
    1, sqlc.arg(output_tokens), sqlc.arg(now_at)
)
ON CONFLICT (actor_type, actor_id, usage_date) DO UPDATE
SET messages_used = daily_usage.messages_used + 1,
    output_tokens_used = daily_usage.output_tokens_used + EXCLUDED.output_tokens_used,
    updated_at = EXCLUDED.updated_at
WHERE daily_usage.messages_used + 1 <= sqlc.arg(message_limit)
  AND daily_usage.output_tokens_used + EXCLUDED.output_tokens_used <= sqlc.arg(output_token_limit)
RETURNING *;

-- name: GetDailyUsage :one
SELECT *
FROM daily_usage
WHERE actor_type = sqlc.arg(actor_type)
  AND actor_id = sqlc.arg(actor_id)
  AND usage_date = sqlc.arg(usage_date);

-- name: AdjustDailyOutputUsage :exec
UPDATE daily_usage
SET output_tokens_used = GREATEST(output_tokens_used + sqlc.arg(delta_tokens), 0),
    updated_at = sqlc.arg(now_at)
WHERE actor_type = sqlc.arg(actor_type)
  AND actor_id = sqlc.arg(actor_id)
  AND usage_date = sqlc.arg(usage_date);

-- name: CreateQuotaReservation :exec
INSERT INTO quota_reservations (
    id, actor_type, actor_id, usage_date, reserved_output_tokens, status
)
VALUES ($1, $2, $3, $4, $5, 'reserved');

-- name: LockQuotaReservation :one
SELECT * FROM quota_reservations WHERE id = $1 FOR UPDATE;

-- name: SettleQuotaReservation :execrows
UPDATE quota_reservations
SET actual_output_tokens = $2,
    status = $3,
    settled_at = $4
WHERE id = $1 AND status = 'reserved';
