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

## 2026-07-26

### M5 published as v0.5.0

- Confirmed GitHub Actions run `30211515823` passed presubmit, E2E, and security
  on release commit `55aa8cb`; CodeQL also passed.
- Created and pushed annotated tag `v0.5.0` with message
  `Glazz v0.5.0 - integrated release candidate`.
- Closed the M5 publication gate. Phase 10/M6 remains the next release milestone;
  Glazz is not approved for public production traffic.

## 2026-07-26

### Public-history references reconciled and M5 CI accepted

- Reconciled milestone references after the public-history rewrite: `v0.2.0`
  points to `8a8b3fd`, `v0.4.0` points to `246ce8e`, and the rewritten M5
  release-candidate baseline is `af7076a`.
- Fixed the concurrent retry integration test's Redis lease-settlement race in
  commit `6c5ec18`, preserving the durable terminal-state assertions while
  tolerating the bounded cleanup window.
- GitHub Actions run `30210682163` passed presubmit, E2E, and security on
  `6c5ec18`.
- Updated the canonical tracker and M5 ledger to record complete M5 acceptance.
  The annotated `v0.5.0` tag remains the only pending M5 release action.

## 2026-07-25

### Signal Workspace published and isolated visual workflow standardized

- Published Signal Workspace release-candidate commit `ecee24b` to `main`,
  including the reviewed redesign, responsive navigation/context behavior,
  accessibility coverage, and 20 updated Linux visual baselines.
- Replaced repeated `/tmp` Compose overrides with committed
  `deploy/compose.e2e.yaml` and `scripts/run-visual-e2e.sh`.
- Added `pnpm e2e:visual` and `pnpm e2e:visual:update`; the runner uses
  loopback-only ports, `.env.test.example`, deterministic OAuth, a fake provider,
  a separate `glazz-e2e` project, failure logs, and unconditional volume cleanup.
- Ran the permanent visual command on the remote execution server: all 20 cases
  passed, no E2E containers or volumes remained, and the persistent development
  stack stayed healthy.
- CI run `30176672297` passed presubmit and security. Its E2E job exposed an
  ambiguous transcript selector, a service-worker-sensitive administrator mock,
  and wide-screen CLS of approximately `0.108`; the follow-up preserves the
  `<0.1` budget by reserving the context lane before hydration.
- Replayed the affected production-browser subset after the fixes: 30 applicable
  cases passed across four viewports, including performance, login/admin, and
  responsive workspace behavior. The permanent visual command then passed all 20
  comparisons again and removed every E2E container and volume; the persistent
  API remained ready.

## 2026-07-24

### Phase 9 completed locally

- Closed QA-005 through QA-010 and SEC-002 with linked evidence in
  `docs/reports/`; all local M5 acceptance rows are green.
- Added and reviewed 20 visual baselines covering five states across 375, 768,
  1024, and 1440 pixel viewports; a second run produced no visual diffs.
- Added an AST-based OpenAPI/runtime route coverage gate, removed two unimplemented
  AsyncAPI events, and handled `conversation.updated` in the browser.
- Added Next.js CSP and security headers, disabled `X-Powered-By`, passed dependency,
  vulnerability, secret, race, XSS, and browser-policy checks, and assigned three
  medium infrastructure hardening items to Phase 10.
- Passed a 1,280-connection WebSocket soak under `-race` with 14.17 ms handshake
  p95, 472,656-byte heap delta, and zero goroutine delta.
- Added real PostgreSQL/Redis readiness failure tests, outbox retry/dead-letter
  evidence, and OTLP collector failure isolation.
- Revalidated account/guest purge, anonymous aggregate retention, redacted audits,
  and content-free telemetry.
- Added production-browser performance budgets. Mobile/wide CLS remained at or
  below 0.0311, encoded JavaScript was 238,867 bytes, and 200-message layout/scroll
  completed within 17 ms without overflow or console errors.
- Corrected the previously accepted code-rendering gap by adding pinned
  `highlight.js` core with nine registered languages and XSS escape tests.
- Passed the complete 112-profile Playwright run with 49 applicable cases green
  and 63 explicit opt-in/non-applicable skips, the full monorepo presubmit, and all
  Go integration packages sequentially under `-race` against isolated services.

### Technical documentation consolidated

- Added a detailed repository `README.md` covering the product, capabilities,
  architecture, exact stack, local/remote execution, provider and Google OAuth
  configuration, tests, security, observability, and troubleshooting.
- Added a documentation index plus portfolio-oriented capability, technical
  architecture, data-model/ERD, and security/production-readiness documents.
- Added 20 Mermaid context, container, component, sequence, state, trust,
  deployment, and ER diagrams derived from the contracts and SQL migrations.
- Rendered every Mermaid block successfully with Mermaid CLI `11.16.0`, checked
  all new relative links, and formatted the complete changed documentation set
  with the repository-pinned Prettier version.
- Corrected the stale Phase 8/M4 release status and linked canonical sources to the
  new review-oriented documentation.

### M4 published

- GitHub Actions run `30118902670` passed E2E, presubmit, and security jobs for
  commit `009efc9`.
- Created and pushed annotated milestone tag `v0.4.0`.

### Phase 9 verification and hardening started

- Formally started M5/Phase 9 and accepted QA-001 through QA-004 from current
  backend, frontend, accessibility, and deterministic browser evidence.
- Changed the WebSocket handler to cancel and join all per-connection loops before
  returning.
- Added pinned `go.uber.org/goleak` `v1.3.0` coverage to the 24-connection
  reconnect test after verifying its API through Context7 and its versions through
  the official Go module proxy.
- Passed ten reconnect/leak cycles under the race detector and the complete
  race-enabled integration suite against isolated PostgreSQL and Redis services.
- Updated the LAN smoke to validate the mobile connection indicator and to automate
  OAuth only in deterministic test mode; the deployed preview passed realtime and
  live-provider chat verification with production-like OAuth configuration.

### Phase 8 completed

- Closed WEB-011 and accepted M4-A07. All Phase 8 tasks are complete.
- Replaced the hardcoded 30-day guest lifetime with validated
  `GUEST_SESSION_TTL` configuration and added deterministic fake-provider usage
  configuration for boundary testing.
- Proved in isolated browser runs that a 1,999-token response leaves one token,
  the next response consumes exactly that remainder, and the login gate activates
  at 2,000 tokens. Proved that an expired session returns with an empty
  conversation and a fresh four-message/2,000-token allowance.
- Passed the 25-case standard responsive browser suite, two guest-edge browser
  cases, full presubmit, race-enabled Go tests, and the complete PostgreSQL/Redis
  integration suite against isolated services.

### Phase 8 registered and administration journeys accepted

- Closed WEB-013, WEB-014, WEB-017, and WEB-018 after adding cursor pagination,
  reconnect/offline recovery, authoritative reload after administration conflicts,
  and explicit non-administrator denial coverage.
- Verified OAuth migration, deep-link reload, ownership denial, cancel/retry,
  exactly-once completion after reconnect, administration conflict messaging, and
  absence of conversation content on denied administration routes.
- Passed 25 applicable Playwright cases across four viewports against isolated
  PostgreSQL/Redis, API, and standalone production web processes.

### Phase 8 integration acceptance started

- Accepted INT-002, WEB-012, WEB-015, and WEB-016 from contract, integration, and
  isolated production-build browser evidence.
- Changed PWA E2E startup to exercise `next build` plus the standalone production
  server when service-worker acceptance is enabled.
- Corrected mobile sidebar modal timing, initial dialog focus after applying
  `inert`, and the loading skeleton's ARIA semantics.
- Added deterministic test-provider latency so browser tests can cancel a running
  generation, exposed retry for cancelled responses, and verified that retry
  creates exactly one completed replacement.
- Corrected recent-auth return routing from the nonexistent `/settings/security`
  path to `/settings`; CI now expires recent auth and proves reauthentication before
  account deletion.
- Verified 24 applicable Playwright cases across four viewports against isolated
  PostgreSQL/Redis, API, and standalone web processes, plus the full repository
  presubmit.

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
