# Glazz Delivery Plan

## How to use this plan

This is the ordered delivery plan for the Glazz MVP. Work begins with the API
description, then backend foundations and slices, then frontend implementation,
integration, verification, and production release.

Status:

- `[ ]` Not started
- `[-]` In progress
- `[x]` Complete
- `[!]` Blocked; record the blocker directly below the task

Rules:

1. Do not mark a task complete until its acceptance criteria pass.
2. Respect dependencies shown in `Depends on`.
3. Keep contract, implementation, tests, and documentation in the same change where
   practical.
4. Close phase exit criteria before starting work that depends on that phase.
5. Exact dependency versions are selected and pinned during foundation tasks.

## Planning model

- A **milestone** is a releasable product outcome. It owns one or more phases and,
  from M2 onward, closes with an annotated minor-version tag.
- A **phase** is an ordered delivery segment inside exactly one milestone. Completing
  a phase does not close its milestone when that milestone owns additional phases.
- A **task** is complete only when its implementation and acceptance criteria pass.
- Milestone progress and release evidence live in the linked milestone progress
  document. `TASKS.md` remains the canonical scope and task-status index.
- `WORKLOG.md` is chronological evidence only and must never redefine task status.

## Milestones

| Milestone | Owned phases | Outcome                                          | Implementation | Acceptance     | Release                             |
| --------- | ------------ | ------------------------------------------------ | -------------- | -------------- | ----------------------------------- |
| M0        | Phase 0      | Reviewed contracts and threat model              | Complete       | Complete       | `v0.1.0`                            |
| M1        | Phase 1      | Reproducible monorepo foundation                 | Complete       | Complete       | No standalone tag; predates M2 rule |
| M2        | Phases 2-3   | Local platform, identity, guests, and quotas     | Complete       | Complete       | `v0.2.0`                            |
| M3        | Phases 4-5   | Provider-neutral streamed chat backend           | Complete       | Complete       | `v0.3.0`                            |
| M4        | Phases 6-8   | Responsive web application and admin surface     | Complete       | Complete       | `v0.4.0`                            |
| M5        | Phase 9      | Integrated, tested, observable release candidate | In progress    | Open           | `v0.5.0` reserved                   |
| M6        | Phase 10     | Approved provider and production deployment      | Not started    | Open           | `v0.6.0` reserved                   |
| Post-MVP  | Phase 11     | Explicitly non-release-critical backlog          | Not started    | Not applicable | Future planning                     |

Current progress: M4 is published as `v0.4.0`. Phases 6-8 and M4 acceptance are
complete.
Granular acceptance evidence is tracked in `docs/m3-chat-backend.md`,
`docs/m4-web-application.md`, and `docs/m5-release-candidate.md`. A `[-]` task
means its implementation exists but at least one acceptance criterion is still
pending; it does not mean the feature is unavailable. M5 and Phase 9 are in
progress with their functional test gates accepted; resilience, security, visual,
privacy, performance, and contract review remain open.

## Phase 0: API and architecture contract

**Milestone owner:** M0

**Objective:** Turn the product decisions into reviewable, executable contracts
before implementing API or UI behavior.

### 0.1 Contract foundations

- [x] **API-001: Create API terminology and resource glossary**
  - Depends on: planning documents
  - Define actor, guest session, auth session, conversation, message, generation,
    model, provider, quota, usage, deletion job, and audit event.
  - Acceptance: terms match `PROJECT.md` and `ARCHITECTURE.md`; no provider-specific
    term appears in public resources.

- [x] **API-002: Write OpenAPI 3.1 skeleton**
  - Depends on: API-001
  - Create server definitions, `/api/v1`, tags, security schemes, common headers,
    pagination, and error envelope.
  - Acceptance: specification validates with zero errors; examples cover one success
    and one error.

- [x] **API-003: Define public/config/model endpoints**
  - Depends on: API-002
  - Describe liveness, readiness, public config, and visible model catalog.
  - Acceptance: cache/security behavior and maintenance response are explicit.

- [x] **API-004: Define guest-session endpoints and schemas**
  - Depends on: API-002
  - Describe create/resume and current allowance.
  - Acceptance: cookie behavior, four-message/2,000-token policy, expiry, and errors
    are represented without exposing a raw guest ID.

- [x] **API-005: Define Google auth and session endpoints**
  - Depends on: API-002
  - Describe start, callback, refresh, logout, current user, sessions, revocation,
    reauthentication, and WebSocket ticket.
  - Acceptance: OAuth redirects, cookies, CSRF, rotation, and reuse errors have
    examples; arbitrary return URLs are impossible.

- [x] **API-006: Define conversation and message endpoints**
  - Depends on: API-002
  - Describe create/list/search/get/rename/archive/delete, message pagination, and
    retry.
  - Acceptance: owner rules, guest restrictions, idle-only model change, cursors,
    idempotency, and terminal generation states are explicit.

- [x] **API-007: Define usage and quota schemas**
  - Depends on: API-002
  - Describe effective limits, remaining usage, reset time, and global-service
    rejection behavior.
  - Acceptance: guest, user, concurrency, and provider-budget error cases exist.

- [x] **API-008: Define account deletion contract**
  - Depends on: API-005
  - Describe recent-auth precondition, deletion request, status, and terminal state.
  - Acceptance: immediate revocation and 24-hour deletion SLA are documented.

- [x] **API-009: Define admin endpoints**
  - Depends on: API-003, API-007
  - Describe model sync/exposure, runtime settings, users/roles, usage, and audit log.
  - Acceptance: admin/recent-auth requirements, redaction, optimistic versioning,
    and audit effects are explicit.

### 0.2 WebSocket contract

- [x] **WS-001: Create AsyncAPI/WebSocket schema**
  - Depends on: API-002
  - Define v1 envelope, connection URL, ticket, event ID, request ID, sequence, and
    timestamp.
  - Acceptance: schema validates and generated/event types can represent every event
    in `ARCHITECTURE.md`.

- [x] **WS-002: Define connection lifecycle events**
  - Depends on: WS-001, API-005
  - Define ready, resume, resync, heartbeat, maintenance, close reasons.
  - Acceptance: reconnect after missed buffer has a deterministic REST fallback.

- [x] **WS-003: Define generation commands and events**
  - Depends on: WS-001, API-006, API-007
  - Define generate, cancel, acknowledgement, rejection, start, delta, completion,
    cancellation, failure, quota, and conversation update.
  - Acceptance: duplicate commands, delta offsets, partial failure, and cancellation
    examples exist.

- [x] **WS-004: Publish canonical protocol fixtures**
  - Depends on: WS-002, WS-003
  - Add valid JSON fixtures for guest success, user success, cancel, quota, reconnect,
    and retryable provider failure.
  - Acceptance: fixtures are syntax/baseline validated in M0; generated schema
    validation is wired into CI by FOUND-006 without changing the fixtures.

### 0.3 Architecture and security decisions

- [x] **ADR-001: Select Go HTTP router and WebSocket library**
  - Depends on: WS-003
  - Evaluate standard-library compatibility, context cancellation, origin controls,
    backpressure, maintenance, and testability.
  - Acceptance: ADR includes rejected alternatives and load-test implications.

- [x] **ADR-002: Select contract generation/validation toolchain**
  - Depends on: API-002, WS-001
  - Select OpenAPI/AsyncAPI validators and Go/TypeScript generation.
  - Acceptance: generation is deterministic and generated files require no manual
    edits.

- [x] **ADR-003: Select JWT signing/key rotation mechanism**
  - Depends on: API-005
  - Choose algorithm, key storage, `kid` publication/rotation, and emergency revoke.
  - Acceptance: ADR covers local, CI, and production key handling.

- [x] **SEC-001: Produce initial threat model**
  - Depends on: API-004 through API-009, WS-003
  - Cover OAuth, CSRF, JWT theft, refresh reuse, WebSocket hijacking, IDOR, guest
    bypass, prompt injection, Markdown XSS, admin abuse, provider leakage, and
    deletion races.
  - Acceptance: each threat has an owner, mitigation, and verification task.

- [x] **API-010: Review and freeze MVP contract baseline**
  - Depends on: all Phase 0 tasks
  - Conduct product, frontend, backend, security, and mobile-future review.
  - Acceptance: M0 contract version and review created; unresolved items are
    explicitly nonblocking or moved to the release gate. The Git tag is created by
    FOUND-001 after repository initialization.

**Phase 0 exit:** M0 complete; API/WS fixtures validate; security review approves
implementation start.

## Phase 1: Monorepo and developer foundation

**Milestone owner:** M1

**Objective:** Create a reproducible workspace with pinned tools and fast feedback.

- [x] **FOUND-001: Create monorepo directories and root tooling**
  - Depends on: API-010
  - Create `apps`, `packages`, `deploy`, `docs`, and `scripts` layout.
  - Acceptance: repository tree matches architecture; root commands are documented;
    tag `v0.1.0` identifies the accepted M0 baseline.

- [x] **FOUND-002: Scaffold Next.js application**
  - Depends on: FOUND-001
  - Use App Router, TypeScript strict mode, ESLint, Tailwind CSS, and supported Node.
  - Acceptance: production build and type check pass; no example landing page remains.

- [x] **FOUND-003: Scaffold Go API and worker commands**
  - Depends on: FOUND-001
  - Pin a supported Go release, create composition roots, config package, graceful
    shutdown, and minimal liveness.
  - Acceptance: API/worker compile and `go test ./...` passes.

- [x] **FOUND-004: Configure shadcn/ui and design tokens**
  - Depends on: FOUND-002
  - Install required primitives and encode light/dark semantic tokens from
    `DESIGN.md`.
  - Acceptance: token showcase passes contrast checks in both themes.

- [x] **FOUND-005: Configure fonts and icons**
  - Depends on: FOUND-002
  - Add Outfit, Work Sans, JetBrains Mono through optimized/self-hosted loading and
    Lucide icons.
  - Acceptance: no layout shift from font loading; no emoji structural icons.

- [x] **FOUND-006: Implement contract validation and generation**
  - Depends on: ADR-002, FOUND-002, FOUND-003
  - Add repeatable commands for spec lint, fixture validation, and client generation.
  - Acceptance: running generation twice produces no diff.

- [x] **FOUND-007: Add repository quality commands**
  - Depends on: FOUND-002, FOUND-003
  - Add format, lint, type, unit, integration, race, contract, E2E, and build commands.
  - Acceptance: one documented root command runs the fast presubmit suite.

- [x] **FOUND-008: Add environment templates and validation**
  - Depends on: FOUND-002, FOUND-003
  - Create safe `.env.example` files; map temporary `API_URL`/`API_KEY` aliases to
    explicit provider configuration without exposing values.
  - Acceptance: startup fails with actionable errors; secrets are gitignored and
    secret scan passes.

- [x] **FOUND-009: Create CI baseline**
  - Depends on: FOUND-006, FOUND-007
  - GitHub Actions runs contract checks, Go/TS checks, unit tests, build, dependency
    and secret scans with caching.
  - Acceptance: pull-request workflow passes from a clean checkout.

**Phase 1 exit:** M1 application skeleton builds reproducibly in CI.

## Phase 2: Local infrastructure and platform services

**Milestone owner:** M2

**Objective:** Provide realistic local dependencies and shared operational plumbing.

- [x] **PLAT-001: Create Docker Compose development stack**
  - Depends on: FOUND-001, FOUND-003
  - Add PostgreSQL, Redis, API, worker, web, health checks, named volumes, and network.
  - Acceptance: one command starts a healthy stack; backups remain off locally.

- [x] **PLAT-002: Establish migration workflow**
  - Depends on: PLAT-001
  - Configure `goose`, migration verification, schema reset, and pre-deploy mode.
  - Acceptance: migrate from empty, rollback safe test migration, and detect drift.

- [x] **PLAT-003: Establish pgx/sqlc workflow**
  - Depends on: PLAT-002
  - Configure pools, transaction runner, query generation, health, and test helpers.
  - Acceptance: generated queries are deterministic and pool shutdown is graceful.

- [x] **PLAT-004: Implement Redis adapter**
  - Depends on: PLAT-001
  - Add health, namespaced TTL keys, rate-limit primitives, leases, tickets, and
    pub/sub interfaces.
  - Acceptance: integration tests cover expiry, contention, and unavailable Redis.

- [x] **PLAT-005: Implement structured config**
  - Depends on: FOUND-008
  - Parse/validate runtime, database, Redis, OAuth, JWT, cookie, provider, admin, and
    telemetry groups at composition root.
  - Acceptance: table tests cover invalid combinations and secret-safe errors.

- [x] **PLAT-006: Implement IDs, clock, and transaction abstractions**
  - Depends on: FOUND-003
  - Add UUIDv7 source, UTC clock, fake variants, and transaction interface.
  - Acceptance: deterministic tests demonstrate injectable behavior.

- [x] **PLAT-007: Implement HTTP middleware baseline**
  - Depends on: ADR-001, PLAT-005
  - Add request IDs, recovery, safe logging, timeouts, body limits, CORS, security
    headers, trusted proxy handling, and JSON errors.
  - Acceptance: middleware tests prove headers, redaction, and failure mapping.

- [x] **PLAT-008: Implement telemetry baseline**
  - Depends on: PLAT-005
  - Add OpenTelemetry traces, Prometheus metrics, JSON logs, and local exporters.
  - Acceptance: one synthetic request is correlated across log, metric, and trace
    without content leakage.

- [x] **PLAT-009: Implement transactional outbox and worker runner**
  - Depends on: PLAT-003, PLAT-006
  - Add durable jobs/events, claim/retry/backoff, advisory locks, dead-letter state,
    and graceful shutdown.
  - Acceptance: concurrent-worker tests prove at-least-once processing and idempotent
    completion.

## Phase 3: Identity, guests, and privacy foundations

**Milestone owner:** M2

**Objective:** Securely identify every actor before chat is implemented.

- [x] **AUTH-001: Create identity schema**
  - Depends on: PLAT-002, SEC-001
  - Add users, identities, sessions, terms acceptances, role/plan/status constraints.
  - Acceptance: migration and repository integration tests pass.

- [x] **AUTH-002: Implement JWT access tokens**
  - Depends on: ADR-003, AUTH-001, PLAT-005
  - Sign/verify issuer, audience, expiry, not-before, session, version, and `kid`.
  - Acceptance: tamper, wrong audience, expiry, rotation, and revoked-session tests
    pass.

- [x] **AUTH-003: Implement refresh rotation and reuse detection**
  - Depends on: AUTH-001, AUTH-002
  - Store only hashes; rotate atomically; revoke family on reuse.
  - Acceptance: concurrency/race tests allow one winner and revoke on replay.

- [x] **AUTH-004: Implement Google OAuth flow**
  - Depends on: AUTH-003, PLAT-007
  - Add state, PKCE, nonce, callback allowlist, verified identity, cookie issue.
  - Acceptance: success, denial, replay, state mismatch, duplicate email, and safe
    redirect E2E/integration cases pass.

- [x] **AUTH-005: Implement session management**
  - Depends on: AUTH-003
  - Add `/me`, list/revoke sessions, logout, recent-auth marker.
  - Acceptance: revocation takes effect promptly across HTTP and WebSocket.

- [x] **AUTH-006: Implement CSRF and browser cookie policy**
  - Depends on: AUTH-003, PLAT-007
  - Protect unsafe cookie-authenticated methods and configure environment-specific
    cookie domain/security.
  - Acceptance: cross-site and missing-token requests fail; valid same-site flow
    works.

- [x] **GUEST-001: Create guest schema and signed cookie**
  - Depends on: PLAT-002, PLAT-006
  - Add hashed public identity, allowance counters, expiry, and migration marker.
  - Acceptance: cookie tampering/rotation tests and no raw ID exposure.

- [x] **GUEST-002: Implement guest-session API**
  - Depends on: GUEST-001, PLAT-007
  - Create/resume and report remaining allowance.
  - Acceptance: idempotent resume, expired guest, and maintenance cases match API.

- [x] **QUOTA-001: Implement rate-limit and quota domain**
  - Depends on: GUEST-001, AUTH-001, PLAT-004
  - Add guest/user daily limits, output reservation, concurrency leases, IP hash,
    global budgets, and reset calculations.
  - Acceptance: boundary, timezone/UTC reset, concurrent, Redis-loss, and refund
    tests pass.

- [x] **GUEST-003: Implement transactional guest migration**
  - Depends on: AUTH-004, GUEST-002
  - Transfer ownership during callback with locking and idempotency.
  - Acceptance: callback replay and concurrent migration create no duplicate/lost
    data.

- [x] **PRIV-001: Implement terms/privacy acceptance**
  - Depends on: AUTH-004
  - Version legal documents and persist acceptance during account creation.
  - Acceptance: account creation cannot complete without current required consent.

- [x] **AUTH-007: Bootstrap first administrators**
  - Depends on: AUTH-004, PLAT-005
  - Promote allowlisted verified emails idempotently and audit the action.
  - Acceptance: non-allowlisted users remain standard users.

## Phase 4: Model catalog and provider gateway

**Milestone owner:** M3

**Milestone acceptance ledger:** `docs/m3-chat-backend.md`

**Objective:** Stream through a provider-neutral interface using a deterministic fake
and OpenCode Go only for development.

- [x] **MODEL-001: Create provider/model/configuration schema**
  - Depends on: PLAT-002
  - Milestone gate: M3-A01
  - Add providers, models, provider mapping, exposure, typed settings, and audit.
  - Acceptance: constraints prevent exposing unsupported/unavailable models.

- [x] **MODEL-002: Define provider-neutral domain types**
  - Depends on: API-010
  - Define requests, messages, chunks, usage, finish reasons, errors, and capabilities.
  - Acceptance: types contain no OpenCode/OpenAI SDK-specific structures.

- [x] **MODEL-003: Implement deterministic fake provider**
  - Depends on: MODEL-002
  - Support configurable chunks, latency, usage, cancellation, partial failure, and
    catalog.
  - Acceptance: tests and local E2E can run with no external API key.

- [x] **MODEL-004: Implement OpenAI-compatible adapter**
  - Depends on: MODEL-002, PLAT-005
  - Milestone gate: M3-A02
  - Add streamed Chat Completions, timeouts, normalized errors/usage, cancellation,
    and safe logging.
  - Acceptance: synthetic contract suite covers normal, malformed, rate-limited,
    timeout, disconnect, and partial stream.

- [x] **MODEL-005: Configure OpenCode Go development provider**
  - Depends on: MODEL-004, FOUND-008
  - Milestone gate: M3-A03
  - Use configured `/zen/go/v1`, map `deepseek-v4-flash`, and keep provider details
    out of domain/public APIs.
  - Acceptance: opt-in smoke test streams a response without exposing the key.

- [x] **MODEL-006: Implement model synchronization**
  - Depends on: MODEL-001, MODEL-004, PLAT-009
  - Milestone gate: M3-A04
  - Fetch provider metadata, upsert mappings, mark missing models unavailable, and
    never auto-enable.
  - Acceptance: repeated sync is idempotent and catalog changes are audited.

- [x] **MODEL-007: Implement public model catalog**
  - Depends on: MODEL-006, AUTH-005, GUEST-002
  - Milestone gate: M3-A01
  - Filter enabled/supported models by actor; guests get default only.
  - Acceptance: disabled and protocol-unsupported models cannot be selected.

- [x] **MODEL-008: Implement provider resilience**
  - Depends on: MODEL-004, PLAT-008
  - Milestone gate: M3-A05
  - Add circuit breaker, pre-stream retry, global concurrency, provider-neutral
    output-budget guard, and health. Exact monetary limits depend on the approved
    production provider and pricing configured in Phase 10.
  - Acceptance: failure injection proves no replay after partial output and correct
    recovery.

**Phase 4 exit:** Complete. M3-A01 through M3-A05 are accepted. The model catalog,
development adapter, synchronization, provider health, resilience, concurrency,
and global output-budget controls passed unit, race, SQL integration, migration,
and opt-in live-provider validation on 2026-07-24. M3 remains open until Phase 5
and its acceptance gates are complete.

## Phase 5: Conversations and realtime chat backend

**Milestone owner:** M3

**Milestone acceptance ledger:** `docs/m3-chat-backend.md`

**Objective:** Deliver the durable chat lifecycle and versioned WebSocket behavior.

- [x] **CHAT-001: Create conversation/chat schema**
  - Depends on: PLAT-002, AUTH-001, GUEST-001, MODEL-001
  - Milestone gate: M3-A10
  - Add conversations, messages, generations, summaries, usage ledger, outbox, and
    ownership/state constraints.
  - Acceptance: exactly-one-owner and one-active-generation constraints are tested.

- [x] **CONV-001: Implement conversation repository/service**
  - Depends on: CHAT-001
  - Milestone gate: M3-A06
  - Add create/get/list/search/rename/archive/delete/model-change ownership rules.
  - Acceptance: guest restriction, user IDOR, archived filters, cursor stability, and
    idle-only model change tests pass.

- [x] **CONV-002: Implement conversation HTTP API**
  - Depends on: CONV-001
  - Milestone gate: M3-A07
  - Expose contract endpoints and message pagination.
  - Acceptance: handler/contract tests cover auth, validation, errors, caching, and
    idempotency.

- [x] **WS-005: Implement one-time WebSocket tickets**
  - Depends on: AUTH-005, GUEST-002, PLAT-004
  - Milestone gate: M3-A08
  - Issue hashed 30-second single-use tickets for users/guests.
  - Acceptance: replay, expiry, actor mismatch, and Redis failure are denied.

- [x] **WS-006: Implement WebSocket connection lifecycle**
  - Depends on: ADR-001, WS-005, PLAT-007
  - Milestone gate: M3-A09
  - Validate origin/ticket, heartbeat, bounded queues, graceful close, resume buffer,
    and cross-instance pub/sub.
  - Acceptance: lifecycle fixtures and reconnect/load tests pass.

- [x] **CHAT-002: Implement generation state machine**
  - Depends on: CHAT-001, MODEL-003, QUOTA-001
  - Milestone gates: M3-A10, M3-A17
  - Reserve quota, persist messages/generation, stream/checkpoint, reconcile usage,
    and finalize terminal state.
  - Acceptance: every allowed/forbidden transition has deterministic tests.

- [x] **CHAT-003: Implement `chat.generate`**
  - Depends on: CHAT-002, WS-006, MODEL-004
  - Milestone gate: M3-A10
  - Validate content/safety, commit before acknowledgement, stream ordered deltas.
  - Acceptance: duplicate idempotency key produces one provider call and one message
    pair.

- [x] **CHAT-004: Implement cancellation**
  - Depends on: CHAT-003
  - Milestone gate: M3-A11
  - Propagate cancellation, persist partial state, release leases, reconcile quota.
  - Acceptance: cancellation works before first token, mid-stream, after completion,
    and across reconnect.

- [x] **CHAT-005: Implement retry of latest failed/cancelled generation**
  - Depends on: CHAT-004, CONV-002
  - Milestone gate: M3-A12
  - Enforce latest-only, no visible branch, new idempotent generation.
  - Acceptance: completed/non-latest retries are rejected and concurrency is safe.

- [x] **CHAT-006: Implement context builder**
  - Depends on: CHAT-002, MODEL-007
  - Milestone gate: M3-A13
  - Build system prompt + versioned summary + recent messages within model limits.
  - Acceptance: role ordering, excluded partial responses, injection boundaries, and
    token budgets are tested.

- [x] **CHAT-007: Implement conversation summarization**
  - Depends on: CHAT-006, PLAT-009
  - Milestone gate: M3-A14
  - Trigger at 70%, lock per conversation, use configurable cheap model, version
    contiguous coverage.
  - Acceptance: concurrent triggers create one valid summary and retain originals.

- [x] **CHAT-008: Implement title generation**
  - Depends on: CHAT-003
  - Milestone gate: M3-A15
  - Generate a safe short title asynchronously after initial exchange.
  - Acceptance: failures do not affect chat; user-renamed titles are never replaced.

- [x] **CHAT-009: Implement safety policy pipeline**
  - Depends on: CHAT-003, SEC-001
  - Milestone gate: M3-A16
  - Add size/format checks, configurable input/output categories, abuse signals, and
    report hooks independent of provider.
  - Acceptance: blocked content uses stable codes and content does not enter logs.

- [x] **USAGE-001: Implement usage API and aggregates**
  - Depends on: CHAT-003, QUOTA-001
  - Milestone gate: M3-A17
  - Expose current usage/reset and aggregate non-identifying metrics.
  - Acceptance: reserved/actual/refunded values reconcile under failure.

**Phase 5 exit:** Complete on 2026-07-24. M3 backend supports fake-provider and
development-provider chat, guest and registered rules, reconnect, cancellation,
retry, summaries, configurable safety, and reconciled usage.

## Phase 6: Administration and privacy backend

**Milestone owner:** M4

**Milestone acceptance ledger:** `docs/m4-web-application.md`

- [x] **ADMIN-001: Implement typed runtime settings**
  - Depends on: MODEL-001, AUTH-007
  - Add versioned reads/updates for models, quotas, prompts, safety, and maintenance.
  - Acceptance: optimistic conflict, validation, cache invalidation, and audit tests.

- [x] **ADMIN-002: Implement model administration**
  - Depends on: MODEL-006, ADMIN-001
  - Expose sync, enable/disable, defaults, ordering, and limits.
  - Acceptance: cannot enable unsupported/unavailable models or remove the only valid
    default without replacement.

- [x] **ADMIN-003: Implement user role administration**
  - Depends on: AUTH-007, ADMIN-001
  - Search users and change roles with recent auth and audit.
  - Acceptance: prevent removal of last admin and self-lockout without handoff.

- [x] **ADMIN-004: Implement usage/error/audit read APIs**
  - Depends on: USAGE-001, PLAT-008, ADMIN-001
  - Return aggregates and redacted audit events.
  - Acceptance: no prompt/response bodies or secrets are queryable.

- [x] **PRIV-002: Implement account deletion request**
  - Depends on: AUTH-005, CHAT-001, PLAT-009
  - Require recent auth, revoke sessions immediately, create durable deletion job.
  - Acceptance: repeated requests are idempotent and login remains blocked.

- [x] **PRIV-003: Implement account purge worker**
  - Depends on: PRIV-002
  - Remove personal/conversation data within 24 hours; retain anonymous aggregates
    and expiring security logs only.
  - Acceptance: integration test proves referential cleanup and non-identifiability.

- [x] **PRIV-004: Implement daily guest cleanup**
  - Depends on: GUEST-003, PLAT-009
  - Delete non-migrated guest conversation/session data daily.
  - Acceptance: migrated/active locked data is preserved; repeated run is safe.

**Phase 6 exit:** Complete on 2026-07-24. M4 now has versioned and cached runtime
settings, safe model-default replacement, concurrency-safe role administration,
aggregate usage/error/latency reads, redacted audit reads, and the previously
accepted deletion and guest-cleanup lifecycle. M4 remains open for Phases 7 and 8.

## Phase 7: Frontend foundation and design system

**Milestone owner:** M4

**Milestone acceptance ledger:** `docs/m4-web-application.md`

**Objective:** Build the complete product shell and accessible components before
wiring full journeys.

- [x] **WEB-001: Implement locale architecture**
  - Depends on: FOUND-002
  - Select i18n ADR/library, typed keys, Spanish/English dictionaries, browser/profile
    preference, English fallback.
  - Acceptance: missing/parity check fails CI; locale persists for users.

- [x] **WEB-002: Implement theme architecture**
  - Depends on: FOUND-004
  - Add light/dark/system behavior without flash and persist preference.
  - Acceptance: token parity and contrast automated checks pass.

- [x] **WEB-003: Build responsive application shell**
  - Depends on: FOUND-004, WEB-001, WEB-002
  - Add Glazz header, shadcn Sidebar desktop, Sheet mobile, main/transcript landmark,
    safe areas, and stable composer region.
  - Acceptance: 375/768/1024/1440 screenshots have no overlap/scrollbar errors.

- [x] **WEB-004: Build conversation navigation components**
  - Depends on: WEB-003
  - List, search, groups, rename, archive, delete, loading/empty/error states.
  - Acceptance: keyboard/touch/screen-reader component tests pass.

- [x] **WEB-005: Build transcript and message renderer**
  - Depends on: WEB-003
  - Add sanitized Markdown, tables, code highlighting/copy, long-content containment.
  - Acceptance: malicious HTML/XSS fixtures are inert; accessibility tests pass.

- [x] **WEB-006: Build streaming reducer and signal rail**
  - Depends on: WEB-005, WS-004
  - Handle start/delta/terminal events, duplicates, offsets, and reduced motion.
  - Acceptance: out-of-order/duplicate fixtures render one correct response.

- [x] **WEB-007: Build chat composer**
  - Depends on: WEB-003
  - Add multiline behavior, draft preservation, send/stop stable control, disabled
    reasons, mobile viewport handling.
  - Acceptance: keyboard, touch, IME composition, and mobile keyboard tests pass.

- [x] **WEB-008: Build model selector and usage indicator**
  - Depends on: WEB-003
  - Plain-language model options, guest restriction, quota/reset display.
  - Acceptance: unavailable model and approaching/exhausted quota states pass.

- [x] **WEB-009: Build connection and failure states**
  - Depends on: WEB-003, WEB-006
  - Add reconnect, resync, maintenance, inline failure, retry, and jump-to-latest.
  - Acceptance: screen-reader announcements are throttled and actionable.

- [x] **WEB-010: Create PWA shell**
  - Depends on: WEB-003
  - Add manifest, approved temporary icon process, static asset caching, offline/update
    states.
  - Acceptance: no API/transcript response is service-worker cached.

**Phase 7 exit:** Complete on 2026-07-24. The responsive bilingual shell, theme
bootstrap, accessible navigation and dialogs, transcript/composer primitives,
quota/model states, throttled stream announcements, and user-controlled PWA
update/offline behavior pass unit and browser acceptance. M4 remains open for
Phase 8.

## Phase 8: Frontend journeys and API integration

**Milestone owner:** M4

**Milestone acceptance ledger:** `docs/m4-web-application.md`

- [x] **INT-001: Integrate generated HTTP client**
  - Depends on: FOUND-006, WEB-003
  - Add server/client-safe transport, CSRF, request IDs, stable error mapping.
  - Acceptance: DTOs are generated; credentials never enter client logs.

- [x] **INT-002: Implement WebSocket client**
  - Depends on: WS-006, WEB-006
  - Obtain tickets, connect, heartbeat, reconnect/backoff, resume, resync, dispatch.
  - Acceptance: protocol fixture and browser integration tests pass.

- [x] **WEB-011: Implement guest journey**
  - Depends on: GUEST-002, CONV-002, CHAT-003, INT-001, INT-002, WEB-007
  - Immediate chat, allowance, limit gate, preserved transcript.
  - Acceptance: four-message/2,000-token edge cases and daily expiry behavior pass E2E.

- [x] **WEB-012: Implement Google login and guest migration UX**
  - Depends on: AUTH-004, GUEST-003, WEB-011
  - Add Google entry, callback/error UI, migration continuity, legal consent.
  - Acceptance: successful login retains conversation exactly once; denial is
    recoverable.

- [x] **WEB-013: Implement registered conversation journey**
  - Depends on: CONV-002, WEB-004, INT-001
  - Create/list/search/rename/archive/delete/model change and pagination.
  - Acceptance: deep links, reload, empty/error/loading, and ownership failures work.

- [x] **WEB-014: Integrate generation/cancel/retry**
  - Depends on: CHAT-005, INT-002, WEB-006, WEB-007, WEB-009
  - Wire acknowledgement, streaming, stop, retry, usage, and draft behavior.
  - Acceptance: reconnect and duplicate command E2E preserve one transcript.

- [x] **WEB-015: Implement settings**
  - Depends on: AUTH-005, WEB-001, WEB-002
  - Profile read, locale/theme, device sessions, revocation, legal links.
  - Acceptance: current-session revocation logs out; other revocation updates list.

- [x] **WEB-016: Implement account deletion**
  - Depends on: PRIV-003, WEB-015
  - Recent auth, specific confirmation, job state, immediate logout.
  - Acceptance: destructive flow is keyboard accessible and passes E2E.

- [x] **WEB-017: Implement admin model/settings UI**
  - Depends on: ADMIN-002, WEB-003
  - Models, defaults, quotas, guest limits, system prompt, safety, maintenance.
  - Acceptance: validation/conflicts/audits are visible; no nested cards.

- [x] **WEB-018: Implement admin user/usage/audit UI**
  - Depends on: ADMIN-003, ADMIN-004, WEB-003
  - User roles, aggregate usage/errors, audit pagination.
  - Acceptance: non-admin routes/API are denied and conversation content absent.

**Phase 8 exit:** Complete on 2026-07-24. All M4 MVP screens and states are
integrated in Spanish/English, including exact guest token exhaustion and expired
session renewal. GitHub Actions run `30118902670` passed and M4 was published as
`v0.4.0`.

## Phase 9: Verification, hardening, and performance

**Milestone owner:** M5

**Milestone acceptance ledger:** `docs/m5-release-candidate.md`

- [x] **QA-001: Complete backend unit/integration suite**
  - Depends on: all backend MVP tasks
  - Acceptance: domain, repositories, handlers, jobs, adapters, and concurrency
    critical paths pass with real PostgreSQL/Redis.

- [x] **QA-002: Complete Go race and leak testing**
  - Depends on: QA-001
  - Exercise WebSockets, refresh rotation, migration, quota, cancellation, worker.
  - Acceptance: `go test -race ./...` passes; no goroutine/stream leaks in scenarios.

- [x] **QA-003: Complete frontend component/accessibility suite**
  - Depends on: all web MVP tasks
  - Acceptance: axe, keyboard, focus, localization, streaming reducer, malicious
    Markdown, 200% zoom behaviors pass.

- [x] **QA-004: Complete Playwright E2E suite**
  - Depends on: WEB-018
  - Cover guest, limit, OAuth stub/migration, registered chat, cancel/retry, archive,
    sessions, deletion, admin, themes/locales, mobile.
  - Acceptance: deterministic fake provider suite passes in CI.

- [ ] **QA-005: Run visual regression matrix**
  - Depends on: QA-003, QA-004
  - Capture 375/768/1024/1440, light/dark, Spanish/English, empty/stream/error/quota,
    admin.
  - Acceptance: no overlap, clipping, unstable dimensions, or unapproved diffs.

- [ ] **SEC-002: Execute application security review**
  - Depends on: QA-001, QA-003, SEC-001
  - Test OAuth/CSRF/JWT/refresh/WS/IDOR/admin/guest bypass/XSS/CSP/secrets/dependencies.
  - Acceptance: no open critical/high findings; medium findings owned and scheduled.

- [ ] **QA-006: Load and soak test realtime API**
  - Depends on: CHAT-009, PLAT-008
  - Model expected connections, slow clients, reconnect storms, cancellation, Redis
    pub/sub, provider latency.
  - Acceptance: bounded memory/queues; p95 targets and graceful degradation documented.

- [ ] **QA-007: Test dependency failure matrix**
  - Depends on: QA-006
  - Simulate PostgreSQL, Redis, provider, telemetry, worker, and network failures.
  - Acceptance: no acknowledged data loss; correct readiness/circuit/recovery.

- [ ] **QA-008: Validate privacy and deletion**
  - Depends on: PRIV-003, PRIV-004
  - Inspect logs/traces/errors/analytics, guest purge, account purge, retained metrics.
  - Acceptance: no content leaks; deletion SLA and retention rules pass.

- [ ] **QA-009: Bundle and web performance audit**
  - Depends on: QA-005
  - Analyze client bundle, hydration, fonts, CLS, long transcripts, code highlighting.
  - Acceptance: no avoidable client-heavy dependency; CLS below 0.1; budgets recorded.

- [ ] **QA-010: Final contract compatibility review**
  - Depends on: QA-004
  - Compare implementation, generated clients, examples, and future mobile needs.
  - Acceptance: zero undocumented endpoints/events or incompatible drift.

**Phase 9 exit:** M5 release candidate satisfies functional, accessibility, security,
privacy, resilience, and performance criteria.

## Phase 10: Production infrastructure and release

**Milestone owner:** M6

**Milestone acceptance ledger:** `docs/m6-production-launch.md`

- [ ] **PROD-001: Select production API/PostgreSQL/Redis hosting**
  - Depends on: M5
  - Evaluate persistent WebSockets, regions, private network, managed backups, secrets,
    scaling, cost, and observability.
  - Acceptance: ADR and cost envelope approved.

- [ ] **PROD-002: Select and approve production LLM provider**
  - Depends on: MODEL-004, QA-006
  - Validate commercial multi-user use, privacy/retention, capacity, model quality,
    regions, cost, streaming, and support.
  - Acceptance: agreement/terms approved; adapter contract and load tests pass.

- [ ] **PROD-003: Provision production environments**
  - Depends on: PROD-001, PROD-002
  - Create staging/production API, worker, DB, Redis, secret manager, networking.
  - Acceptance: infrastructure is reproducible and access is least-privilege.

- [ ] **PROD-004: Configure HTTPS, domains, and Google OAuth**
  - Depends on: PROD-003
  - Configure `glazz.hlab.sh`, API host, certificates, callbacks, cookie/CORS/origin
    allowlists.
  - Acceptance: production login/WSS work with no insecure fallback.

- [ ] **PROD-005: Build deployment pipelines**
  - Depends on: PROD-003, FOUND-009
  - Immutable image, artifact provenance/SBOM, staging promotion, migration job,
    rolling deploy, rollback.
  - Acceptance: deploy and rollback drills pass without contract/schema break.

- [ ] **PROD-006: Enable backups and complete restore drill**
  - Depends on: PROD-003
  - Daily backup, seven-day retention, PITR if supported, encrypted storage.
  - Acceptance: measured RPO <=24h and RTO <=4h; runbook verified.

- [ ] **PROD-007: Configure dashboards, alerts, and runbooks**
  - Depends on: PROD-003, PLAT-008
  - Cover availability, latency, auth abuse, WS, provider, quota/spend, DB/Redis,
    jobs/deletion, backup.
  - Acceptance: every critical alert has owner, severity, and tested runbook.

- [ ] **PROD-008: Configure production model catalog and limits**
  - Depends on: PROD-002, ADMIN-002
  - Sync approved provider, enable reviewed models, choose defaults, global budgets,
    and maintenance fallback.
  - Acceptance: unsupported models remain hidden; cost circuit breaker test passes.

- [ ] **PROD-009: Complete legal and content review**
  - Depends on: PRIV-001, QA-008
  - Review Terms, Privacy Policy, age gate, model/provider disclosure, report flow.
  - Acceptance: approved versions deployed and acceptance versioning verified.

- [ ] **PROD-010: Run staging release rehearsal**
  - Depends on: PROD-004 through PROD-009
  - Execute migrations, smoke/E2E/load/security, provider failover/circuit, deletion,
    backup restore, rollback.
  - Acceptance: release checklist signed with no critical blockers.

- [ ] **PROD-011: Production launch**
  - Depends on: PROD-010
  - Controlled rollout with budget/concurrency limits and active monitoring.
  - Acceptance: M6 health, auth, guest, chat, admin, deletion, telemetry, and alert
    smoke checks pass.

- [ ] **PROD-012: Post-launch review**
  - Depends on: PROD-011
  - Review first 24h/7d metrics, incidents, cost, conversion baseline, provider quality.
  - Acceptance: actions are prioritized; numeric product targets are proposed.

## Phase 11: Post-MVP backlog

**Milestone owner:** None. This phase is outside the M0-M6 release plan.

These items are intentionally outside the release-critical path:

- [ ] Conversation export
- [ ] Public sharing with explicit privacy controls
- [ ] Attachments and file lifecycle
- [ ] Image input/generation
- [ ] Web search with source citations
- [ ] Message editing, regeneration, and branches
- [ ] User-defined instructions/system prompts
- [ ] Paid Glazz plans and billing
- [ ] Native mobile clients
- [ ] OpenAI Responses adapter
- [ ] Anthropic adapter
- [ ] Google adapter
- [ ] Multi-provider routing and automated failover
- [ ] Advanced moderation vendor integration
- [ ] Organization/team workspaces

Each post-MVP item requires product scope, threat-model updates, contract changes,
data-retention analysis, design states, and a new delivery breakdown before coding.
