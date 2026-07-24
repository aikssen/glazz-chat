-- name: ListRuntimeSettings :many
SELECT * FROM runtime_settings ORDER BY key;

-- name: GetRuntimeSetting :one
SELECT * FROM runtime_settings WHERE key = sqlc.arg(key);

-- name: UpdateRuntimeSetting :one
UPDATE runtime_settings
SET value = sqlc.arg(value),
    version = version + 1,
    updated_by_user_id = sqlc.arg(updated_by_user_id),
    updated_at = sqlc.arg(now_at)
WHERE key = sqlc.arg(key)
  AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: ListAdminModels :many
SELECT * FROM models ORDER BY sort_order, name, id;

-- name: GetAdminModel :one
SELECT * FROM models WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: UpdateAdminModel :one
UPDATE models
SET enabled = COALESCE(sqlc.narg(enabled)::boolean, enabled),
    audience = COALESCE(sqlc.narg(audience)::text[], audience),
    default_for = COALESCE(sqlc.narg(default_for)::text[], default_for),
    sort_order = COALESCE(sqlc.narg(sort_order)::integer, sort_order),
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: CountSelectableDefaults :one
SELECT COUNT(*)::bigint
FROM models
WHERE enabled
  AND available
  AND supported
  AND sqlc.arg(actor_type)::text = ANY(default_for);

-- name: ListAdminUsers :many
SELECT * FROM users
WHERE (
    sqlc.narg(query)::text IS NULL
    OR email ILIKE '%' || sqlc.narg(query)::text || '%'
    OR display_name ILIKE '%' || sqlc.narg(query)::text || '%'
)
AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL
    OR (created_at, id) < (
        sqlc.narg(after_created_at)::timestamptz,
        sqlc.narg(after_id)::uuid
    )
)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: GetAdminUser :one
SELECT * FROM users WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: CountAdministrators :one
SELECT COUNT(*)::bigint FROM users
WHERE role = 'admin' AND status = 'active';

-- name: UpdateUserRole :one
UPDATE users
SET role = sqlc.arg(role),
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: AggregateAdminUsage :one
SELECT
    COUNT(*)::bigint AS generations,
    COALESCE(SUM(input_tokens), 0)::bigint AS input_tokens,
    COALESCE(SUM(output_tokens), 0)::bigint AS output_tokens,
    COALESCE(SUM(estimated_cost_microunits), 0)::bigint AS estimated_cost_microunits
FROM usage_ledger
WHERE occurred_at >= sqlc.arg(period_start)
  AND occurred_at < sqlc.arg(period_end);

-- name: ListAdminAuditEvents :many
SELECT * FROM admin_audit_log
WHERE (
    sqlc.narg(before_occurred_at)::timestamptz IS NULL
    OR (occurred_at, id) < (
        sqlc.narg(before_occurred_at)::timestamptz,
        sqlc.narg(before_id)::uuid
    )
)
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_size);
