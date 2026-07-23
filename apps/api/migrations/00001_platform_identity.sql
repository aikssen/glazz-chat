-- +goose Up
CREATE TABLE glazz_migration_checksums (
    version bigint PRIMARY KEY,
    checksum text NOT NULL CHECK (length(checksum) = 64),
    recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    display_name text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 120),
    avatar_url text,
    locale text NOT NULL DEFAULT 'en' CHECK (locale IN ('en', 'es')),
    role text NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    plan text NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'pro')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deletion_pending', 'disabled')),
    token_version integer NOT NULL DEFAULT 1 CHECK (token_version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_normalized_key ON users (lower(email));

CREATE TABLE user_identities (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider = 'google'),
    provider_subject text NOT NULL,
    verified_email text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_subject),
    UNIQUE (user_id, provider)
);

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id uuid NOT NULL,
    device_label text,
    token_version integer NOT NULL DEFAULT 1 CHECK (token_version > 0),
    refresh_expires_at timestamptz NOT NULL,
    recent_auth_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    reuse_detected_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_user_active_idx
    ON auth_sessions (user_id, created_at DESC)
    WHERE revoked_at IS NULL;
CREATE INDEX auth_sessions_family_idx ON auth_sessions (family_id);

CREATE TABLE auth_refresh_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    session_id uuid NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);
CREATE INDEX auth_refresh_tokens_session_idx ON auth_refresh_tokens (session_id);

CREATE TABLE terms_acceptances (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    terms_version text NOT NULL,
    privacy_version text NOT NULL,
    ip_hash bytea CHECK (ip_hash IS NULL OR octet_length(ip_hash) = 32),
    accepted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, terms_version, privacy_version)
);

CREATE TABLE guest_sessions (
    id uuid PRIMARY KEY,
    identity_hash bytea NOT NULL UNIQUE CHECK (octet_length(identity_hash) = 32),
    prompt_count integer NOT NULL DEFAULT 0 CHECK (prompt_count BETWEEN 0 AND 4),
    output_token_count integer NOT NULL DEFAULT 0 CHECK (output_token_count BETWEEN 0 AND 2000),
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    migrated_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    migrated_at timestamptz,
    CHECK ((migrated_user_id IS NULL) = (migrated_at IS NULL))
);
CREATE INDEX guest_sessions_expiry_idx
    ON guest_sessions (expires_at)
    WHERE migrated_user_id IS NULL;

CREATE TABLE conversations (
    id uuid PRIMARY KEY,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    guest_session_id uuid REFERENCES guest_sessions(id) ON DELETE CASCADE,
    title text NOT NULL DEFAULT 'New conversation' CHECK (length(title) BETWEEN 1 AND 120),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((user_id IS NOT NULL)::integer + (guest_session_id IS NOT NULL)::integer = 1)
);
CREATE INDEX conversations_user_updated_idx ON conversations (user_id, updated_at DESC, id);
CREATE UNIQUE INDEX conversations_guest_single_idx ON conversations (guest_session_id);

CREATE TABLE daily_usage (
    actor_type text NOT NULL CHECK (actor_type IN ('guest', 'user', 'global')),
    actor_id uuid NOT NULL,
    usage_date date NOT NULL,
    messages_used integer NOT NULL DEFAULT 0 CHECK (messages_used >= 0),
    output_tokens_used bigint NOT NULL DEFAULT 0 CHECK (output_tokens_used >= 0),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (actor_type, actor_id, usage_date)
);

CREATE TABLE quota_reservations (
    id uuid PRIMARY KEY,
    actor_type text NOT NULL CHECK (actor_type IN ('guest', 'user')),
    actor_id uuid NOT NULL,
    usage_date date NOT NULL,
    reserved_output_tokens integer NOT NULL CHECK (reserved_output_tokens > 0),
    actual_output_tokens integer CHECK (actual_output_tokens >= 0),
    status text NOT NULL CHECK (status IN ('reserved', 'committed', 'refunded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz
);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    event_type text NOT NULL CHECK (event_type ~ '^[a-z][a-z0-9_.-]+$'),
    payload jsonb NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by text,
    processed_at timestamptz,
    dead_lettered_at timestamptz,
    last_error_class text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT (processed_at IS NOT NULL AND dead_lettered_at IS NOT NULL))
);
CREATE INDEX outbox_events_claim_idx
    ON outbox_events (available_at, created_at)
    WHERE processed_at IS NULL AND dead_lettered_at IS NULL;

CREATE TABLE outbox_receipts (
    event_id uuid NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE,
    handler_name text NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, handler_name)
);

CREATE TABLE admin_audit_log (
    id uuid PRIMARY KEY,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    before_value jsonb,
    after_value jsonb,
    request_id text,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_log_occurred_idx ON admin_audit_log (occurred_at DESC, id);

-- +goose Down
DROP TABLE admin_audit_log;
DROP TABLE outbox_receipts;
DROP TABLE outbox_events;
DROP TABLE quota_reservations;
DROP TABLE daily_usage;
DROP TABLE conversations;
DROP TABLE guest_sessions;
DROP TABLE terms_acceptances;
DROP TABLE auth_refresh_tokens;
DROP TABLE auth_sessions;
DROP TABLE user_identities;
DROP TABLE users;
DROP TABLE glazz_migration_checksums;
