# M5 Release Candidate Progress

## Ownership

- **Milestone:** M5
- **Owned phase:** Phase 9
- **Reserved release:** `v0.5.0`

## Status

Phase 9 and all local M5 acceptance gates completed on 2026-07-24. Release-candidate
commit `ecee24b` was pushed to `main` on 2026-07-25. CI run `30176672297` passed
presubmit and security; E2E exposed an ambiguous transcript selector, a flaky
service-worker-backed user mock, and wide-screen CLS above budget. The follow-up is
implemented locally and passes the affected 30-case production-browser subset plus
all 20 visual comparisons. M5 remains untagged and unpublished until that follow-up
is pushed and its GitHub CI rerun is green.

M4 was published as `v0.4.0` after its E2E, presubmit, and security jobs completed
successfully.

## Acceptance ledger

| ID     | Task          | Acceptance                                               | Status   | Evidence                                                                                              |
| ------ | ------------- | -------------------------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------- |
| M5-A01 | QA-001        | Backend unit and real PostgreSQL/Redis integration suite | Accepted | Full isolated integration suite covers repositories, handlers, jobs, adapters, and concurrency        |
| M5-A02 | QA-002        | Race and goroutine/stream leak testing                   | Accepted | Full integration under `-race`; ten 24-connection reconnect cycles with `goleak`                      |
| M5-A03 | QA-003        | Frontend component and accessibility suite               | Accepted | 29 unit tests plus axe, keyboard, focus, IME, Markdown, localization, and 200% reflow E2E             |
| M5-A04 | QA-004        | Deterministic Playwright E2E suite                       | Accepted | Final Phase 9 run: 49 applicable cases passed across four viewports; 63 opt-in/non-applicable skipped |
| M5-A05 | QA-005        | Complete visual regression matrix                        | Accepted | 20 reviewed Linux baselines; committed isolated runner repeated the matrix with zero diff and cleanup  |
| M5-A06 | SEC-002       | Application security review                              | Accepted | No open critical/high; browser headers fixed; medium production items assigned to Phase 10            |
| M5-A07 | QA-006        | Realtime load and soak                                   | Accepted | 1,280 race-enabled connections; 14.17 ms p95; bounded heap and zero goroutine delta                   |
| M5-A08 | QA-007        | Dependency failure matrix                                | Accepted | PostgreSQL, Redis, provider, telemetry, worker/outbox, and network behavior verified                  |
| M5-A09 | QA-008        | Privacy and deletion validation                          | Accepted | Content-free signals, 24-hour SLA, personal purge, anonymous aggregates, and guest cleanup verified   |
| M5-A10 | QA-009/QA-010 | Web performance and contract compatibility               | Accepted | CLS <= 0.0311; JS 238,867 bytes; 35 documented HTTP routes and reviewed runtime exceptions            |

## Verification completed

- OpenAPI and AsyncAPI lint with generated-artifact drift rejection.
- Full Go unit and integration suites under the race detector against isolated
  PostgreSQL and Redis.
- WebSocket lifecycle joins for read, write, Redis subscription, and heartbeat
  loops, with `goleak` verification after reconnect load.
- Twenty-nine frontend unit tests.
- Deterministic Playwright coverage for guest limits, OAuth migration, registered
  conversations, cancellation/retry, reconnect, sessions, deletion,
  administration, themes, locales, responsive behavior, accessibility, and PWA.
- LAN preview smoke for connected realtime state and a non-deterministic response
  from the configured live provider without automating production-like OAuth.
- Production Next.js and Go builds.
- Phase 9 evidence reports:
  - [Visual and contract verification](./reports/phase9-visual-contract.md)
  - [Application security review](./reports/phase9-security-review.md)
  - [Resilience and privacy verification](./reports/phase9-resilience-privacy.md)
  - [Web performance audit](./reports/phase9-web-performance.md)

## Release sequence

1. Completed: commit and push the Phase 9 release candidate as `ecee24b`.
2. Completed locally: correct the E2E regressions from CI run `30176672297` and
   verify the affected subset plus the visual matrix.
3. Pending: push the follow-up and require presubmit, E2E, and security jobs to
   pass.
4. Create and push annotated `v0.5.0`.
5. Start Phase 10 only after the published M5 gate.
