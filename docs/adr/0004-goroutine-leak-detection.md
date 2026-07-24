# ADR 0004: Goroutine Leak Detection

- Status: Accepted
- Decision date: 2026-07-24

## Context

Phase 9 requires repeatable evidence that WebSocket reconnects, cancellation, and
worker concurrency do not leave goroutines running after their owning operation
ends. Race detection finds unsafe access but does not prove lifecycle cleanup.
Raw `runtime.NumGoroutine` comparisons are sensitive to unrelated runtime and
dependency activity.

## Decision

Use `go.uber.org/goleak` `v1.3.0` in integration tests that own and explicitly
close all created resources. Snapshot pre-existing goroutines with
`goleak.IgnoreCurrent`, close the test server and Redis client, then call
`goleak.VerifyNone`.

The WebSocket handler must explicitly cancel and join its read, write,
subscription, and heartbeat loops before returning.

The library ID `/uber-go/goleak` and `VerifyNone`/`IgnoreCurrent` usage were
verified with Context7 on 2026-07-24. The pinned release list was verified through
the official Go module proxy with `go list -m -versions go.uber.org/goleak`.

## Consequences

- Leak checks run only in controlled, non-parallel lifecycle tests.
- Integration fixtures must expose idempotent cleanup so verification runs after
  owned resources close.
- `go test -race -tags=integration ./...` remains required because leak and race
  detection cover different failure classes.
