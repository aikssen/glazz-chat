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

-- name: ClearOtherModelDefault :many
UPDATE models
SET default_for = array_remove(default_for, sqlc.arg(actor_type)::text),
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE id <> sqlc.arg(model_id)
  AND sqlc.arg(actor_type)::text = ANY(default_for)
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

-- name: LockAdministratorRoleChanges :exec
SELECT pg_advisory_xact_lock(hashtextextended('glazz.admin-role-changes', 0));

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
    COUNT(*) FILTER (
        WHERE generations.status IN ('failed', 'rejected')
    )::bigint AS failed_generations,
    COALESCE(SUM(usage_ledger.input_tokens), 0)::bigint AS input_tokens,
    COALESCE(SUM(usage_ledger.output_tokens), 0)::bigint AS output_tokens,
    COALESCE(SUM(usage_ledger.estimated_cost_microunits), 0)::bigint AS estimated_cost_microunits,
    COALESCE(
        AVG(EXTRACT(EPOCH FROM (generations.completed_at - generations.accepted_at)) * 1000)
            FILTER (WHERE generations.completed_at IS NOT NULL),
        0
    )::double precision AS average_latency_ms,
    COALESCE(
        percentile_cont(0.95) WITHIN GROUP (
            ORDER BY EXTRACT(EPOCH FROM (generations.completed_at - generations.accepted_at)) * 1000
        ) FILTER (WHERE generations.completed_at IS NOT NULL),
        0
    )::double precision AS p95_latency_ms
FROM usage_ledger
JOIN generations ON generations.id = usage_ledger.generation_id
WHERE usage_ledger.occurred_at >= sqlc.arg(period_start)
  AND usage_ledger.occurred_at < sqlc.arg(period_end);

-- name: AggregateAdminErrors :many
SELECT
    COALESCE(NULLIF(error_code, ''), 'unknown')::text AS code,
    COUNT(*)::bigint AS count
FROM generations
WHERE completed_at >= sqlc.arg(period_start)
  AND completed_at < sqlc.arg(period_end)
  AND status IN ('failed', 'rejected')
GROUP BY COALESCE(NULLIF(error_code, ''), 'unknown')
ORDER BY count DESC, code;

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
