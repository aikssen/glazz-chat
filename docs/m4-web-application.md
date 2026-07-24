# M4 Web Application Progress

## Ownership

- **Milestone:** M4
- **Owned phases:** Phase 6, Phase 7, and Phase 8
- **Reserved release:** `v0.4.0`

## Status

M4 is in progress and is not tagged. The release tag remains reserved as
`v0.4.0` until Phase 6 through Phase 8 acceptance criteria and CI are green.
M3 also remains untagged while its acceptance hardening is completed.

The application is available as a testable preview while those acceptance cases
remain open. In `TASKS.md`, `[-]` means implemented with incomplete acceptance
coverage, not that the whole feature is still being built.

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
- Fourteen frontend unit tests, including streaming idempotency, dictionary parity,
  theme contrast, and UUID generation on insecure local-network origins.
- Sixteen Playwright E2E checks across the responsive viewport matrix. They cover
  the guest limit, OAuth consent/denial, exactly-once migration, restored deep links,
  authenticated settings, all administration views, account deletion, axe WCAG
  A/AA checks, keyboard focus, IME input, inert malicious Markdown, 200% reflow, and
  the PWA offline state.
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

## Remaining before `v0.4.0`

- Finish the M3 acceptance cases listed in `docs/m3-chat-backend.md` and publish
  `v0.3.0` first.
- Complete authenticated E2E coverage for ownership denial, generation
  cancellation/retry, and recent-auth recovery. Registered rename/archive/restore/
  delete, both session-revocation paths, browser-locale fallback, and authenticated
  locale persistence are now covered.
- Add explicit screen-reader announcement throttling and exercise the PWA waiting
  update state.
- Run full repository presubmit and CI, then create the annotated `v0.4.0` tag only
  after every M4 acceptance item is green.
