-- +goose Up
CREATE TABLE providers (
    id uuid PRIMARY KEY,
    code text NOT NULL UNIQUE CHECK (code ~ '^[a-z][a-z0-9_-]{1,63}$'),
    display_name text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 120),
    adapter text NOT NULL CHECK (adapter IN ('fake', 'openai_compatible')),
    enabled boolean NOT NULL DEFAULT false,
    health_status text NOT NULL DEFAULT 'unknown'
        CHECK (health_status IN ('unknown', 'healthy', 'degraded', 'unavailable')),
    settings jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(settings) = 'object'),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE models (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9._-]{1,127}$'),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    context_window integer NOT NULL CHECK (context_window BETWEEN 1024 AND 10000000),
    max_output_tokens integer NOT NULL CHECK (max_output_tokens BETWEEN 1 AND context_window),
    capabilities jsonb NOT NULL CHECK (
        jsonb_typeof(capabilities) = 'object'
        AND capabilities @> '{"chatCompletions": true}'::jsonb
    ),
    enabled boolean NOT NULL DEFAULT false,
    available boolean NOT NULL DEFAULT false,
    supported boolean NOT NULL DEFAULT false,
    audience text[] NOT NULL DEFAULT '{}'::text[]
        CHECK (audience <@ ARRAY['guest', 'user']::text[]),
    default_for text[] NOT NULL DEFAULT '{}'::text[] CHECK (
        default_for <@ ARRAY['guest', 'user']::text[]
        AND default_for <@ audience
    ),
    sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT enabled OR (available AND supported AND cardinality(audience) > 0))
);
CREATE UNIQUE INDEX models_one_guest_default_idx
    ON models ((true))
    WHERE enabled AND 'guest' = ANY(default_for);
CREATE UNIQUE INDEX models_one_user_default_idx
    ON models ((true))
    WHERE enabled AND 'user' = ANY(default_for);
CREATE INDEX models_catalog_idx ON models (sort_order, name, id) WHERE enabled;

CREATE TABLE provider_models (
    provider_id uuid NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id uuid NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider_model_id text NOT NULL CHECK (length(provider_model_id) BETWEEN 1 AND 200),
    available boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    synced_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider_id, model_id),
    UNIQUE (provider_id, provider_model_id)
);

INSERT INTO providers (
    id, code, display_name, adapter, enabled, health_status, settings
) VALUES (
    '00000000-0000-7000-8000-000000000001',
    'fake',
    'Deterministic Fake',
    'fake',
    true,
    'healthy',
    '{"developmentOnly": true}'::jsonb
);

INSERT INTO models (
    id, slug, name, description, context_window, max_output_tokens,
    capabilities, enabled, available, supported, audience, default_for, sort_order
) VALUES (
    '00000000-0000-7000-8000-000000000101',
    'deepseek-v4-flash',
    'DeepSeek V4 Flash',
    'Default development chat model.',
    131072,
    8192,
    '{"chatCompletions": true, "markdown": true, "code": true}'::jsonb,
    true,
    true,
    true,
    ARRAY['guest', 'user'],
    ARRAY['guest', 'user'],
    0
);

INSERT INTO provider_models (
    provider_id, model_id, provider_model_id, metadata
) VALUES (
    '00000000-0000-7000-8000-000000000001',
    '00000000-0000-7000-8000-000000000101',
    'deepseek-v4-flash',
    '{"deterministic": true}'::jsonb
);

ALTER TABLE conversations
    ADD COLUMN model_id uuid REFERENCES models(id),
    ADD COLUMN generation_state text NOT NULL DEFAULT 'idle'
        CHECK (generation_state IN ('idle', 'accepted', 'streaming')),
    ADD COLUMN renamed_by_user boolean NOT NULL DEFAULT false,
    ADD COLUMN last_message_at timestamptz,
    ADD COLUMN deleted_at timestamptz;
UPDATE conversations
SET model_id = '00000000-0000-7000-8000-000000000101'
WHERE model_id IS NULL;
ALTER TABLE conversations ALTER COLUMN model_id SET NOT NULL;
CREATE INDEX conversations_guest_updated_idx
    ON conversations (guest_session_id, updated_at DESC, id)
    WHERE deleted_at IS NULL;
CREATE INDEX conversations_user_search_idx
    ON conversations USING gin (to_tsvector('simple', title))
    WHERE deleted_at IS NULL;

CREATE TABLE messages (
    id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL DEFAULT '' CHECK (length(content) <= 200000),
    status text NOT NULL CHECK (status IN ('pending', 'complete', 'cancelled', 'failed')),
    sequence integer NOT NULL CHECK (sequence > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, sequence)
);
CREATE INDEX messages_conversation_page_idx ON messages (conversation_id, sequence DESC, id);

CREATE TABLE generations (
    id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    assistant_message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    parent_generation_id uuid REFERENCES generations(id) ON DELETE SET NULL,
    model_id uuid NOT NULL REFERENCES models(id),
    provider_id uuid NOT NULL REFERENCES providers(id),
    quota_reservation_id uuid REFERENCES quota_reservations(id),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    status text NOT NULL CHECK (
        status IN ('accepted', 'streaming', 'completed', 'cancelled', 'failed', 'rejected')
    ),
    retryable boolean NOT NULL DEFAULT false,
    finish_reason text CHECK (finish_reason IN ('stop', 'length', 'cancelled', 'safety', 'error')),
    error_code text,
    input_tokens integer NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens integer NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    cached_tokens integer NOT NULL DEFAULT 0 CHECK (cached_tokens >= 0),
    stream_offset integer NOT NULL DEFAULT 0 CHECK (stream_offset >= 0),
    provider_request_id text,
    accepted_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, idempotency_key),
    CHECK (user_message_id <> assistant_message_id),
    CHECK (
        (status IN ('accepted', 'streaming') AND completed_at IS NULL)
        OR (status IN ('completed', 'cancelled', 'failed', 'rejected') AND completed_at IS NOT NULL)
    )
);
CREATE UNIQUE INDEX generations_one_active_conversation_idx
    ON generations (conversation_id)
    WHERE status IN ('accepted', 'streaming');
CREATE INDEX generations_conversation_created_idx
    ON generations (conversation_id, accepted_at DESC, id);

CREATE TABLE conversation_summaries (
    id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    model_id uuid NOT NULL REFERENCES models(id),
    content text NOT NULL CHECK (length(content) BETWEEN 1 AND 200000),
    from_sequence integer NOT NULL CHECK (from_sequence > 0),
    through_sequence integer NOT NULL CHECK (through_sequence >= from_sequence),
    version integer NOT NULL CHECK (version > 0),
    input_tokens integer NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, version),
    UNIQUE (conversation_id, from_sequence, through_sequence)
);

CREATE TABLE usage_ledger (
    id uuid PRIMARY KEY,
    generation_id uuid NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
    actor_type text NOT NULL CHECK (actor_type IN ('guest', 'user')),
    actor_id uuid NOT NULL,
    provider_id uuid NOT NULL REFERENCES providers(id),
    model_id uuid NOT NULL REFERENCES models(id),
    input_tokens integer NOT NULL CHECK (input_tokens >= 0),
    output_tokens integer NOT NULL CHECK (output_tokens >= 0),
    cached_tokens integer NOT NULL DEFAULT 0 CHECK (cached_tokens >= 0),
    estimated_cost_microunits bigint NOT NULL DEFAULT 0 CHECK (estimated_cost_microunits >= 0),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (generation_id)
);
CREATE INDEX usage_ledger_actor_time_idx
    ON usage_ledger (actor_type, actor_id, occurred_at DESC);

-- +goose Down
DROP TABLE usage_ledger;
DROP TABLE conversation_summaries;
DROP TABLE generations;
DROP TABLE messages;
DROP INDEX conversations_user_search_idx;
DROP INDEX conversations_guest_updated_idx;
ALTER TABLE conversations
    DROP COLUMN deleted_at,
    DROP COLUMN last_message_at,
    DROP COLUMN renamed_by_user,
    DROP COLUMN generation_state,
    DROP COLUMN model_id;
DROP TABLE provider_models;
DROP TABLE models;
DROP TABLE providers;
