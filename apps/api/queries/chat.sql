-- name: NextMessageSequence :one
SELECT COALESCE(MAX(sequence), 0)::integer + 1
FROM messages
WHERE conversation_id = sqlc.arg(conversation_id);

-- name: CreateMessage :one
INSERT INTO messages (
    id, conversation_id, role, content, status, sequence, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(role), sqlc.arg(content),
    sqlc.arg(status), sqlc.arg(sequence), sqlc.arg(now_at), sqlc.arg(now_at)
)
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = sqlc.arg(id);

-- name: ListConversationMessages :many
SELECT *
FROM messages
WHERE conversation_id = sqlc.arg(conversation_id)
  AND (
      sqlc.narg(before_sequence)::integer IS NULL
      OR sequence < sqlc.narg(before_sequence)::integer
  )
ORDER BY sequence DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListContextMessages :many
SELECT *
FROM messages
WHERE conversation_id = sqlc.arg(conversation_id)
  AND (
      role = 'user'
      OR status = 'complete'
  )
ORDER BY sequence, id;

-- name: AppendAssistantMessage :one
UPDATE messages
SET content = content || sqlc.arg(delta),
    status = 'pending',
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND role = 'assistant'
  AND status = 'pending'
RETURNING *;

-- name: FinalizeMessage :execrows
UPDATE messages
SET status = sqlc.arg(status),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND status = 'pending';

-- name: CreateGeneration :one
INSERT INTO generations (
    id, conversation_id, user_message_id, assistant_message_id,
    parent_generation_id, model_id, provider_id, quota_reservation_id,
    idempotency_key, status, accepted_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(user_message_id),
    sqlc.arg(assistant_message_id), sqlc.narg(parent_generation_id),
    sqlc.arg(model_id), sqlc.arg(provider_id), sqlc.narg(quota_reservation_id),
    sqlc.arg(idempotency_key), 'accepted', sqlc.arg(now_at), sqlc.arg(now_at)
)
ON CONFLICT (conversation_id, idempotency_key) DO UPDATE
SET idempotency_key = EXCLUDED.idempotency_key
WHERE false
RETURNING *;

-- name: GetGeneration :one
SELECT * FROM generations WHERE id = sqlc.arg(id);

-- name: GetGenerationByIdempotencyKey :one
SELECT *
FROM generations
WHERE conversation_id = sqlc.arg(conversation_id)
  AND idempotency_key = sqlc.arg(idempotency_key);

-- name: GetLatestGeneration :one
SELECT *
FROM generations
WHERE conversation_id = sqlc.arg(conversation_id)
ORDER BY accepted_at DESC, id DESC
LIMIT 1;

-- name: MarkGenerationStreaming :one
UPDATE generations
SET status = 'streaming',
    started_at = COALESCE(started_at, sqlc.arg(now_at)),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND status = 'accepted'
RETURNING *;

-- name: CheckpointGeneration :execrows
UPDATE generations
SET stream_offset = sqlc.arg(stream_offset),
    output_tokens = sqlc.arg(output_tokens),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND status = 'streaming'
  AND stream_offset <= sqlc.arg(stream_offset);

-- name: FinalizeGeneration :one
UPDATE generations
SET status = sqlc.arg(status),
    retryable = sqlc.arg(retryable),
    finish_reason = sqlc.narg(finish_reason),
    error_code = sqlc.narg(error_code),
    input_tokens = sqlc.arg(input_tokens),
    output_tokens = sqlc.arg(output_tokens),
    cached_tokens = sqlc.arg(cached_tokens),
    stream_offset = sqlc.arg(stream_offset),
    provider_request_id = sqlc.narg(provider_request_id),
    completed_at = sqlc.arg(now_at),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND status IN ('accepted', 'streaming')
RETURNING *;

-- name: CreateUsageLedgerEntry :exec
INSERT INTO usage_ledger (
    id, generation_id, actor_type, actor_id, provider_id, model_id,
    input_tokens, output_tokens, cached_tokens, estimated_cost_microunits, occurred_at
) VALUES (
    sqlc.arg(id), sqlc.arg(generation_id), sqlc.arg(actor_type), sqlc.arg(actor_id),
    sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(input_tokens),
    sqlc.arg(output_tokens), sqlc.arg(cached_tokens),
    sqlc.arg(estimated_cost_microunits), sqlc.arg(now_at)
)
ON CONFLICT (generation_id) DO NOTHING;

-- name: GetLatestSummary :one
SELECT *
FROM conversation_summaries
WHERE conversation_id = sqlc.arg(conversation_id)
ORDER BY version DESC
LIMIT 1;

-- name: CreateConversationSummary :one
INSERT INTO conversation_summaries (
    id, conversation_id, model_id, content, from_sequence, through_sequence,
    version, input_tokens, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(conversation_id), sqlc.arg(model_id), sqlc.arg(content),
    sqlc.arg(from_sequence), sqlc.arg(through_sequence), sqlc.arg(version),
    sqlc.arg(input_tokens), sqlc.arg(now_at)
)
ON CONFLICT (conversation_id, version) DO UPDATE
SET content = conversation_summaries.content
RETURNING *;

-- name: GetActorUsage :one
SELECT
    COALESCE(SUM(input_tokens), 0)::bigint AS input_tokens,
    COALESCE(SUM(output_tokens), 0)::bigint AS output_tokens,
    COUNT(*)::bigint AS generations
FROM usage_ledger
WHERE actor_type = sqlc.arg(actor_type)
  AND actor_id = sqlc.arg(actor_id)
  AND occurred_at >= sqlc.arg(from_at);

-- name: CountActorActiveGenerations :one
SELECT COUNT(*)::bigint
FROM generations g
JOIN conversations c ON c.id = g.conversation_id
WHERE g.status IN ('accepted', 'streaming')
  AND (
      (sqlc.arg(actor_type)::text = 'user' AND c.user_id = sqlc.arg(actor_id))
      OR
      (sqlc.arg(actor_type)::text = 'guest' AND c.guest_session_id = sqlc.arg(actor_id))
  );
