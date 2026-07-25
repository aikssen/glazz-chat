# Phase 9 Visual and Contract Verification

- Date: 2026-07-24
- Tasks: QA-005, QA-010
- Result: Accepted

## Visual regression matrix

Playwright stores Linux baselines beside
`apps/web/tests/e2e/visual.spec.ts`. The matrix contains 20 images:

| State                    | Locale  | Theme | 375  | 768  | 1024 | 1440 |
| ------------------------ | ------- | ----- | ---- | ---- | ---- | ---- |
| Empty chat               | Spanish | Light | Pass | Pass | Pass | Pass |
| Completed chat           | English | Dark  | Pass | Pass | Pass | Pass |
| Controlled startup error | Spanish | Dark  | Pass | Pass | Pass | Pass |
| Guest quota gate         | English | Light | Pass | Pass | Pass | Pass |
| Administration           | Spanish | Light | Pass | Pass | Pass | Pass |

Two consecutive production-build runs passed: one created the reviewed baselines
and the second produced no diff. Representative mobile, tablet, desktop, quota,
dark-theme, and administration images were inspected at original resolution. No
overlap, clipping, horizontal overflow, unstable control size, or obscured content
was found.

Update baselines only after reviewing the rendered state:

```bash
pnpm e2e:visual:update
pnpm e2e:visual
```

Both commands use the committed `deploy/compose.e2e.yaml` override and
`scripts/run-visual-e2e.sh`. The runner creates a loopback-only, fake-provider,
disposable stack and removes its containers and PostgreSQL volume on every exit.
A normal run without update mode is the acceptance gate. CI and local Linux
runners must not silently approve new images.

The permanent runner was revalidated on 2026-07-25 after the Signal Workspace CI
follow-up: all 20 comparisons passed, teardown left no E2E containers or volumes,
and the persistent development API remained ready. The associated
foundation/performance/workspace subset passed 30 applicable cases across the four
configured viewports.

## Contract compatibility

The final comparison covers:

- OpenAPI 3.1 against the chi runtime router;
- AsyncAPI events against browser and server implementations;
- generated Go and TypeScript clients;
- six WebSocket fixtures containing 13 events;
- runtime-only development and transport routes.

`pnpm contracts:lint` reports 35 OpenAPI routes, 37 runtime routes, and two
reviewed runtime-only routes:

| Runtime-only route                | Reason                                        |
| --------------------------------- | --------------------------------------------- |
| `GET /api/v1/ws`                  | WebSocket handshake is described by AsyncAPI  |
| `GET /api/v1/auth/test/authorize` | Development-only deterministic OAuth endpoint |

The AST-based `contracts:coverage` gate fails on any other undocumented runtime
route or any documented route missing from the router.

The review removed two AsyncAPI events that had no producer
(`quota.updated`, `maintenance.changed`) and added browser handling for the
implemented `conversation.updated` event. Regeneration completed with no diff in
the committed generated clients. No incompatible HTTP or realtime drift remains.
