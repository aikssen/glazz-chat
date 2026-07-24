# M4 Web Application Progress

## Status

M4 is in progress and is not tagged. The release tag remains reserved as
`v0.4.0` until Phase 6 through Phase 8 acceptance criteria and CI are green.
M3 also remains untagged while its acceptance hardening is completed.

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
- Safe Markdown/GFM rendering, code copy, PWA manifest, and navigation-only offline
  fallback without API or transcript caching.

## Verification completed

- OpenAPI and AsyncAPI lint plus 14 WebSocket fixture validations.
- Next.js production build, TypeScript, ESLint, and Prettier checks.
- Ten frontend unit tests, including streaming idempotency and theme contrast.
- Nine Playwright E2E checks across the responsive viewport matrix; the guest-limit
  case runs once on mobile and verifies all four streamed prompts plus the login
  gate.
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

## Remaining before `v0.4.0`

- Finish the M3 acceptance cases listed in `docs/m3-chat-backend.md` and publish
  `v0.3.0` first.
- Add deterministic Google OAuth stub E2E coverage for consent, denial, callback,
  exactly-once guest migration, and restored deep links.
- Complete authenticated E2E coverage for conversation ownership, sessions,
  deletion, recent-auth recovery, and every administration surface.
- Add axe, keyboard, focus, IME, malicious Markdown, 200% zoom, and screen-reader
  announcement coverage.
- Complete locale parity/fallback automation and bilingual legal draft routing.
- Add visible PWA offline/update states and the jump-to-latest transcript control.
- Run full repository presubmit and CI, then create the annotated `v0.4.0` tag only
  after every M4 acceptance item is green.
