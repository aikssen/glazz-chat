-- name: ListPublicModels :many
SELECT *
FROM models
WHERE models.enabled
  AND models.available
  AND models.supported
  AND sqlc.arg(actor_type)::text = ANY(models.audience)
  AND (
    sqlc.arg(actor_type)::text <> 'guest'
    OR sqlc.arg(actor_type)::text = ANY(models.default_for)
  )
  AND EXISTS (
    SELECT 1
    FROM provider_models
    JOIN providers ON providers.id = provider_models.provider_id
    WHERE provider_models.model_id = models.id
      AND provider_models.available
      AND providers.enabled
      AND providers.health_status <> 'unavailable'
  )
ORDER BY models.sort_order, models.name, models.id;

-- name: GetSelectableModel :one
SELECT *
FROM models
WHERE models.id = sqlc.arg(model_id)
  AND models.enabled
  AND models.available
  AND models.supported
  AND sqlc.arg(actor_type)::text = ANY(models.audience)
  AND (
    sqlc.arg(actor_type)::text <> 'guest'
    OR sqlc.arg(actor_type)::text = ANY(models.default_for)
  )
  AND EXISTS (
    SELECT 1
    FROM provider_models
    JOIN providers ON providers.id = provider_models.provider_id
    WHERE provider_models.model_id = models.id
      AND provider_models.available
      AND providers.enabled
      AND providers.health_status <> 'unavailable'
  );

-- name: GetDefaultModel :one
SELECT *
FROM models
WHERE models.enabled
  AND models.available
  AND models.supported
  AND sqlc.arg(actor_type)::text = ANY(models.default_for)
  AND EXISTS (
    SELECT 1
    FROM provider_models
    JOIN providers ON providers.id = provider_models.provider_id
    WHERE provider_models.model_id = models.id
      AND provider_models.available
      AND providers.enabled
      AND providers.health_status <> 'unavailable'
  );

-- name: GetProviderForModel :one
SELECT
    provider_models.provider_id,
    provider_models.model_id,
    provider_models.provider_model_id,
    provider_models.available AS mapping_available,
    provider_models.metadata,
    provider_models.synced_at,
    providers.code AS provider_code,
    providers.adapter,
    providers.enabled AS provider_enabled,
    providers.health_status,
    providers.settings
FROM provider_models
JOIN providers ON providers.id = provider_models.provider_id
WHERE provider_models.model_id = sqlc.arg(model_id)
  AND provider_models.available
  AND providers.enabled
  AND providers.health_status <> 'unavailable'
ORDER BY providers.code
LIMIT 1;

-- name: UpsertProvider :one
INSERT INTO providers (
    id, code, display_name, adapter, enabled, health_status, settings, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(code), sqlc.arg(display_name), sqlc.arg(adapter),
    sqlc.arg(enabled), sqlc.arg(health_status), sqlc.arg(settings), sqlc.arg(now_at)
)
ON CONFLICT (code) DO UPDATE
SET display_name = EXCLUDED.display_name,
    adapter = EXCLUDED.adapter,
    enabled = EXCLUDED.enabled,
    health_status = EXCLUDED.health_status,
    settings = EXCLUDED.settings,
    version = providers.version + 1,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: SetProviderEnabledByCode :exec
UPDATE providers
SET enabled = sqlc.arg(enabled),
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE code = sqlc.arg(code)
  AND enabled IS DISTINCT FROM sqlc.arg(enabled);

-- name: GetModelBySlug :one
SELECT * FROM models WHERE slug = sqlc.arg(slug);

-- name: CreateDiscoveredModel :one
INSERT INTO models (
    id, slug, name, description, context_window, max_output_tokens,
    capabilities, enabled, available, supported, audience, default_for,
    sort_order, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(slug), sqlc.arg(name), sqlc.arg(description),
    sqlc.arg(context_window), sqlc.arg(max_output_tokens), sqlc.arg(capabilities),
    false, false, true, ARRAY['user']::text[], '{}'::text[],
    sqlc.arg(sort_order), sqlc.arg(now_at), sqlc.arg(now_at)
)
RETURNING *;

-- name: UpsertProviderModel :exec
INSERT INTO provider_models (
    provider_id, model_id, provider_model_id, available, metadata, synced_at
) VALUES (
    sqlc.arg(provider_id), sqlc.arg(model_id), sqlc.arg(provider_model_id),
    true, sqlc.arg(metadata), sqlc.arg(synced_at)
)
ON CONFLICT (provider_id, model_id) DO UPDATE
SET provider_model_id = EXCLUDED.provider_model_id,
    available = true,
    metadata = EXCLUDED.metadata,
    synced_at = EXCLUDED.synced_at;

-- name: MarkProviderModelsUnavailable :exec
UPDATE provider_models
SET available = false,
    synced_at = sqlc.arg(synced_at)
WHERE provider_id = sqlc.arg(provider_id)
  AND NOT (provider_model_id = ANY(sqlc.arg(present_model_ids)::text[]));

-- name: GetProviderByCode :one
SELECT * FROM providers WHERE code = sqlc.arg(code);

-- name: ListProviderModelMappings :many
SELECT * FROM provider_models
WHERE provider_id = sqlc.arg(provider_id)
ORDER BY provider_model_id, model_id;

-- name: RefreshModelAvailability :execrows
UPDATE models
SET available = EXISTS (
        SELECT 1
        FROM provider_models
        JOIN providers ON providers.id = provider_models.provider_id
        WHERE provider_models.model_id = models.id
          AND provider_models.available
          AND providers.enabled
    ),
    version = version + 1,
    updated_at = sqlc.arg(now_at)
WHERE available IS DISTINCT FROM EXISTS (
    SELECT 1
    FROM provider_models
    JOIN providers ON providers.id = provider_models.provider_id
    WHERE provider_models.model_id = models.id
      AND provider_models.available
      AND providers.enabled
);
