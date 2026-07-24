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
