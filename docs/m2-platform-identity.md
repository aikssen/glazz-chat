# M2 Platform and Identity Record

Date: 2026-07-23

## Outcome

M2 establishes the secure actor boundary required before chat implementation:
PostgreSQL migrations and typed queries, Redis primitives, structured runtime
configuration, HTTP/telemetry foundations, an outbox worker, Google OAuth with
state/PKCE/nonce, Ed25519 JWTs, rotating refresh sessions, guest identity,
transactional guest migration, consent records, administrator bootstrap, and
quota reservation/refund behavior.

The milestone is versioned as `v0.2.0`. OpenCode Go remains a development-only
provider setting and is not involved in M2 identity or platform behavior.

## Security Properties

- OAuth state is server-held, single-use, expiring, and binds PKCE, nonce,
  allowlisted return path, locale, consent, and optional guest ownership.
- Browser access and refresh credentials are `HttpOnly`; mutations use a signed
  double-submit CSRF token and origin allowlisting.
- Refresh tokens are stored only as SHA-256 hashes, rotate atomically, and revoke
  their family when replay is detected.
- Guest cookies are signed and expose no database identifier. IP controls use a
  keyed HMAC rather than storing raw addresses.
- Quota reservations charge the upper output bound first, refund unused tokens,
  enforce one generation lease, and fail closed when Redis is unavailable.
- Logs and metrics use bounded route/error classes and never include credentials,
  prompt bodies, or model responses.

## Verification

- Docker Compose builds and starts exact PostgreSQL, Redis, API, worker, and web
  images with dependency health checks and local persistence policy.
- Migration integration rolls the schema down and up from an empty M2 database,
  validates checksums, and exercises generated `sqlc` queries.
- Integration tests cover Redis expiry/rate/lease contention, identity replay,
  concurrent refresh reuse, guest expiry and migration, quota settlement, and
  concurrent outbox claims with idempotent completion.
- Contract generation, Go unit/race tests, vet/build, frontend checks, dependency
  audits, and secret scanning remain required in CI.
