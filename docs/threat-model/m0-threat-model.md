# M0 Threat Model

- Status: Approved baseline
- Date: 2026-07-23
- Scope: HTTP API, WebSocket protocol, OAuth, guests, chat, admin, provider gateway,
  and deletion lifecycle

## Assets

- User identity and sessions
- Guest and registered conversation content
- Provider/OAuth/JWT secrets
- Quota and cost budgets
- Administrative configuration and audit trail
- Availability of chat and account deletion

## Trust boundaries

1. Browser/PWA to Go API over HTTPS/WSS
2. Go API to Google OAuth
3. Go API/worker to PostgreSQL and Redis
4. Go API to development/production LLM provider
5. Administrator browser to privileged API
6. CI/deployment system to secret manager and production

## Threat register

| ID | Threat | Impact | Primary mitigation | Verification owner/task |
| --- | --- | --- | --- | --- |
| T01 | OAuth state/code interception or login CSRF | Account takeover | PKCE, nonce, server-held single-use state, fixed callbacks | Backend / AUTH-004 |
| T02 | Unsafe OAuth return URL | Phishing/token leakage | Exact allowlist; server-selected defaults | Security / AUTH-004 |
| T03 | CSRF on cookie-authenticated mutation | Unauthorized change | SameSite cookies, CSRF token, Origin checks | Backend / AUTH-006 |
| T04 | Access JWT theft/algorithm confusion | Session compromise | HttpOnly, short TTL, EdDSA-only parser, required claims | Security / AUTH-002 |
| T05 | Refresh-token theft/reuse race | Long-lived takeover | Opaque hash, atomic rotation, family revoke, reuse telemetry | Backend / AUTH-003 |
| T06 | WebSocket hijacking or ticket replay | Data exposure/actions | Single-use 30s ticket, strict Origin, actor binding | Realtime / WS-005 |
| T07 | IDOR across conversations/sessions | Privacy breach | Repository ownership predicates and authz tests | Backend / CONV-001 |
| T08 | Guest quota bypass | Cost/abuse | Signed cookie, IP hash, Redis limits, durable counters, global breaker | Platform / QUOTA-001 |
| T09 | Duplicate generation on reconnect | Cost/inconsistent transcript | Idempotency key, durable ack, one active generation | Chat / CHAT-003 |
| T10 | Slow WebSocket client exhausts memory | Availability | Bounded queues, write timeout, close slow clients | Realtime / WS-006 |
| T11 | Oversized/malformed frames | Availability/parser abuse | Read/body limits, schema validation, early reject | Realtime / WS-006 |
| T12 | Prompt injection overrides authorization/safety | Unsafe output/data action | No tools in MVP, fixed domain authorization, independent policies | Safety / CHAT-009 |
| T13 | Markdown/XSS from model output | Browser compromise | No raw HTML, sanitizer, CSP, safe links | Frontend / WEB-005 |
| T14 | Provider key or payload leaks in logs | Secret/privacy breach | Adapter redaction, allowlisted fields, log tests | Platform / MODEL-004 |
| T15 | Provider sends malformed/hostile stream | Crash/corruption | Size limits, normalized parser, terminal failure state | Provider / MODEL-004 |
| T16 | Admin privilege escalation or abuse | Service-wide compromise | Server role checks, recent auth, last-admin guard, audit | Admin / ADMIN-003 |
| T17 | Runtime setting lost update | Unsafe policy | Versioned optimistic update and audit before/after | Admin / ADMIN-001 |
| T18 | Account deletion race with active generation | Retained/new data | Revoke first, block writes, durable purge job, idempotency | Privacy / PRIV-002 |
| T19 | Guest cleanup deletes migrated data | Data loss | Transactional ownership, locks, migration marker | Privacy / PRIV-004 |
| T20 | Redis failure disables protection | Cost/abuse | Fail closed for generation/tickets, degrade reads only | Platform / PLAT-004 |
| T21 | Database/outbox inconsistency | Lost state/job | Same transaction, idempotent consumers, retries | Platform / PLAT-009 |
| T22 | Dependency/supply-chain compromise | Code execution | Context7+official verification, pinning, lockfiles, SBOM, scans | CI / FOUND-009 |
| T23 | Secret committed or exposed in bundle | Credential compromise | gitignore, secret scan, server-only config, bundle inspection | CI / FOUND-008 |
| T24 | Sensitive telemetry cardinality/content | Privacy/cost | Bounded labels, hashed IDs, no message content | Platform / PLAT-008 |

## Privacy decisions

- Operational telemetry never contains prompt/response bodies.
- Guest content is deleted by the daily cleanup if not migrated.
- Account sessions are revoked immediately; purge completes within 24 hours.
- Security logs expire after 30 days.
- Provider approval includes commercial use, retention, training, data region, and
  subprocessors. OpenCode Go is development-only.

## Residual risks

- Safety classifiers and system prompts cannot guarantee harmless output.
- IP/device controls cannot completely prevent distributed guest abuse.
- A provider sees message content required for inference.
- Legal/privacy claims require counsel and final production-provider review.

These risks are accepted for M0 only with global budgets, clear disclosures,
reporting, provider review, and production release gates.

