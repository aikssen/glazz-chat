# M3 Streamed Chat Backend Progress

## Ownership

- **Milestone:** M3
- **Owned phases:** Phase 4 and Phase 5
- **Reserved release:** `v0.3.0`
- Completing Phase 4 alone does not complete M3. Both phase exits and every blocking
  acceptance item below must be complete.

## Status

M3 implementation and local acceptance are complete. Phase 4 and Phase 5 completed
on 2026-07-24 with M3-A01 through M3-A17 accepted. The accepted commit becomes the
`v0.3.0` release only after it is pushed and GitHub CI is green.

## Implemented

- Provider/model catalog schema with a DeepSeek V4 Flash public model and
  provider-neutral mappings.
- Provider-neutral request, message, chunk, usage, finish, and normalized error
  types.
- Deterministic fake provider plus an OpenAI-compatible streamed Chat Completions
  adapter and resilience wrapper.
- Actor-owned conversation CRUD, search, archive, idle-only model changes, and
  durable message pagination.
- Hashed, actor-bound, single-use 30-second WebSocket tickets.
- Strict-origin WebSocket connections with bounded queues, heartbeat, Redis
  pub/sub, ordered sequence numbers, and bounded replay.
- Durable generation acceptance before command acknowledgement, idempotent
  message-pair creation, streaming checkpoints, terminal state, quota settlement,
  usage ledger, cancellation, and latest-only retry.
- Context budgeting, configurable summary model, versioned summary persistence
  under an advisory lock, Unicode-safe initial title generation, configurable
  input/output safety policy, guest CSRF, and usage reporting.

OpenCode Go remains a development-only OpenAI-compatible endpoint. When provider
configuration is absent, the backend uses the deterministic fake and requires no
external key.

## Verification completed

- `go test -race ./...`
- Integration tests against PostgreSQL and Redis for M2 plus durable/idempotent M3
  chat.
- Real WebSocket handshake, ready event, one-time ticket replay denial, and expiry.
- OpenAI-compatible timeout, malformed stream, rate-limit, partial disconnect,
  pre-stream retry, and circuit recovery tests.
- Idempotent provider-model synchronization that never auto-enables new models.
- Opt-in smoke test against the configured development provider.
- Remaining-output quota regression coverage for sequential guest prompts.
- Packaged API/worker startup on the remote Compose stack.
- Guest smoke flow: readiness, session plus CSRF cookie, model catalog,
  conversation creation, WebSocket ticket, and usage response.
- Phase 5 integration coverage against isolated PostgreSQL and Redis for
  conversations, WebSocket lifecycle, generation transitions, cancellation,
  retry, context, summaries, titles, safety, quota settlement, and usage.
- Full remote `pnpm check`, migration reset/validate, and
  `go test -tags=integration ./...` on 2026-07-24.
- Active preview rebuild with migrations `00007`-`00008`, healthy API/web/worker,
  and a passing opt-in Playwright stream against the configured development
  provider.
- LAN preview regression test covering effective CORS, deterministic development
  OAuth, registered-session recovery, WebSocket connection, enabled composer, and
  a live model response after removing Compose overrides of `env_file`.

## Acceptance ledger

Status uses the same notation as `TASKS.md`: `[x]` complete, `[-]` partially
verified, and `[ ]` not yet accepted. This ledger is the milestone release gate;
the task entries in `TASKS.md` remain the canonical scope index.

- [x] **M3-A01: Model schema and public catalog constraints**
  - Evidence: `internal/models/service_integration_test.go`,
    `queries/models.sql`, and migration `00006_model_availability.sql` prove that
    disabled, unsupported, unavailable, orphaned, provider-disabled, and
    guest-ineligible models cannot be exposed or selected.
- [x] **M3-A02: OpenAI-compatible adapter contract**
  - Evidence: `internal/provider/openai_test.go` covers normal streaming, usage,
    malformed events, timeout, rate limiting, and partial disconnect.
- [x] **M3-A03: Development-provider smoke**
  - Evidence: opt-in `internal/provider/configured_smoke_test.go` streamed a bounded
    response from the configured OpenCode Go endpoint on 2026-07-24 without
    committing or printing credentials.
- [x] **M3-A04: Provider model synchronization**
  - Evidence: `internal/models/synchronizer_integration_test.go` proves idempotency,
    unavailable transitions, ignored unknown models, and no automatic exposure.
- [x] **M3-A05: Provider resilience**
  - Evidence: `internal/provider/resilient_test.go` proves pre-stream-only retry,
    bounded concurrency, no replay after partial output, circuit opening, health,
    and recovery. `internal/provider/openai_test.go` proves authenticated health and
    first-chunk/idle-stream timeouts. The quota service enforces a transactional,
    runtime-configurable global output-token budget before provider admission.
  - Exact currency spend enforcement is a Phase 10 concern because it requires the
    approved production provider's pricing and billable-usage semantics.
- [x] **M3-A06: Conversation ownership and pagination**
  - Evidence: `internal/conversations/service_integration_test.go` covers guest
    restrictions, cross-user IDOR denial, archived/search filters, stable keyset
    cursors, message pagination, idle-only model changes, and durable create/delete
    idempotency.
- [x] **M3-A07: Conversation HTTP contract behavior**
  - Evidence: `internal/platform/server/conversations_integration_test.go` covers
    authentication, guest CSRF, strict validation, stable errors, ETag/304 behavior,
    cache headers, and idempotent create/delete replay.
- [x] **M3-A08: WebSocket ticket denial matrix**
  - Evidence: `internal/realtime/tickets_test.go` and
    `handler_integration_test.go` cover actor binding, expiry, single use, replay,
    and Redis failures during issuance and consumption.
- [x] **M3-A09: WebSocket connection lifecycle**
  - Evidence: `internal/realtime/handler_integration_test.go` and `broker_test.go`
    cover origin denial, heartbeat timeout, bounded queues, replay/resync,
    cross-broker pub/sub, and 24 concurrent reconnects.
- [x] **M3-A10: Generation state machine and idempotency**
  - Evidence: `internal/chat/service_integration_test.go` covers durable acceptance,
    one active generation, duplicate commands, one provider call, terminal
    transition denial, checkpoints, failure finalization, and one ledger record.
- [x] **M3-A11: Cancellation lifecycle**
  - Evidence: the chat integration suite cancels before the first token, mid-stream,
    after reconnect, and rejects cancellation after terminal state while proving
    partial persistence, lease release, and quota settlement.
- [x] **M3-A12: Retry lifecycle**
  - Evidence: the chat integration suite proves latest-terminal eligibility,
    completed rejection, parent linkage, duplicate idempotency, one retry provider
    call, and deterministic concurrent terminal rejection.
- [x] **M3-A13: Context builder**
  - Evidence: the chat integration suite proves system/user role boundaries,
    partial-assistant exclusion, summary placement, recent-message ordering, and the
    70% context budget.
- [x] **M3-A14: Summarization**
  - Evidence: migration `00008_summary_model.sql` and the chat integration suite
    prove runtime model selection, advisory-lock concurrency, contiguous versions,
    failure isolation, and retention of original messages.
- [x] **M3-A15: Title generation**
  - Evidence: the chat integration suite proves Unicode-safe 60-rune truncation,
    post-terminal failure isolation, initial-title-only behavior, and preservation
    of user-renamed titles.
- [x] **M3-A16: Safety pipeline**
  - Evidence: `internal/chat/safety.go`, unit tests, and the chat integration suite
    prove runtime input/output categories, provider-independent reports without
    content, no persistence for blocked text, and stable `safety_blocked` errors.
- [x] **M3-A17: Usage reconciliation and aggregates**
  - Evidence: the chat integration suite proves reserved/actual/refunded
    reconciliation and exactly one ledger row under success, provider failure,
    cancellation, and retry; admin aggregates expose no actor fields.

## `v0.3.0` release gate

- Commit and push the accepted M3 tree.
- Require a green GitHub CI run for that exact commit.
- Create and push the annotated `v0.3.0` tag from that commit only after CI passes.
