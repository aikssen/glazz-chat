-- name: GetAccountDeletionJobByUser :one
SELECT * FROM account_deletion_jobs
WHERE user_id = sqlc.arg(user_id)
ORDER BY requested_at DESC
LIMIT 1;

-- name: CreateAccountDeletionJob :one
INSERT INTO account_deletion_jobs (
    id, user_id, status, requested_at, due_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), 'pending',
    sqlc.arg(requested_at), sqlc.arg(due_at)
)
ON CONFLICT (user_id) DO UPDATE
SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: MarkUserDeletionPending :execrows
UPDATE users
SET status = 'deletion_pending',
    token_version = token_version + 1,
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND status = 'active';

-- name: RevokeAllUserSessions :exec
UPDATE auth_sessions
SET revoked_at = COALESCE(revoked_at, sqlc.arg(now_at)),
    token_version = token_version + 1
WHERE user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL;

-- name: ClaimAccountDeletionJobs :many
WITH candidates AS (
    SELECT pending.id
    FROM account_deletion_jobs AS pending
    WHERE (
        pending.status IN ('pending', 'failed')
        OR (
            pending.status = 'processing'
            AND pending.started_at < sqlc.arg(now_at)::timestamptz - interval '15 minutes'
        )
    )
      AND pending.requested_at <= sqlc.arg(requested_before)
    ORDER BY pending.requested_at, pending.id
    FOR UPDATE SKIP LOCKED
    LIMIT sqlc.arg(batch_size)
)
UPDATE account_deletion_jobs AS jobs
SET status = 'processing',
    started_at = sqlc.arg(now_at),
    attempts = attempts + 1,
    last_error_class = NULL
FROM candidates
WHERE jobs.id = candidates.id
RETURNING jobs.*;

-- name: CompleteAccountDeletionJob :execrows
UPDATE account_deletion_jobs
SET status = 'completed',
    user_id = NULL,
    completed_at = sqlc.arg(now_at),
    last_error_class = NULL
WHERE id = sqlc.arg(id)
  AND status = 'processing';

-- name: FailAccountDeletionJob :execrows
UPDATE account_deletion_jobs
SET status = 'failed',
    last_error_class = sqlc.arg(last_error_class)
WHERE id = sqlc.arg(id)
  AND status = 'processing';

-- name: DeleteUserForPurge :execrows
DELETE FROM users
WHERE id = sqlc.arg(id)
  AND status = 'deletion_pending';

-- name: DeleteExpiredGuestSessions :execrows
DELETE FROM guest_sessions AS guests
WHERE guests.expires_at < sqlc.arg(expired_before)
  AND guests.migrated_user_id IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM conversations
      JOIN generations ON generations.conversation_id = conversations.id
      WHERE conversations.guest_session_id = guests.id
        AND generations.status IN ('accepted', 'streaming')
  );
