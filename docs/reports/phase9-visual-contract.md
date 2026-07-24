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
E2E_BASE_URL=http://localhost:13000 \
E2E_API_URL=http://localhost:18080 \
E2E_OAUTH=true \
pnpm --filter @glazz/web exec playwright test \
  tests/e2e/visual.spec.ts --update-snapshots
```

A normal run without `--update-snapshots` is the acceptance gate. CI and local
Linux runners must not silently approve new images.

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
