# M4 Web Application Progress

## Ownership

- **Milestone:** M4
- **Owned phases:** Phase 6, Phase 7, and Phase 8
- **Reserved release:** `v0.4.0`

## Status

M4 implementation and local acceptance are complete. The milestone is not tagged;
`v0.4.0` remains pending until CI runs green on the consolidated commit.
Phases 6, 7, and 8 are complete.

The application remains available as a testable preview until the release gate is
published.

## Acceptance ledger

| ID | Phase | Acceptance | Status | Evidence |
| --- | --- | --- | --- | --- |
| M4-A01 | 6 | Typed settings validate values, reject stale versions, invalidate cache, and audit changes | Accepted | Unit and PostgreSQL/Redis integration tests |
| M4-A02 | 6 | Model administration rejects invalid exposure and atomically preserves valid defaults | Accepted | PostgreSQL integration tests |
| M4-A03 | 6 | Role changes require recent auth and preserve an active administrator under concurrency | Accepted | HTTP and concurrent PostgreSQL integration tests |
| M4-A04 | 6 | Usage exposes aggregate tokens, failures, latency, and error codes; audit reads redact sensitive values | Accepted | SQL aggregation, redaction, and pagination tests |
| M4-A05 | 6 | Account deletion, purge, and guest cleanup preserve required anonymous aggregates | Accepted | Existing Phase 6 lifecycle integration tests |
| M4-A06 | 7 | Frontend foundation and design-system acceptance | Accepted | Unit tests, WCAG checks, responsive visual matrix, live-provider smoke, and PWA policy/offline tests |
| M4-A07 | 8 | Integrated user and administration journeys | Accepted | Standard responsive E2E plus isolated 2,000-token and expiry edge profiles |

## Test now

- Direct LAN URL: `http://192.168.1.10:3000`
- The composer becomes available when the status rail reads `Conectado`.
- Guest chat works immediately and enforces the configured short trial.
- Deterministic development login is available from the Google login flow.
- This environment is a development preview and is not the `v0.4.0` release.

## Implemented

- Typed runtime settings for maintenance, quotas, system prompt, safety categories,
  model visibility, and concurrency.
- Administration APIs and bilingual screens for models, runtime settings, users,
  aggregate usage, and redacted audit events.
- Idempotent account deletion request, immediate session revocation, durable purge,
  login blocking, and daily guest cleanup.
- Responsive Next.js chat application for 375, 768, 1024, and 1440 pixel viewports.
- Light, dark, and system themes with Spanish and English application dictionaries.
- Generated OpenAPI types consumed through the `@glazz/contracts` workspace package.
- WebSocket ticketing, heartbeat, reconnect/resume, ordered streaming reducer,
  cancellation, retry, and usage refresh.
- Guest chat with four-prompt gate, preserved transcript, explicit legal-consent
  dialog, and Google OAuth entry.
- Registered conversation create, search, select, rename, archive, restore, delete,
  model selection, and OAuth return deep link behavior.
- Settings for profile, preferences, sessions, logout, reauthentication, and account
  deletion.
- Authenticated locale persistence through `PATCH /me`, profile hydration on every
  browser session, and an English fallback for unsupported browser locales.
- Safe Markdown/GFM rendering, code copy, PWA manifest, and navigation-only offline
  fallback without API or transcript caching.
- Development-only deterministic OAuth approval and denial screens, with production
  configuration guards and one-time callback state.
- Visible offline/update states, focus-contained destructive/login dialogs, IME-safe
  composition, bilingual legal drafts, and jump-to-latest transcript navigation.

## Verification completed

- OpenAPI and AsyncAPI lint plus 14 WebSocket fixture validations.
- Next.js production build, TypeScript, ESLint, and Prettier checks.
- Twenty-six frontend unit tests, including streaming idempotency, dictionary parity,
  theme contrast, and UUID generation on insecure local-network origins.
- Twenty-seven applicable Playwright E2E checks across the responsive viewport
  matrix and guest-edge profile. They cover
  the guest limit, OAuth consent/denial, exactly-once migration, restored deep links,
  cursor pagination, authenticated settings, cancellation/retry, reconnect with one
  completed response, ownership denial, all administration views, conflict recovery,
  non-administrator denial without conversation content, recent-auth account
  deletion, axe WCAG A/AA checks, keyboard focus, IME input, inert malicious
  Markdown, 200% reflow, and PWA offline/update states.
- Guest-edge runs prove a 1,999-token response followed by an exactly one-token
  remainder, the login gate at 2,000 tokens, and renewal to a clean full allowance
  after session expiry.
- Isolated registered-user acceptance on the development server provisions separate
  PostgreSQL/Redis instances and ports. Seven mobile checks cover browser-locale
  fallback, cross-session locale persistence, conversation rename/archive/restore/
  delete, and revocation of both another session and the current session.
- `go test -race ./...`.
- PostgreSQL/Redis integration suites for model sync, administration, deletion,
  retained anonymous aggregates, and guest cleanup.
- Final API, worker, and web images build and become healthy in the remote Compose
  environment.

## Defect found during acceptance

Sequential guest prompts initially failed after a short first response because the
second quota reservation requested the full 2,000-token budget instead of the
remaining balance. The quota service now caps each reservation by model, actor, and
global remaining output budgets. SQL constraints remain the final concurrency
barrier. Unit and browser regression checks cover the correction.

Guest expiry and the exact output boundary could not be exercised reliably while
the API fixed the session lifetime at 30 days and the deterministic provider always
reported six tokens. `GUEST_SESSION_TTL` now configures the lifetime explicitly,
and `LLM_FAKE_OUTPUT_TOKENS` controls only fake-provider accounting. The fake
provider caps reported usage to the request's reserved maximum, matching the
provider contract and allowing the final one-token reservation to settle safely.

Direct LAN access initially left the composer offline because the browser bundle
targeted `localhost`, and message submission then failed because `randomUUID()` is
not exposed on insecure LAN origins. The client now derives the API host from the
browser location and uses a Web Crypto UUID v4 fallback. Browser regressions cover
both a streamed guest response and the guest login gate through the LAN URL.

The first remote preview also retained the server's ignored `.env` with
`LLM_PROVIDER_KIND=fake`, so its usage counters represented deterministic test
usage rather than OpenCode consumption. Remote setup now documents the separate
secret sync and container recreation. An opt-in live-provider browser test rejects
the deterministic response and validates the complete streamed path.

Synchronizing the environment rotated the cookie-signing key and exposed a session
recovery defect: stale user cookies forced WebSocket ticket requests through the
wrong CSRF policy and left the composer offline. Refresh now clears cookies signed
by an obsolete key, actor-aware CSRF falls back to the valid guest session, and the
client performs one bounded recovery attempt. A browser regression covers the
deployment-key rotation.

Model-catalog inspection later found that the configured OpenAI-compatible gateway
was still represented by the deterministic `fake` provider record. Synchronization
therefore refreshed only the seeded DeepSeek mapping and ignored newly advertised
models. Repeated integration tests had also left 33 model and 13 provider fixtures
because cleanup ran after their pools closed or before dependent generations were
removed. Runtime registration now uses a separate `configured` provider, sync
imports unknown chat models without auto-enabling future discoveries, and test
cleanup is ordered and repeatable. The development catalog was transactionally
cleaned and the 23 models currently advertised by OpenCode Go were explicitly
enabled for registered users.

Phase 7 acceptance exposed two deployment-specific defects. The standalone Next.js
image omitted `public`, so the service worker was unavailable in the container;
the image now copies public assets beside the standalone application. The active
remote `.env` also targeted browser-local `localhost` while the preview used the
LAN address. The preview now follows the documented LAN origin configuration, and
responsive tests require a connected realtime state with no application error.

The initial PWA controller listener also reloaded the page when the first worker
claimed it. Reload is now gated by explicit acceptance of a waiting update.
Automated checks prove that only same-origin navigation is cached, API routes are
excluded, offline status is visible, and initial installation remains stable.

## Remaining before `v0.4.0`

- Consolidate the validated work in a local commit, run CI, and create the
  annotated `v0.4.0` tag only after CI is green.
