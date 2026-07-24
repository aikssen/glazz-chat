# Glazz Architecture

> This is the canonical detailed architecture specification. For review-oriented
> Mermaid context, container, component, sequence, and deployment diagrams, see
> [`docs/technical-architecture.md`](./docs/technical-architecture.md). For the
> relational model, see [`docs/data-model.md`](./docs/data-model.md).

## 1. Purpose

This document is the technical source of truth for the Glazz MVP. It defines
system boundaries, contracts, data ownership, security, realtime behavior, and
deployment constraints. Product scope is defined in [PROJECT.md](./PROJECT.md).

Architecture principles:

1. API-first: describe and test contracts before implementing handlers or screens.
2. Provider-neutral domain: upstream LLM details stop at an adapter boundary.
3. Vertical slices: code is organized by user capability, not framework layer.
4. Durable before realtime: acknowledged business state is stored in PostgreSQL.
5. Explicit failure: every asynchronous operation has a visible state and retry
   policy.
6. Secure defaults: deny by default, short-lived credentials, auditable admin
   actions, and no prompt bodies in logs.
7. Replaceable infrastructure: Redis, provider, and hosting integrations are behind
   narrow interfaces.
8. Current dependencies: resolve APIs and stable versions with Context7, verify
   against upstream primary documentation, then pin exact versions.

## 2. System context

```text
                    +----------------------+
                    | Google OAuth / OIDC  |
                    +----------+-----------+
                               |
+-------------+     HTTPS/WSS  v     +-----------------------+
| Browser/PWA | <------------------> | Go API + Worker       |
| Next.js     |                       | HTTP, WebSocket, jobs |
+------+------+                       +---+--------+----------+
       |                                  |        |
       | SSR/RSC                          |        +----------+
       v                                  v                   v
+-------------+                    +-------------+      +-----------+
| Vercel      |                    | PostgreSQL  |      | Redis     |
+-------------+                    | source truth|      | ephemeral |
                                   +------+------+      +-----------+
                                          |
                                          | streamed provider API
                                          v
                                   +------------------+
                                   | LLM provider     |
                                   | dev: OpenCode Go |
                                   | prod: approved   |
                                   +------------------+
```

The Next.js application never calls an LLM provider directly. The Go API owns
authentication, authorization, quotas, safety checks, model selection, persistence,
and provider credentials.

## 3. Monorepo layout

```text
.
|-- apps/
|   |-- api/
|   |   |-- cmd/api/
|   |   |-- cmd/worker/
|   |   |-- internal/
|   |   |   |-- platform/
|   |   |   |-- identity/
|   |   |   |-- guests/
|   |   |   |-- conversations/
|   |   |   |-- chat/
|   |   |   |-- models/
|   |   |   |-- quotas/
|   |   |   |-- admin/
|   |   |   `-- privacy/
|   |   |-- migrations/
|   |   |-- queries/
|   |   `-- go.mod
|   `-- web/
|       |-- src/app/
|       |-- src/features/
|       |-- src/components/
|       |-- src/lib/
|       |-- src/messages/
|       `-- package.json
|-- packages/
|   `-- contracts/
|       |-- openapi.yaml
|       |-- websocket.asyncapi.yaml
|       |-- generated/typescript/
|       `-- fixtures/
|-- deploy/
|   |-- compose.yaml
|   |-- docker/
|   `-- observability/
|-- docs/
|   |-- adr/
|   |-- runbooks/
|   `-- threat-model/
|-- scripts/
|-- PROJECT.md
|-- ARCHITECTURE.md
|-- DESIGN.md
|-- AGENTS.md
`-- TASKS.md
```

`packages/contracts` contains source contracts, generated client types, and shared
fixtures. It contains no runtime business logic shared between TypeScript and Go.

## 4. Runtime components

### 4.1 Next.js web

- App Router with Server Components by default.
- Client Components only for interactive chat, WebSocket state, composer, dialogs,
  and browser-only APIs.
- Server-side locale and session-aware shell rendering.
- Generated typed HTTP client from OpenAPI.
- A handwritten, contract-tested WebSocket client around generated event types.
- No provider keys, refresh-token logic, or authorization decisions.
- PWA shell caches static assets only; chat sending requires a live connection.

### 4.2 Go API

One deployable API binary exposes REST and WebSocket traffic. Features are vertical
slices with their own transport, application service, domain types, repository
ports, and tests. Shared platform packages are limited to cross-cutting concerns
such as configuration, database transactions, telemetry, clocks, identifiers, and
HTTP middleware.

Dependency injection is explicit through constructors. Package globals may contain
constants only. Time, randomness, token signing, providers, repositories, and
external clients are injected.

### 4.3 Worker

The worker uses the same feature packages but runs separate commands:

- Delete expired guest conversations daily.
- Complete account deletion within 24 hours.
- Synchronize provider model metadata.
- Aggregate non-identifying usage metrics.
- Process transactional outbox events.
- Expire stale generations and sessions.

Jobs use PostgreSQL-backed durable records and advisory locks. Redis is not the
only record of job completion.

### 4.4 PostgreSQL

PostgreSQL owns users, identities, sessions, conversations, messages, generations,
summaries, quotas, configuration, audits, deletion jobs, model catalog, usage, and
outbox records. Use `pgx`, `sqlc`, SQL migrations with `goose`, transactions, and
database constraints. Application-side checks never replace ownership, uniqueness,
or state constraints that PostgreSQL can enforce.

### 4.5 Redis

Redis owns only reconstructible, short-lived coordination:

- Rate-limit buckets
- Global and per-user concurrency leases
- WebSocket connection registry
- Cross-instance pub/sub
- Cached configuration/model reads
- One-time WebSocket tickets

All keys have TTLs. Loss of Redis degrades or temporarily blocks work but does not
lose committed conversations.

### 4.6 LLM provider gateway

The MVP adapter protocol is OpenAI-compatible Chat Completions. OpenCode Go is used
only in development. Production uses a separately approved provider selected by
configuration.

```go
type ChatProvider interface {
    StreamChat(ctx context.Context, request ChatRequest) (ChatStream, error)
    ListModels(ctx context.Context) ([]ProviderModel, error)
    Health(ctx context.Context) error
}

type ChatStream interface {
    Recv() (ChatChunk, error)
    Close() error
}
```

Domain types use internal model IDs. The adapter maps internal IDs to upstream IDs,
normalizes chunks and usage, applies timeouts, classifies errors, and strips
provider-specific payloads before returning.

Required resilience:

- Connect, first-byte, idle-stream, and total timeouts
- Exponential backoff with jitter only before streaming starts
- No automatic replay after a partial response
- Circuit breaker per provider
- Global concurrency and provider-neutral output-token budget controls
- Exact currency spend limits after production provider pricing is approved
- Cancellation propagated from WebSocket to upstream request
- Metrics tagged by internal model/provider IDs, never prompt content

## 5. Backend slices

| Slice           | Owns                                                                            |
| --------------- | ------------------------------------------------------------------------------- |
| `identity`      | Google callback, users, identities, JWTs, refresh rotation, device sessions     |
| `guests`        | Anonymous cookie identity, limits, retention, migration ownership               |
| `conversations` | CRUD, archive/search, ownership, message pagination                             |
| `chat`          | Generation lifecycle, WebSocket commands/events, cancellation, retry, summaries |
| `models`        | Internal catalog, provider mapping, supported capabilities, sync                |
| `quotas`        | Daily usage, concurrency, guest allowance, global budgets                       |
| `admin`         | Runtime settings, user roles, audits, usage views, maintenance                  |
| `privacy`       | Terms consent, account deletion, data purge                                     |
| `platform`      | Config, DB, Redis, telemetry, HTTP, IDs, clock, outbox                          |

Slices communicate through explicit application interfaces or domain events, not
by importing another slice's persistence implementation.

## 6. Frontend slices

| Slice           | Owns                                                             |
| --------------- | ---------------------------------------------------------------- |
| `auth`          | User/session state, Google login entry, logout, reauthentication |
| `guest`         | Anonymous allowance display and login conversion gate            |
| `conversations` | Sidebar list, search, rename, archive, delete                    |
| `chat`          | Transcript, composer, streaming state, cancel, retry             |
| `models`        | Model selector and capability display                            |
| `settings`      | Theme, language, sessions, account deletion                      |
| `admin`         | Models, quotas, prompts, maintenance, audit and usage views      |
| `realtime`      | Connection state, reconnect, event ordering, subscriptions       |

Route segments compose features. They do not become a second backend and do not
duplicate authorization logic.

## 7. Data model

All primary keys are UUIDv7 generated through an injected ID source. Timestamps are
UTC `timestamptz`. User-visible ordering uses `(created_at, id)`.

### 7.1 Identity

| Table               | Important fields and constraints                                                                                     |
| ------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `users`             | `id`, `email`, `display_name`, `avatar_url`, `locale`, `role`, `plan`, `status`, timestamps; unique normalized email |
| `user_identities`   | `user_id`, `provider`, `provider_subject`, verified email; unique `(provider, provider_subject)`                     |
| `auth_sessions`     | hashed refresh token family/current token, device metadata, expiry, revoked/reuse timestamps                         |
| `terms_acceptances` | user, terms/privacy versions, accepted timestamp, IP hash                                                            |

Roles are `user` and `admin`; plan is independently `free`, `pro`, or future values.
Authorization checks role, not plan labels.

### 7.2 Guests and ownership

| Table            | Important fields and constraints                                                                       |
| ---------------- | ------------------------------------------------------------------------------------------------------ |
| `guest_sessions` | signed-cookie public ID hash, first/last seen, prompt count, output token count, expiry, migrated user |
| `conversations`  | nullable `user_id` xor `guest_session_id`, title, model, status, summary pointer, timestamps           |

A database check guarantees exactly one owner. Migration locks the guest row and
conversation, assigns the user, records migration, and commits in one transaction.

### 7.3 Chat

| Table                    | Important fields and constraints                                                                           |
| ------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `messages`               | conversation, role, sanitized content, status, sequence, token estimates, timestamps                       |
| `generations`            | conversation, user message, assistant message, model, provider, state, idempotency key, usage, error class |
| `conversation_summaries` | conversation, version, covered message sequence, content, model, token count                               |

Message roles for the MVP are `user` and `assistant`. The effective system prompt
and summaries are not inserted as user-visible messages.

Generation states:

```text
accepted -> streaming -> completed
    |           |-> cancelled
    |           `-> failed
    `-> rejected
```

Only one `accepted` or `streaming` generation may exist per conversation. A partial
assistant message may be retained with `cancelled` or `failed` status and is not
automatically included in subsequent context.

### 7.4 Models, configuration, and usage

| Table              | Important fields and constraints                                                            |
| ------------------ | ------------------------------------------------------------------------------------------- |
| `providers`        | internal key, adapter kind, enabled, encrypted secret reference, health                     |
| `models`           | internal ID, display name, context/output limits, capabilities, lifecycle                   |
| `provider_models`  | provider, model, upstream ID, price metadata, sync state                                    |
| `model_exposure`   | model, audience, enabled, ordering, effective limits                                        |
| `runtime_settings` | typed key, JSON value, version, updated by                                                  |
| `usage_ledger`     | actor, conversation, generation, model/provider, input/output/cached tokens, estimated cost |
| `daily_usage`      | aggregate counters by actor/date                                                            |
| `admin_audit_log`  | actor, action, target, redacted before/after, request metadata                              |
| `outbox_events`    | event type, payload, attempts, availability, processed timestamp                            |

Provider sync updates metadata and marks missing models unavailable. It never
enables a model for users.

## 8. HTTP API contract

### 8.1 Conventions

- Base path: `/api/v1`
- JSON uses `camelCase`; timestamps use RFC 3339 UTC.
- IDs are opaque strings.
- Mutations accept `Idempotency-Key` where duplication has business impact.
- Request correlation uses `X-Request-ID`; the server creates one if absent.
- Cursor pagination uses `?limit=&after=` with a stable opaque cursor.
- `Cache-Control: no-store` on authenticated and guest-personal responses.
- OpenAPI 3.1 is the source of truth.
- Breaking changes require `/api/v2`; additive fields are allowed in v1.

Error envelope:

```json
{
  "error": {
    "code": "quota_exceeded",
    "message": "Daily message limit reached.",
    "requestId": "req_...",
    "details": {
      "resetAt": "2026-07-24T05:00:00Z"
    }
  }
}
```

Messages are localized by the client from stable error codes. Server messages are
safe English fallbacks. Validation errors identify fields without echoing secrets
or prompt content.

### 8.2 Public and runtime

| Method | Path             | Purpose                                                                               |
| ------ | ---------------- | ------------------------------------------------------------------------------------- |
| `GET`  | `/health/live`   | Process liveness; no dependency checks                                                |
| `GET`  | `/health/ready`  | Database/Redis readiness; provider status is reported but does not block admin access |
| `GET`  | `/config/public` | Locale, maintenance, legal versions, guest policy, feature flags                      |
| `GET`  | `/models`        | Enabled models for the current actor                                                  |

### 8.3 Guest

| Method | Path                      | Purpose                            |
| ------ | ------------------------- | ---------------------------------- |
| `POST` | `/guest-sessions`         | Create/resume signed guest session |
| `GET`  | `/guest-sessions/current` | Remaining allowance and expiry     |

The raw guest identifier is never returned. Cookies are signed, rotated when
appropriate, and scoped to the API domain.

### 8.4 Authentication and sessions

| Method   | Path                       | Purpose                                                           |
| -------- | -------------------------- | ----------------------------------------------------------------- |
| `GET`    | `/auth/google/start`       | Create OAuth state/PKCE and redirect to Google                    |
| `GET`    | `/auth/google/callback`    | Validate callback, login, migrate guest conversation, set cookies |
| `POST`   | `/auth/refresh`            | Rotate refresh token and issue access token                       |
| `POST`   | `/auth/logout`             | Revoke current session and clear cookies                          |
| `GET`    | `/me`                      | Current user and effective permissions                            |
| `GET`    | `/me/sessions`             | List active device sessions                                       |
| `DELETE` | `/me/sessions/{sessionId}` | Revoke one device session                                         |
| `POST`   | `/me/reauthenticate`       | Start recent-auth flow for destructive action                     |
| `DELETE` | `/me`                      | Request account deletion                                          |
| `GET`    | `/me/deletion`             | Read deletion-job state                                           |
| `POST`   | `/auth/ws-ticket`          | Issue single-use, short-lived WebSocket ticket                    |

OAuth state is server-held and bound to the browser session. Redirect targets are
allowlisted. The callback never accepts an arbitrary return URL.

### 8.5 Conversations and messages

| Method   | Path                                       | Purpose                                             |
| -------- | ------------------------------------------ | --------------------------------------------------- |
| `GET`    | `/conversations`                           | List/search/filter active or archived conversations |
| `POST`   | `/conversations`                           | Create conversation; guests are restricted to one   |
| `GET`    | `/conversations/{conversationId}`          | Conversation metadata                               |
| `PATCH`  | `/conversations/{conversationId}`          | Rename, archive/unarchive, change model when idle   |
| `DELETE` | `/conversations/{conversationId}`          | Delete owned conversation                           |
| `GET`    | `/conversations/{conversationId}/messages` | Cursor-paginated transcript                         |
| `POST`   | `/conversations/{conversationId}/retry`    | Retry latest failed/cancelled generation            |
| `GET`    | `/usage`                                   | Effective limits, usage, reset time                 |

Chat generation is commanded over WebSocket. Conversation creation and metadata
remain REST resources so mobile and degraded clients can use stable semantics.

### 8.6 Administration

All endpoints require `admin`; changes require CSRF protection, recent auth for
high-impact operations, and audit logging.

| Method  | Path                         | Purpose                                              |
| ------- | ---------------------------- | ---------------------------------------------------- |
| `GET`   | `/admin/models`              | Internal catalog, provider mapping, exposure status  |
| `POST`  | `/admin/models/sync`         | Enqueue provider metadata synchronization            |
| `PATCH` | `/admin/models/{modelId}`    | Enable, disable, order, or limit a model             |
| `GET`   | `/admin/settings`            | Read typed runtime settings                          |
| `PATCH` | `/admin/settings/{key}`      | Update quota, prompt, safety, or maintenance setting |
| `GET`   | `/admin/users`               | Search users without conversation content            |
| `PATCH` | `/admin/users/{userId}/role` | Promote or demote user                               |
| `GET`   | `/admin/usage`               | Aggregate usage, latency, errors, and estimated cost |
| `GET`   | `/admin/audit-log`           | Cursor-paginated administrative audit trail          |

## 9. WebSocket protocol

### 9.1 Connection

1. Client obtains a single-use ticket from `POST /auth/ws-ticket`.
2. Client connects to `wss://api-host/api/v1/ws?ticket=<opaque>`.
3. Server validates ticket, `Origin`, actor, expiry, and maintenance state.
4. Server sends `connection.ready` with heartbeat and resume parameters.
5. Client reconnects with exponential backoff and requests resume from the last
   acknowledged sequence.

Tickets expire within 30 seconds, are stored hashed in Redis, and are consumed once.
This avoids durable credentials in URLs and supports future mobile clients.

Envelope:

```json
{
  "version": 1,
  "type": "chat.generate",
  "eventId": "evt_...",
  "requestId": "req_...",
  "sequence": 42,
  "occurredAt": "2026-07-23T20:00:00Z",
  "payload": {}
}
```

`eventId` deduplicates commands. `sequence` orders server events within a
connection/resume stream. Unknown additive fields are ignored; unknown event types
are reported and ignored unless the major protocol version is unsupported.

### 9.2 Client commands

| Event               | Payload                                   | Result                                                |
| ------------------- | ----------------------------------------- | ----------------------------------------------------- |
| `connection.resume` | last server sequence                      | Replays buffered events or returns resync requirement |
| `chat.generate`     | conversation ID, content, idempotency key | Validates, persists user message, starts generation   |
| `chat.cancel`       | conversation ID, generation ID            | Cancels owned active generation                       |
| `heartbeat.pong`    | heartbeat ID                              | Keeps connection alive                                |

Content limits are enforced before persistence. A command is acknowledged only
after its durable state is committed.

### 9.3 Server events

| Event                        | Meaning                                              |
| ---------------------------- | ---------------------------------------------------- |
| `connection.ready`           | Authenticated actor, heartbeat interval, server time |
| `command.acknowledged`       | Command accepted and committed                       |
| `command.rejected`           | Stable error code and safe details                   |
| `chat.started`               | Generation and assistant message IDs                 |
| `chat.delta`                 | Ordered text delta and offset                        |
| `chat.completed`             | Final status, usage, finish reason                   |
| `chat.cancelled`             | Cancellation confirmed, partial content status       |
| `chat.failed`                | Retryable flag and normalized error code             |
| `conversation.updated`       | Metadata changed, including generated title          |
| `heartbeat.ping`             | Connection liveness probe                            |
| `connection.resync_required` | Buffer missed; refetch REST resources                |

Deltas are transient but the server periodically checkpoints the assistant message
and always writes terminal state. On reconnect, REST is authoritative if event
replay is unavailable.

### 9.4 Backpressure and ordering

- One active generation per actor and conversation in the MVP.
- Each connection has bounded outbound buffers.
- Slow clients are disconnected with a retryable reason before unbounded memory
  growth.
- Deltas carry monotonically increasing offsets; duplicate or stale deltas are
  ignored by the client.
- Pub/sub fans events across API replicas; database state resolves conflicts.

## 10. Critical flows

### 10.1 Generate response

```text
Client       WebSocket       Chat service      PostgreSQL     Provider
  | generate     |                |                 |             |
  |------------->| validate actor |                 |             |
  |              |--------------->| quota + safety  |             |
  |              |                |-- transaction ->|             |
  |<-------------| acknowledged   |                 |             |
  |              |                |---------------->| stream      |
  |<-------------| started/deltas |<----------------|             |
  |              |                |-- checkpoints ->|             |
  |<-------------| completed      |-- final usage ->|             |
```

The transaction creates the user message, assistant placeholder, generation, quota
reservation, and outbox event. Finalization reconciles reserved and actual usage.

### 10.2 Guest migration

The Google callback locks the guest session and conversation, creates or resolves
the user, checks that the conversation was not previously migrated, replaces guest
ownership with user ownership, records the migration, creates the auth session,
and commits. Replaying the callback returns the same ownership result without
duplicating messages.

### 10.3 Context summarization

Before generation, the context builder estimates tokens using model metadata. At
70% of the effective window it schedules or performs summarization under a lock.
The summary covers a contiguous message sequence. Recent messages remain verbatim.
A generation records the summary version it used for reproducibility.

## 11. Security architecture

### 11.1 Browser security

- HTTPS and WSS only outside local development.
- Strict allowlist for CORS and WebSocket `Origin`.
- `Secure`, `HttpOnly`, `SameSite=Lax` cookies.
- CSRF token on cookie-authenticated unsafe HTTP methods.
- CSP with nonces/hashes and no arbitrary model HTML execution.
- HSTS, `X-Content-Type-Options`, restrictive `Permissions-Policy`, and safe
  referrer policy.
- Markdown sanitization; no raw HTML; safe external-link attributes.
- No JWT, OAuth code, provider key, or refresh token in local storage or logs.

### 11.2 Token security

- Asymmetric access JWT signing with `kid` rotation.
- Validate issuer, audience, expiry, not-before, session ID, and token version.
- Refresh tokens are opaque random secrets; only strong hashes are stored.
- Rotation is atomic. Reuse revokes the token family and alerts telemetry.
- Authorization reloads critical role/session state where immediate revocation is
  required.

### 11.3 Abuse and content safety

- Layered guest and registered-user rate limits by actor and privacy-preserving IP
  hash.
- Payload, token, connection, and generation duration limits.
- Input/output policy interfaces independent of provider.
- Configurable safety categories and report-response workflow.
- System prompt is guidance, not an authorization or safety boundary.
- Global provider output-token budgets and concurrency circuit breakers.
- Exact currency spend enforcement uses approved provider pricing in Phase 10.

### 11.4 Secrets

- `.env` is local-only and gitignored.
- Production secrets come from the hosting secret manager.
- Config validation fails startup on missing or malformed required values.
- Admin APIs never return secret values.
- Provider and OAuth keys are independently rotatable.

## 12. Configuration

Environment variables configure deployment wiring, not routine product policy.
Routine settings live in typed, versioned database records and are edited through
admin APIs.

Required configuration groups:

- Runtime: environment, ports, public web/API URLs, trusted proxies
- PostgreSQL and Redis connections
- Google OAuth client, secret, callback, allowed domains
- JWT issuer/audience/signing key references
- Cookie domains and security flags
- Provider adapter, base URL, API key reference
- Bootstrap admin emails
- Telemetry exporters and log level

Development aliases `API_URL` and `API_KEY` may be mapped into the provider config
during migration, but final names must be explicit, such as
`LLM_PROVIDER_BASE_URL` and `LLM_PROVIDER_API_KEY`.

## 13. Observability

Every request, WebSocket command, generation, and job carries correlation IDs.

Logs:

- Structured JSON
- Actor IDs hashed or internal; no emails by default
- No prompts, responses, OAuth tokens, cookies, or provider payloads
- Error class, duration, state transition, and request ID

Metrics:

- HTTP latency/status and active requests
- WebSocket connections, reconnects, buffer pressure, command rejection
- Provider connect/first-token/total latency, token counts, errors, circuit state
- Quota denials and Redis limiter health
- Database pool, query latency, outbox lag, job age
- Auth success/failure, refresh reuse, account deletion SLA

Traces cross HTTP/WebSocket, application service, database, Redis, and provider
calls without recording message content.

## 14. Testing strategy

| Layer                  | Required coverage                                                                  |
| ---------------------- | ---------------------------------------------------------------------------------- |
| Unit                   | Domain policies, state machines, quota calculations, token/context logic           |
| Repository integration | Real PostgreSQL/Redis through containers; constraints and transactions             |
| Contract               | OpenAPI validation, generated client compatibility, WebSocket fixtures             |
| Provider adapter       | Recorded synthetic streams and error cases; no secret-dependent unit tests         |
| Handler                | Authn/authz, validation, idempotency, error mapping                                |
| Race/concurrency       | `go test -race`, generation locks, refresh rotation, guest migration               |
| Frontend component     | Rendering, keyboard, accessibility, streaming reducers                             |
| E2E                    | Guest limit/login migration, registered chat, cancellation, retry, admin, deletion |
| Resilience             | Provider timeout, Redis loss, reconnect, duplicate commands, worker retry          |
| Security               | Dependency scans, secret scan, SAST, auth abuse cases, OWASP-focused tests         |
| Performance            | WebSocket fan-out, message pagination, p95 first-token and API targets             |

Tests use injected clock/ID/provider dependencies and deterministic fixtures.
Coverage thresholds are not a substitute for behavior-focused critical-path tests.

## 15. Deployment

Local Docker Compose runs web, API, worker, PostgreSQL, Redis, and optional
observability services. Application processes support graceful shutdown and
readiness gates.

Production:

- Web begins on Vercel.
- API and worker use the same immutable container image with different commands.
- API replicas require sticky connections only if the chosen WebSocket library
  cannot resume through Redis; correctness cannot depend on stickiness.
- Managed PostgreSQL and Redis use private networking where available.
- Migrations run as a single pre-deploy job, never on every replica startup.
- Deployments are rolling with backward-compatible schema changes.
- Database changes use expand/migrate/contract sequencing.

Backups are off locally. Production launch requires configured backups, restore
instructions, and a successful restore drill.

## 16. Architecture decision status

Recorded:

1. [HTTP router and WebSocket library](./docs/adr/0001-http-router-and-websocket.md)
2. [OpenAPI and AsyncAPI generation toolchain](./docs/adr/0002-contract-toolchain.md)
3. [JWT signing and key rotation mechanism](./docs/adr/0003-jwt-signing-and-rotation.md)
4. [Goroutine leak detection](./docs/adr/0004-goroutine-leak-detection.md)

ADRs still required before the corresponding production gate:

1. PostgreSQL job/outbox evolution if the current implementation changes
2. Production provider adapter and approved LLM provider
3. Production API, PostgreSQL, and Redis hosting
4. Hosted error tracking and telemetry retention
5. Backup, point-in-time recovery, and restore policy

## 17. External technical references

- Next.js App Router: <https://nextjs.org/docs/app>
- Tailwind CSS with Next.js: <https://tailwindcss.com/docs/installation/framework-guides/nextjs>
- Go release policy: <https://go.dev/doc/devel/release>
- PostgreSQL current documentation: <https://www.postgresql.org/docs/current/>
- OpenCode Go API: <https://opencode.ai/docs/go/>
- Context7 CLI workflow: `.agents/skills/context7-cli/SKILL.md`
