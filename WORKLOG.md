# Glazz Work Log

## Purpose

This file records dated engineering activity, verification evidence, decisions, and
the next working focus. It is not a task tracker and must not duplicate or override
status from `TASKS.md` or a milestone acceptance ledger.

Rules:

1. Append new dated entries at the top of the log.
2. Record completed actions and observed results, not speculative completion.
3. Link task IDs, acceptance IDs, commits, or test commands when available.
4. Update canonical status in `TASKS.md` and the relevant milestone document.
5. Never use unchecked boxes here. Pending work belongs in the canonical tracker.

## 2026-07-24

### Phase 7 frontend foundation completed

- Closed WEB-002 through WEB-010 and accepted M4-A06.
- Added pre-hydration light/dark/system theme selection, responsive application
  height handling, stable 40/44-pixel controls, skip navigation, inert mobile
  sheets, focus-contained dialogs, and explicit focus restoration.
- Added empty navigation states, unavailable-model and quota messaging, lifecycle-
  only stream announcements, and bilingual labels across chat, settings, and admin.
- Changed the PWA to cache only same-origin navigation, exclude API/transcript
  responses, wait for explicit update acceptance, and expose an offline status.
- Corrected the standalone web image to include public assets and restored the
  documented direct-LAN environment origins.
- Verified lint, typecheck, 26 frontend unit tests, production build, four responsive
  screenshots, WCAG/keyboard/reflow checks, service-worker cache policy and offline
  behavior, Compose health, and a completed response from the live provider.

### Configured provider catalog corrected and cleaned

- Compared the configured provider `/models` response with PostgreSQL and found 23
  current OpenCode Go models, while the database contained 33 leaked test models
  and 13 leaked test providers.
- Corrected runtime provider registration so a configured OpenAI-compatible
  provider no longer reuses the deterministic `fake` provider identity.
- Extended synchronization to import newly discovered chat models with stable
  slugs, readable names, conservative limits, user audience, and disabled exposure
  until an administrator explicitly enables them. The worker now refreshes the
  configured catalog on startup so a fresh environment maps its default model
  without a manual administrative action.
- Corrected integration cleanup ordering and summary-model cleanup so repeated test
  runs no longer add model or provider records to the shared development database.
- Removed the 33 test models, 13 test providers, and their test audit records in one
  transaction after proving they had no conversation or generation references.
- Synchronized and explicitly enabled all 23 models currently returned by the
  configured subscription. DeepSeek V4 Flash remains the guest/user default and
  the only guest-visible model.
- Verified full presubmit, all PostgreSQL/Redis integration tests, 23 public
  registered-user models, and a completed LAN browser generation routed through
  the `configured` provider rather than `fake`.

### Phase 6 administration and privacy backend completed

- Closed ADMIN-001 through ADMIN-004 and accepted M4-A01 through M4-A05.
- Added Redis-cached typed runtime settings with optimistic versioning, validation,
  explicit invalidation, and redacted immutable audit records.
- Made default-model replacement atomic, rejected unsupported or unavailable model
  exposure, and preserved a valid guest/user default during administrative changes.
- Serialized administrator role changes, blocked self-demotion and inactive-user
  promotion, and used the injected clock for recent-auth validation.
- Expanded administration usage reads with failed-generation counts, average and
  p95 latency, and grouped error codes without exposing prompts or responses.
- Added service and HTTP integration tests using isolated PostgreSQL databases and
  Redis namespaces, including concurrent cross-demotion and real cookie/JWT/CSRF
  enforcement.
- Validated OpenAPI and AsyncAPI, frontend lint/typecheck/tests/build, `go vet`,
  `go test -race ./...`, Go builds, and three full PostgreSQL/Redis integration
  passes on the remote development server.

### Phase 5 preview configuration regression corrected

- Reopened M3 after the LAN preview showed `Load failed`, perpetual reconnect, a
  disabled composer, and `Google login is not configured`.
- Traced the regression to Compose `environment` defaults overriding the root
  `env_file`: the API received localhost-only CORS/redirect URLs and disabled
  development OAuth even though the remote environment file was updated.
- Removed duplicated externally managed settings from the Compose environment,
  configured both LAN and localhost origins, restored deterministic development
  OAuth for `manual-20260724@glazz.test`, and preserved the existing JWT issuer so
  prior sessions remain valid.
- Verified LAN CORS headers, OAuth start/approval/callback, `/me`, registered usage,
  WebSocket connection, restored conversations, enabled composer, and a real model
  response with a non-destructive Playwright recovery smoke.

### Phase 5 completed locally

- Accepted M3-A09 through M3-A17 and marked CHAT-001 through CHAT-009, WS-006,
  and USAGE-001 complete from automated evidence.
- Added deterministic heartbeat timeout, replay-gap resync, bounded-queue,
  cross-instance pub/sub, and concurrent reconnect coverage.
- Proved generation idempotency and state constraints, cancellation before and
  during streaming, reconnect cancellation, latest-only retry, partial usage
  reconciliation, and one ledger entry per terminal generation.
- Added a 70% context builder, configurable summary model, advisory-lock summary
  versions, Unicode-safe initial titles, and preservation of user-renamed titles.
- Added configurable provider-independent input/output safety rules and
  content-free reporting; blocked input is not persisted and blocked output uses
  the stable `safety_blocked` code.
- Rebuilt an isolated PostgreSQL schema through migrations `00001`-`00008`, ran
  migration validation, full `pnpm check`, and all Go integration tests against
  PostgreSQL and Redis.
- Synchronized the validated tree to the active remote preview, rebuilt API,
  worker, and web containers, confirmed healthy dependencies, and passed the
  opt-in Playwright stream test against the configured live provider.
- At this checkpoint, Phase 5 was complete locally and M3 remained untagged
  pending an owner-authorized commit/push and green GitHub CI; no changes had been
  pushed.

### Phase 5 conversations and realtime checkpoint

- Closed M3-A06/M3-A07 with durable conversation create/delete idempotency,
  ownership/IDOR enforcement, archived/search filters, stable keyset pagination,
  idle-state mutation guards, message pagination, ETag revalidation, and HTTP
  authentication/CSRF/error coverage.
- Closed M3-A08 by covering Redis failure behavior in addition to ticket expiry,
  actor binding, single use, and replay denial.
- Added lost replay-window detection and `connection.resync_required`, client-side
  resource rehydration, disallowed-origin coverage, bounded-queue coverage, and
  cross-broker Redis pub/sub evidence for M3-A09.
- Verified migration `00007` through `down/up/validate`, backend race tests,
  PostgreSQL integration, realtime integration, and frontend typecheck/lint/tests.

### Realtime client envelope fix

- Traced the misleading chat quota alert to `invalid_command` events emitted for
  heartbeat, resume, and cancel envelopes that omitted contract-required fields.
- Centralized client WebSocket envelope construction with an idempotency key on
  every command and preserved the server heartbeat ID in pong responses.
- Scoped visible generation errors to rejected `chat.generate` commands and mapped
  quota/concurrency codes separately from generic conflicts.
- Verified frontend typecheck, lint, 18 unit tests, and the production Next.js build;
  rebuilt only the active web container.

### Phase 4 completed

- Closed MODEL-001 through MODEL-008 and accepted M3-A01 through M3-A05.
- Enforced active provider mappings and provider health in public model listing,
  default resolution, and selection; guests receive only their configured default.
- Added a migration that permits synchronized models to become unavailable while
  preserving exposure constraints and provides the typed
  `quota.global.output_tokens` runtime setting.
- Added provider health, inter-chunk idle timeout, bounded concurrency, circuit
  recovery, and no-replay-after-partial-output coverage.
- Verified `go test -race ./...`, model SQL integration, migration
  `down/up/validate`, and the opt-in configured-provider smoke on the isolated
  development server.
- Phase 4 is complete; M3 remains open because it also owns Phase 5.

### Progress model clarified

- Defined milestone ownership for all delivery phases:
  `M0→Phase 0`, `M1→Phase 1`, `M2→Phases 2-3`, `M3→Phases 4-5`,
  `M4→Phases 6-8`, `M5→Phase 9`, and `M6→Phase 10`.
- Classified Phase 11 as post-MVP backlog outside the M0-M6 release plan.
- Added separate implementation, acceptance, and release status to the milestone
  dashboard in `TASKS.md`.
- Added explicit phase ownership to every milestone progress document and created
  the M6/Phase 10 release-gate document.
- Added the M3-A01 through M3-A17 acceptance ledger and linked Phase 4/5 tasks to
  their gates.
- Corrected MODEL-004 and MODEL-006 to complete based on existing automated
  evidence.

### M4 acceptance

- Added isolated Playwright execution on the development server without changing
  the active preview stack.
- Verified browser-locale fallback, authenticated locale persistence, conversation
  rename/archive/restore/delete, and current/other session revocation.
- Added `PATCH /me` locale persistence and generated contract/store changes.
- Verified contract lint, frontend typecheck/lint/unit tests, and
  `go test -race ./...` on the development server.

### Next focus

Begin Phase 5 with M3-A06 conversation ownership/pagination and M3-A07 HTTP
contract behavior, then continue through the realtime and generation lifecycle
gates.
