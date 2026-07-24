# Phase 9 Resilience and Privacy Verification

- Date: 2026-07-24
- Tasks: QA-006, QA-007, QA-008
- Result: Accepted

## Realtime load model

The opt-in integration profile runs 40 waves of 32 concurrent WebSocket
connections against real Redis. Each connection receives `connection.ready` and
closes cleanly. The profile runs under the race detector and `goleak`.

| Measure             |                Result |        Budget |
| ------------------- | --------------------: | ------------: |
| Connections         |                 1,280 |         1,280 |
| Elapsed             |                739 ms | Informational |
| Throughput          | 1,731.4 connections/s | Informational |
| Handshake p95       |              14.17 ms |         < 2 s |
| Heap delta after GC |         472,656 bytes |      < 64 MiB |
| Goroutine delta     |                     0 |         <= 16 |

Run it explicitly:

```bash
GLAZZ_RUN_SOAK=true go test -tags=integration -race \
  ./internal/realtime -run TestWebSocketReconnectSoak -count=1 -v
```

The regular suite retains a fast 24-connection reconnect case. Separate tests
prove bounded client queues, heartbeat cleanup, Redis replay/resync, cancellation
joins, provider first-chunk/idle timeouts, partial-stream handling, circuit
recovery, and provider concurrency admission.

This is release-candidate evidence, not a purchased-capacity forecast. PROD-002 and
PROD-010 must repeat the profile with the selected provider, production network,
replica count, and expected concurrency.

## Dependency failure matrix

| Failure                                  | Expected behavior                                           | Evidence                                 |
| ---------------------------------------- | ----------------------------------------------------------- | ---------------------------------------- |
| PostgreSQL unavailable                   | Readiness 503; liveness remains process-only                | Closed-pool integration test             |
| Redis unavailable                        | Readiness 503; quota/tickets fail closed                    | Closed-client integration and unit tests |
| Provider timeout                         | Stable retryable timeout before/within stream               | OpenAI-compatible adapter tests          |
| Provider disconnect/malformed/rate limit | Normalize error; never replay partial output                | Adapter and resilient-gateway tests      |
| Provider repeated failure                | Circuit opens, health degrades, recovers after window       | Resilient-gateway tests                  |
| Telemetry collector unavailable          | HTTP request still succeeds; export failure is non-critical | OTLP-unavailable unit test               |
| Worker handler failure                   | Durable retry, exponential delay, terminal dead-letter      | Real PostgreSQL outbox integration test  |
| Concurrent workers                       | One claim/receipt, no duplicate side effect                 | Real PostgreSQL outbox integration test  |
| Client/network disconnect                | Reconnect/resume or bounded resync; one transcript          | Realtime integration and browser E2E     |

No test acknowledged user data before losing its durable write. PostgreSQL and
Redis correctly remove the instance from readiness; external provider and
telemetry failures remain isolated from durable application state.

## Privacy and deletion

The real-database lifecycle test proves:

- the deletion request is idempotent;
- account status changes immediately and all sessions are revoked;
- login is blocked while deletion is pending;
- user, identity, sessions, conversations, and messages are purged;
- the durable job reaches `completed`;
- daily usage remains only as a non-content operational aggregate;
- expired unmigrated guests are removed;
- active and migrated guest records are not removed by the expiry pass.

The database constrains `due_at` to no later than 24 hours after request, and the
worker checks purge work hourly. The tested immediate run completes inside that
SLA.

Telemetry emits route templates, request/trace IDs, status, and duration. Safety,
server, worker, and outbox logs emit stage/category or error type only. Static log
call-site review found no prompt, response, system-prompt, cookie, token, OAuth
code, API key, raw email, or raw IP attributes. Administrative audit tests prove
system-prompt and authorization fields are redacted and conversation content is
absent.

Final log/trace retention periods and deletion from backups remain PROD-006 and
PROD-009 decisions because they depend on the selected production vendors and
approved legal policy.
