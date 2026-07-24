# M5 Release Candidate Progress

## Status

M5 verification is in progress and is not tagged. Acceptance work may validate and
harden M3/M4 behavior, but it does not waive either earlier milestone gate.

## Completed

- Frontend accessibility suite for guest and legal routes using axe WCAG A/AA tags.
- Keyboard focus containment, Escape dismissal, and trigger-focus restoration for
  application dialogs.
- IME composition, inert malicious Markdown, responsive viewport, and 200% reflow
  browser checks.
- Deterministic OAuth consent, denial, one-time callback, guest migration,
  authenticated settings/admin, and account-deletion journeys.
- PWA offline-state browser coverage against the production container.
- Next.js typecheck, lint, 12 unit tests, and production build.
- Go race suite and PostgreSQL/Redis integration suite.

## In Progress

- `QA-004`: complete conversation lifecycle, cancellation/retry, ownership,
  session-revocation, and recent-auth Playwright journeys.
- M3 backend acceptance expansion for state transitions, cancellation/reconnect,
  retry concurrency, context/summary/title behavior, safety policy hooks, and usage
  reconciliation.

## Remaining

- `QA-001`, `QA-002`: broaden backend and concurrency/leak coverage.
- `QA-005`: capture and approve the complete visual regression matrix.
- `SEC-002`: execute the application security review and own every finding.
- `QA-006`, `QA-007`: realtime load/soak and dependency-failure matrices.
- `QA-008`: privacy/deletion evidence review.
- `QA-009`: bundle, hydration, CLS, and long-transcript performance audit.
- `QA-010`: final implementation-to-contract compatibility review.
- Run the complete presubmit and CI before creating `v0.5.0`.
