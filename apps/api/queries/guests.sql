-- name: CreateGuestSession :one
INSERT INTO guest_sessions (id, identity_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AdjustGuestOutputUsage :exec
UPDATE guest_sessions
SET output_token_count = GREATEST(output_token_count + sqlc.arg(delta_tokens), 0),
    last_seen_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(guest_id);

-- name: GetGuestSessionByIdentityHash :one
SELECT * FROM guest_sessions WHERE identity_hash = $1;

-- name: GetGuestSession :one
SELECT * FROM guest_sessions WHERE id = $1;

-- name: TouchGuestSession :exec
UPDATE guest_sessions SET last_seen_at = $2 WHERE id = $1;

-- name: LockGuestSession :one
SELECT * FROM guest_sessions WHERE id = $1 FOR UPDATE;

-- name: IncrementGuestAllowance :one
UPDATE guest_sessions
SET prompt_count = prompt_count + 1,
    output_token_count = output_token_count + sqlc.arg(output_tokens),
    last_seen_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(guest_id)
  AND migrated_user_id IS NULL
  AND expires_at > sqlc.arg(now_at)
  AND prompt_count < sqlc.arg(message_limit)
  AND output_token_count + sqlc.arg(output_tokens) <= sqlc.arg(output_token_limit)
RETURNING *;

-- name: MigrateGuestConversations :execrows
UPDATE conversations
SET user_id = $2, guest_session_id = NULL, updated_at = $3
WHERE guest_session_id = $1;

-- name: MarkGuestMigrated :execrows
UPDATE guest_sessions
SET migrated_user_id = $2, migrated_at = $3, last_seen_at = $3
WHERE id = $1 AND migrated_user_id IS NULL;
