-- name: CreateUserConversation :one
INSERT INTO conversations (
    id, user_id, title, model_id, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(title), sqlc.arg(model_id),
    sqlc.arg(now_at), sqlc.arg(now_at)
)
RETURNING *;

-- name: CreateGuestConversation :one
INSERT INTO conversations (
    id, guest_session_id, title, model_id, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(guest_session_id), sqlc.arg(title), sqlc.arg(model_id),
    sqlc.arg(now_at), sqlc.arg(now_at)
)
RETURNING *;

-- name: GetUserConversation :one
SELECT *
FROM conversations
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL;

-- name: GetGuestConversation :one
SELECT *
FROM conversations
WHERE id = sqlc.arg(id)
  AND guest_session_id = sqlc.arg(guest_session_id)
  AND deleted_at IS NULL;

-- name: GetGuestConversationByOwner :one
SELECT *
FROM conversations
WHERE guest_session_id = sqlc.arg(guest_session_id)
  AND deleted_at IS NULL;

-- name: ListUserConversations :many
SELECT *
FROM conversations
WHERE user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
  AND (sqlc.arg(include_archived)::boolean OR status = 'active')
  AND (
      sqlc.narg(search)::text IS NULL
      OR to_tsvector('simple', title) @@ plainto_tsquery('simple', sqlc.narg(search)::text)
  )
  AND (
      sqlc.narg(before_updated_at)::timestamptz IS NULL
      OR (updated_at, id) < (
          sqlc.narg(before_updated_at)::timestamptz,
          sqlc.narg(before_id)::uuid
      )
  )
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateUserConversation :one
UPDATE conversations
SET title = COALESCE(sqlc.narg(title)::text, title),
    status = COALESCE(sqlc.narg(status)::text, status),
    model_id = COALESCE(sqlc.narg(model_id)::uuid, model_id),
    renamed_by_user = renamed_by_user OR sqlc.narg(title)::text IS NOT NULL,
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(model_id)::uuid IS NULL
      OR generation_state = 'idle'
  )
RETURNING *;

-- name: UpdateGuestConversation :one
UPDATE conversations
SET title = COALESCE(sqlc.narg(title)::text, title),
    model_id = COALESCE(sqlc.narg(model_id)::uuid, model_id),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND guest_session_id = sqlc.arg(guest_session_id)
  AND deleted_at IS NULL
  AND (
      sqlc.narg(model_id)::uuid IS NULL
      OR generation_state = 'idle'
  )
RETURNING *;

-- name: SoftDeleteUserConversation :execrows
UPDATE conversations
SET deleted_at = sqlc.arg(now_at),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
  AND generation_state = 'idle';

-- name: SoftDeleteGuestConversation :execrows
UPDATE conversations
SET deleted_at = sqlc.arg(now_at),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND guest_session_id = sqlc.arg(guest_session_id)
  AND deleted_at IS NULL
  AND generation_state = 'idle';

-- name: SetConversationGenerationState :execrows
UPDATE conversations
SET generation_state = sqlc.arg(state),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: SetGeneratedConversationTitle :execrows
UPDATE conversations
SET title = sqlc.arg(title),
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND NOT renamed_by_user
  AND title = 'New conversation';
