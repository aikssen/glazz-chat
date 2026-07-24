# M3 Streamed Chat Backend Progress

## Ownership

- **Milestone:** M3
- **Owned phases:** Phase 4 and Phase 5
- **Reserved release:** `v0.3.0`
- Completing Phase 4 alone does not complete M3. Both phase exits and every blocking
  acceptance item below must be complete.

## Status

M3 is in progress and is not tagged. The release tag remains reserved as
`v0.3.0` until Phase 5 acceptance criteria and CI are green. Phase 4 completed on
2026-07-24 with M3-A01 through M3-A05 accepted.

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
- Context budgeting, versioned summary persistence under an advisory lock, initial
  title generation, input format/size policy, guest CSRF, and usage reporting.

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
- [ ] **M3-A06: Conversation ownership and pagination**
  - Add guest restrictions, cross-user IDOR denial, archived filters, stable cursors,
    message pagination, and idle-only model-change tests.
- [ ] **M3-A07: Conversation HTTP contract behavior**
  - Add handler coverage for authentication, CSRF, validation, cache behavior,
    stable errors, and idempotency.
- [-] **M3-A08: WebSocket ticket denial matrix**
  - Evidence complete: actor binding, expiry, single use, and replay denial.
  - Pending: Redis failure behavior at issuance and consumption.
- [ ] **M3-A09: WebSocket connection lifecycle**
  - Add origin denial, heartbeat timeout, bounded slow-client queues, resume,
    resync fallback, cross-instance pub/sub, reconnect storm, and load coverage.
- [-] **M3-A10: Generation state machine and idempotency**
  - Evidence complete: durable acceptance, one provider call, one message pair, and
    completed usage in `internal/integration/m3_chat_integration_test.go`.
  - Pending: exhaustive allowed/forbidden transitions and failure reconciliation.
- [ ] **M3-A11: Cancellation lifecycle**
  - Cover cancellation before the first token, mid-stream, after completion, and
    after reconnect; verify partial persistence, lease release, and quota settlement.
- [ ] **M3-A12: Retry lifecycle**
  - Cover latest-only enforcement, completed/non-latest rejection, duplicate
    idempotency, and concurrent retry safety.
- [ ] **M3-A13: Context builder**
  - Verify role ordering, partial-response exclusion, injection boundaries, summary
    placement, and input/output token budgets.
- [ ] **M3-A14: Summarization**
  - Verify the 70% trigger, advisory-lock concurrency, contiguous version coverage,
    failure isolation, and retention of original messages.
- [ ] **M3-A15: Title generation**
  - Verify safe truncation, asynchronous failure isolation, initial-exchange-only
    behavior, and preservation of user-renamed titles.
- [ ] **M3-A16: Safety pipeline**
  - Connect configurable input/output categories and provider-independent report
    hooks; prove stable blocked-content codes and content-free logs.
- [ ] **M3-A17: Usage reconciliation and aggregates**
  - Reconcile reserved, actual, and refunded usage under success, provider failure,
    cancellation, and retry; prove aggregates are non-identifying.

## Remaining before `v0.3.0`

- Close every unchecked or partial Phase 5 item in `M3-A06` through `M3-A17`.
- Reconcile the corresponding Phase 5 task statuses in `TASKS.md` from acceptance
  evidence rather than implementation availability.
- Run the full repository presubmit and CI, then commit, push, and create the
  annotated `v0.3.0` tag only after CI is green.
