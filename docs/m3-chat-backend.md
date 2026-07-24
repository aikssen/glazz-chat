# M3 Streamed Chat Backend Progress

## Status

M3 is in progress and is not tagged. The release tag remains reserved as
`v0.3.0` until Phase 4 and Phase 5 acceptance criteria and CI are green.

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

## Remaining before `v0.3.0`

- Expand ownership, cursor, state-transition, cancellation timing/reconnect,
  retry-concurrency, context ordering, summary concurrency, title, and usage
  reconciliation tests.
- Complete configurable safety categories/report hooks and non-identifying usage
  aggregates.
- Run the full repository presubmit and CI, then commit, push, and create the
  annotated `v0.3.0` tag only after CI is green.
