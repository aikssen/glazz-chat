# M5 Release Candidate Progress

## Ownership

- **Milestone:** M5
- **Owned phase:** Phase 9
- **Reserved release:** `v0.5.0`

## Status

M5 and Phase 9 started on 2026-07-24. Functional backend, frontend, accessibility,
and deterministic browser gates are accepted. Security, visual, load, failure,
privacy, performance, and contract gates remain open. M5 is not tagged.

M4 was published as `v0.4.0` after its E2E, presubmit, and security jobs completed
successfully.

## Acceptance ledger

| ID | Task | Acceptance | Status | Evidence |
| --- | --- | --- | --- | --- |
| M5-A01 | QA-001 | Backend unit and real PostgreSQL/Redis integration suite | Accepted | Full isolated integration suite covers repositories, handlers, jobs, adapters, and concurrency |
| M5-A02 | QA-002 | Race and goroutine/stream leak testing | Accepted | Full integration under `-race`; ten 24-connection reconnect cycles with `goleak` |
| M5-A03 | QA-003 | Frontend component and accessibility suite | Accepted | 26 unit tests plus axe, keyboard, focus, IME, Markdown, localization, and 200% reflow E2E |
| M5-A04 | QA-004 | Deterministic Playwright E2E suite | Accepted | Responsive standard suite and guest-edge profile pass locally and in the M4 CI E2E job |
| M5-A05 | QA-005 | Complete visual regression matrix | Open | State/theme/locale matrix and approved baselines remain |
| M5-A06 | SEC-002 | Application security review | Open | Threat-driven manual and automated review remains |
| M5-A07 | QA-006 | Realtime load and soak | Open | Load model, budgets, soak, and graceful degradation evidence remain |
| M5-A08 | QA-007 | Dependency failure matrix | Open | PostgreSQL, Redis, provider, telemetry, worker, and network scenarios remain |
| M5-A09 | QA-008 | Privacy and deletion validation | Open | Content-flow inspection and retention/SLA evidence remain |
| M5-A10 | QA-009/QA-010 | Web performance and contract compatibility | Open | Bundle/CLS/long-transcript audit and final contract inventory remain |

## Verification completed

- OpenAPI and AsyncAPI lint with generated-artifact drift rejection.
- Full Go unit and integration suites under the race detector against isolated
  PostgreSQL and Redis.
- WebSocket lifecycle joins for read, write, Redis subscription, and heartbeat
  loops, with `goleak` verification after reconnect load.
- Twenty-six frontend unit tests.
- Deterministic Playwright coverage for guest limits, OAuth migration, registered
  conversations, cancellation/retry, reconnect, sessions, deletion,
  administration, themes, locales, responsive behavior, accessibility, and PWA.
- LAN preview smoke for connected realtime state and a non-deterministic response
  from the configured live provider without automating production-like OAuth.
- Production Next.js and Go builds.

## Remaining sequence

1. QA-005 visual state/theme/locale regression matrix.
2. SEC-002 application security review and finding ownership.
3. QA-006 realtime load/soak, followed by QA-007 dependency failure testing.
4. QA-008 privacy and deletion evidence review.
5. QA-009 web performance and QA-010 final contract compatibility.
6. Complete CI and create `v0.5.0` only when every acceptance row is green.
