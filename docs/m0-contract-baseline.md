# M0 Contract Baseline Review

## Ownership

- **Milestone:** M0
- **Owned phase:** Phase 0
- **Release:** `v0.1.0`

- Baseline: `v0.1.0`
- Date: 2026-07-23
- Status: Accepted for implementation

## Scope reviewed

- Product and glossary consistency
- Public HTTP resources and errors
- Guest and registered-user rules
- Google OAuth, JWT cookies, refresh rotation, sessions, and recent auth
- Conversation ownership and generation retry
- Account deletion
- Administrator model/settings/user/usage/audit operations
- WebSocket ticket, lifecycle, commands, events, resume, ordering, cancellation,
  quota, and normalized provider failures
- Provider neutrality and future mobile compatibility
- Initial threat model

## Review decisions

### Product

- Chat generation stays WebSocket-only in the MVP.
- REST remains authoritative for resources and reconnect resynchronization.
- Guest migration is described as an OAuth callback side effect and must be
  transactional/idempotent.
- Provider identities are absent from public schemas.

### Backend

- All acknowledged commands follow durable commit.
- HTTP mutations with business impact require idempotency keys.
- Administrative writes require optimistic versioning and audit.
- Resource-not-owned is returned as `404` to reduce enumeration.

### Frontend

- Stable error/event codes drive localization.
- Deltas include offsets; duplicates are ignored.
- Missed replay results in explicit REST resource refetch.
- Authentication remains cookie-based; WebSocket uses a one-time ticket.

### Security

- EdDSA access JWT and opaque rotating refresh token are separate mechanisms.
- CSRF applies to unsafe cookie-authenticated HTTP requests.
- WebSocket tickets expire in at most 30 seconds and are consumed once.
- Raw model HTML is outside the API contract and must be sanitized in the web app.
- Prompt/response content is excluded from operational telemetry.

### Future mobile client

- REST/WS contracts do not depend on Next.js.
- Cookie auth is the browser profile. A future mobile authentication profile may
  add an Authorization header without changing resources.
- IDs, cursors, errors, idempotency, and events are transport-neutral.

## Validation record

- `packages/contracts/openapi.yaml`: Redocly valid.
- `packages/contracts/websocket.asyncapi.yaml`: Redocly valid.
- Six canonical fixture files: JSON syntax valid and manually mapped to event
  component schemas.
- Duplicate OpenAPI `operationId`: none.
- M0 threat register: 24 threats, each mapped to a verification task.

Generated runtime validation and CI wiring are intentionally Phase 1 (`FOUND-006`);
they do not change the accepted contract.

## Deferred nonblocking decisions

- Exact patch versions are pinned immediately before installation in Phase 1 after
  Context7 and upstream re-verification.
- Production API/Redis/PostgreSQL hosting is a production milestone decision.
- Production LLM provider is a release gate; OpenCode Go remains development-only.
- The repository starts at semantic version `v0.1.0`. This tag identifies the M0
  contract baseline and remains the starting point for M1 implementation.

## Approval

The baseline is internally coherent and approved to begin M1 foundations. Any
breaking change requires a contract review and version decision.
