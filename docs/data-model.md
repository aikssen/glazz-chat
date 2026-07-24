# Glazz Data Model and ERD

## Purpose and authority

This document explains the PostgreSQL model, its ownership boundaries, lifecycle,
and principal constraints. The executable source of truth is
[`apps/api/migrations`](../apps/api/migrations/); typed queries live under
`apps/api/internal/platform/store`.

PostgreSQL is the durable system of record. Redis data is intentionally absent from
the ERD because it is TTL-bound coordination/cache state, not relational authority.

## Domain map

```mermaid
flowchart LR
    Identity["Identity and sessions"]
    Guest["Guest acquisition"]
    Chat["Conversation and generation"]
    Catalog["Provider and model catalog"]
    Usage["Quota and usage"]
    Control["Settings and administration"]
    Async["Outbox and privacy jobs"]

    Identity --> Chat
    Guest --> Chat
    Guest -->|migration| Identity
    Catalog --> Chat
    Chat --> Usage
    Control --> Catalog
    Control --> Usage
    Identity --> Control
    Async --> Identity
    Async --> Catalog
```

## Complete relationship view

This diagram emphasizes cardinality. Polymorphic references such as
`daily_usage.actor_id` are application-validated and described separately because
PostgreSQL cannot express one foreign key to multiple actor tables.

```mermaid
erDiagram
    USERS ||--o{ USER_IDENTITIES : has
    USERS ||--o{ AUTH_SESSIONS : opens
    AUTH_SESSIONS ||--o{ AUTH_REFRESH_TOKENS : rotates
    USERS ||--o{ TERMS_ACCEPTANCES : accepts
    USERS o|--o{ GUEST_SESSIONS : receives_migration
    USERS ||--o{ CONVERSATIONS : owns
    GUEST_SESSIONS ||--o| CONVERSATIONS : owns
    MODELS ||--o{ CONVERSATIONS : selected_for
    CONVERSATIONS ||--o{ MESSAGES : contains
    CONVERSATIONS ||--o{ GENERATIONS : runs
    MESSAGES ||--o{ GENERATIONS : user_input
    MESSAGES ||--o{ GENERATIONS : assistant_output
    GENERATIONS o|--o{ GENERATIONS : retries
    MODELS ||--o{ GENERATIONS : executes_as
    PROVIDERS ||--o{ GENERATIONS : executes_through
    QUOTA_RESERVATIONS o|--o| GENERATIONS : funds
    CONVERSATIONS ||--o{ CONVERSATION_SUMMARIES : summarizes
    MODELS ||--o{ CONVERSATION_SUMMARIES : produced_by
    GENERATIONS ||--o| USAGE_LEDGER : accounts
    PROVIDERS ||--o{ USAGE_LEDGER : attributes
    MODELS ||--o{ USAGE_LEDGER : attributes
    PROVIDERS ||--o{ PROVIDER_MODELS : maps
    MODELS ||--o{ PROVIDER_MODELS : maps
    USERS o|--o{ RUNTIME_SETTINGS : updates
    USERS o|--o{ ADMIN_AUDIT_LOG : acts
    USERS o|--o| ACCOUNT_DELETION_JOBS : requests
    OUTBOX_EVENTS ||--o{ OUTBOX_RECEIPTS : handled_by
```

## Identity and guest ERD

```mermaid
erDiagram
    USERS {
        uuid id PK
        text email UK
        text display_name
        text avatar_url
        text locale
        text role
        text plan
        text status
        int token_version
        int version
        timestamptz created_at
        timestamptz updated_at
    }

    USER_IDENTITIES {
        uuid id PK
        uuid user_id FK
        text provider
        text provider_subject UK
        text verified_email
        timestamptz created_at
        timestamptz updated_at
    }

    AUTH_SESSIONS {
        uuid id PK
        uuid user_id FK
        uuid family_id
        text device_label
        int token_version
        timestamptz refresh_expires_at
        timestamptz recent_auth_at
        timestamptz last_seen_at
        timestamptz revoked_at
        timestamptz reuse_detected_at
        timestamptz created_at
    }

    AUTH_REFRESH_TOKENS {
        bytea token_hash PK
        uuid session_id FK
        timestamptz created_at
        timestamptz expires_at
        timestamptz used_at
    }

    TERMS_ACCEPTANCES {
        uuid user_id PK,FK
        text terms_version PK
        text privacy_version PK
        bytea ip_hash
        timestamptz accepted_at
    }

    GUEST_SESSIONS {
        uuid id PK
        bytea identity_hash UK
        int prompt_count
        int output_token_count
        timestamptz first_seen_at
        timestamptz last_seen_at
        timestamptz expires_at
        uuid migrated_user_id FK
        timestamptz migrated_at
    }

    ACCOUNT_DELETION_JOBS {
        uuid id PK
        uuid user_id FK,UK
        text status
        timestamptz requested_at
        timestamptz due_at
        timestamptz started_at
        timestamptz completed_at
        int attempts
        text last_error_class
    }

    USERS ||--o{ USER_IDENTITIES : has
    USERS ||--o{ AUTH_SESSIONS : opens
    AUTH_SESSIONS ||--o{ AUTH_REFRESH_TOKENS : rotates
    USERS ||--o{ TERMS_ACCEPTANCES : accepts
    USERS o|--o{ GUEST_SESSIONS : migrated_to
    USERS o|--o| ACCOUNT_DELETION_JOBS : deletion
```

### Identity invariants

- Email uniqueness is case-insensitive through `lower(email)`.
- Google subject, not email, is the stable external identity key.
- One user has at most one identity per provider.
- Roles are `user` or `admin`; status is `active`, `deletion_pending`, or
  `disabled`.
- User and session token versions invalidate previously issued access tokens.
- Refresh tokens are stored as 32-byte hashes, never reusable plaintext.
- Active sessions and session families are indexed for revocation/reuse handling.
- Legal acceptance is versioned by both Terms and Privacy versions.
- IP evidence is an optional fixed-length hash, not a raw address.
- Guest identity is stored as a fixed-length hash and referenced through a signed
  cookie.
- `migrated_user_id` and `migrated_at` must be null or non-null together.
- A deletion job must be due no later than 24 hours after request.

## Conversation and generation ERD

```mermaid
erDiagram
    CONVERSATIONS {
        uuid id PK
        uuid user_id FK
        uuid guest_session_id FK
        uuid model_id FK
        text title
        text status
        text generation_state
        boolean renamed_by_user
        timestamptz last_message_at
        timestamptz deleted_at
        text creation_idempotency_key
        text deletion_idempotency_key
        timestamptz created_at
        timestamptz updated_at
    }

    MESSAGES {
        uuid id PK
        uuid conversation_id FK
        text role
        text content
        text status
        int sequence
        timestamptz created_at
        timestamptz updated_at
    }

    GENERATIONS {
        uuid id PK
        uuid conversation_id FK
        uuid user_message_id FK
        uuid assistant_message_id FK
        uuid parent_generation_id FK
        uuid model_id FK
        uuid provider_id FK
        uuid quota_reservation_id FK
        text idempotency_key
        text status
        boolean retryable
        text finish_reason
        text error_code
        int input_tokens
        int output_tokens
        int cached_tokens
        int stream_offset
        text provider_request_id
        timestamptz accepted_at
        timestamptz started_at
        timestamptz completed_at
        timestamptz updated_at
    }

    CONVERSATION_SUMMARIES {
        uuid id PK
        uuid conversation_id FK
        uuid model_id FK
        text content
        int from_sequence
        int through_sequence
        int version
        int input_tokens
        timestamptz created_at
    }

    USERS ||--o{ CONVERSATIONS : owns
    GUEST_SESSIONS ||--o| CONVERSATIONS : owns
    MODELS ||--o{ CONVERSATIONS : selected
    CONVERSATIONS ||--o{ MESSAGES : contains
    CONVERSATIONS ||--o{ GENERATIONS : runs
    MESSAGES ||--o{ GENERATIONS : user_message
    MESSAGES ||--o{ GENERATIONS : assistant_message
    GENERATIONS o|--o{ GENERATIONS : parent
    CONVERSATIONS ||--o{ CONVERSATION_SUMMARIES : has
    MODELS ||--o{ CONVERSATION_SUMMARIES : creates
```

`USERS`, `GUEST_SESSIONS`, and `MODELS` are shown as relationship endpoints here
and defined in their respective domain diagrams.

### Ownership invariants

- A conversation has exactly one owner: `user_id` XOR `guest_session_id`.
- One guest session can own at most one conversation.
- Registered ownership is indexed for stable `updated_at, id` pagination.
- Guest and registered create requests have separate owner-scoped idempotency
  indexes.
- User search uses a GIN index over the conversation title.
- Soft-deleted conversations are excluded from active-list indexes.

### Message invariants

- Message role is `user` or `assistant`.
- Message status is `pending`, `complete`, `cancelled`, or `failed`.
- `(conversation_id, sequence)` is unique and provides deterministic transcript
  ordering.
- Content has an upper bound of 200,000 characters.

### Generation invariants

- Every generation references distinct user and assistant messages.
- A partial unique index allows at most one `accepted` or `streaming` generation
  per conversation.
- `(conversation_id, idempotency_key)` is unique.
- Active states must not have `completed_at`; terminal states must have it.
- Terminal status is `completed`, `cancelled`, `failed`, or `rejected`.
- Finish reason is normalized to `stop`, `length`, `cancelled`, `safety`, or
  `error`.
- `stream_offset` persists progress for duplicate/gap handling.
- A retry points to its parent generation rather than branching the public
  conversation model.

### Summary invariants

Conversation summaries are modeled for bounded context construction:

- sequence range is explicit and valid;
- version is unique per conversation;
- the same range cannot be summarized twice;
- the producing model and input usage are retained;
- original messages remain authoritative.

## Provider and model catalog ERD

```mermaid
erDiagram
    PROVIDERS {
        uuid id PK
        text code UK
        text display_name
        text adapter
        boolean enabled
        text health_status
        jsonb settings
        int version
        timestamptz created_at
        timestamptz updated_at
    }

    MODELS {
        uuid id PK
        text slug UK
        text name
        text description
        int context_window
        int max_output_tokens
        jsonb capabilities
        boolean enabled
        boolean available
        boolean supported
        text_array audience
        text_array default_for
        int sort_order
        int version
        timestamptz created_at
        timestamptz updated_at
    }

    PROVIDER_MODELS {
        uuid provider_id PK,FK
        uuid model_id PK,FK
        text provider_model_id UK
        boolean available
        jsonb metadata
        timestamptz synced_at
    }

    PROVIDERS ||--o{ PROVIDER_MODELS : exposes
    MODELS ||--o{ PROVIDER_MODELS : maps
    MODELS ||--o{ CONVERSATIONS : selected_by
    MODELS ||--o{ GENERATIONS : used_by
    PROVIDERS ||--o{ GENERATIONS : serves
```

### Catalog invariants

- Provider adapter is `fake` or `openai_compatible`.
- Provider health is normalized to `unknown`, `healthy`, `degraded`, or
  `unavailable`.
- Model capability JSON must include `chatCompletions: true`.
- Audience/default arrays may contain only `guest` and `user`.
- Defaults must be a subset of audience.
- Partial unique indexes permit only one enabled guest default and one enabled user
  default.
- An enabled model must be supported and have an audience.
- Availability is tracked separately from enablement so an unavailable configured
  model can remain visible to administrators without being selected for new work.
- Provider model identifiers are isolated in the mapping table.

## Quota and usage ERD

```mermaid
erDiagram
    DAILY_USAGE {
        text actor_type PK
        uuid actor_id PK
        date usage_date PK
        int messages_used
        bigint output_tokens_used
        timestamptz updated_at
    }

    QUOTA_RESERVATIONS {
        uuid id PK
        text actor_type
        uuid actor_id
        date usage_date
        int reserved_output_tokens
        int actual_output_tokens
        text status
        timestamptz created_at
        timestamptz settled_at
    }

    USAGE_LEDGER {
        uuid id PK
        uuid generation_id FK,UK
        text actor_type
        uuid actor_id
        uuid provider_id FK
        uuid model_id FK
        int input_tokens
        int output_tokens
        int cached_tokens
        bigint estimated_cost_microunits
        timestamptz occurred_at
    }

    GENERATIONS o|--o| QUOTA_RESERVATIONS : consumes
    GENERATIONS ||--o| USAGE_LEDGER : records
    PROVIDERS ||--o{ USAGE_LEDGER : attributes
    MODELS ||--o{ USAGE_LEDGER : attributes
```

### Polymorphic actors

`actor_type` plus `actor_id` identifies a guest or user. This is intentionally not
a database foreign key because the ID may refer to either `guest_sessions` or
`users`; `daily_usage` also permits the synthetic `global` actor. Services validate
the pair and lifecycle cleanup preserves only allowed anonymous aggregates.

### Accounting model

1. A command reserves a maximum output-token amount before provider work.
2. The reservation becomes `committed` with actual use or `refunded`.
3. `daily_usage` provides fast enforcement by actor/day.
4. `usage_ledger` records one immutable attribution per completed generation.
5. Global settings cap both concurrency and total output consumption.

Reservations prevent concurrent requests from each observing the same remaining
budget. Database constraints and transactions provide the durable barrier.

## Control, audit, and asynchronous work ERD

```mermaid
erDiagram
    RUNTIME_SETTINGS {
        text key PK
        text value_kind
        jsonb value
        int version
        uuid updated_by_user_id FK
        timestamptz updated_at
    }

    ADMIN_AUDIT_LOG {
        uuid id PK
        uuid actor_user_id FK
        text action
        text target_type
        text target_id
        jsonb before_value
        jsonb after_value
        text request_id
        timestamptz occurred_at
    }

    OUTBOX_EVENTS {
        uuid id PK
        text event_type
        jsonb payload
        text idempotency_key UK
        int attempts
        timestamptz available_at
        timestamptz locked_at
        text locked_by
        timestamptz processed_at
        timestamptz dead_lettered_at
        text last_error_class
        timestamptz created_at
    }

    OUTBOX_RECEIPTS {
        uuid event_id PK,FK
        text handler_name PK
        timestamptz completed_at
    }

    USERS o|--o{ RUNTIME_SETTINGS : updates
    USERS o|--o{ ADMIN_AUDIT_LOG : acts
    OUTBOX_EVENTS ||--o{ OUTBOX_RECEIPTS : handled
```

### Runtime-setting invariants

- Key names follow a constrained namespace.
- `value_kind` and the JSON value type must agree.
- version increments support optimistic concurrency.
- the updater reference may become null after account deletion while the setting
  remains.

Current keys include maintenance, guest/user limits, global concurrency/output
budget, the system prompt, and configured safety categories.

### Audit invariants

Audit events retain actor, action, target, before/after state, request correlation,
and time. Reads redact sensitive fields. Conversation content and secrets do not
belong in audit values.

### Outbox invariants

- Event type follows a stable namespaced format.
- Idempotency key is globally unique.
- processed and dead-lettered states are mutually exclusive.
- claim indexes ignore terminal rows.
- locks expire so another worker can recover abandoned work.
- per-handler receipts prevent an already completed effect from running again.

## Schema evolution

Migrations are append-only after release and include a checksum table to detect
edited historical migration content. Generated sqlc code is committed and checked
for drift.

Production changes follow expand/migrate/contract:

1. add backward-compatible structures;
2. deploy code that can read/write both states where required;
3. migrate or backfill data under observation;
4. switch all readers/writers;
5. remove obsolete structures in a later release.

Production migrations run as one pre-deploy job, not concurrently in every API
replica.

## Data lifecycle and privacy

| Data                       | Lifecycle                                                    |
| -------------------------- | ------------------------------------------------------------ |
| Guest session/conversation | expires unless migrated; worker removes unmigrated data      |
| User conversation/messages | retained until account/conversation deletion policy applies  |
| Auth refresh token hashes  | expire or are removed with session/user                      |
| OAuth/WS ticket state      | short TTL in Redis; not in PostgreSQL ERD                    |
| Generation and usage       | retained for user experience and accountable aggregate usage |
| Operational logs/traces    | no prompt/response bodies; retention selected in M6          |
| Audit records              | retained/redacted according to production policy             |
| Account deletion job       | durable until completed/operational retention expires        |
| Anonymous aggregates       | may remain when permitted and no longer identify the user    |

Final production retention periods, backup deletion behavior, and legal basis are
M6 decisions and must match the approved Privacy Policy.

## Backup and recovery expectations

The local Compose database is not a production durability design. Production
requires:

- managed encrypted storage and backups;
- point-in-time recovery appropriate to the selected provider;
- documented RPO/RTO;
- restore into an isolated environment;
- application-level integrity checks after restore;
- credential rotation and access audit;
- a successful restore drill before launch and on a recurring schedule.
