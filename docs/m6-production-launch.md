# M6 Production Launch Progress

## Ownership

- **Milestone:** M6
- **Owned phase:** Phase 10
- **Reserved release:** `v0.6.0`

## Status

M5 is published. M6 implementation has not started, but an initial single-node
capacity baseline is now available as pre-decision evidence. No production host
or LLM provider has been selected and no M6 task is accepted yet.

## Planning evidence

| Scope                                                                                                      | Evidence                                                                       | Effect on gate                                    |
| ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------- |
| Demo machine sizing, image footprint, memory, disk, HTTP, WebSocket, and fake-provider generation capacity | [Production demo capacity baseline](./reports/production-capacity-baseline.md) | Informs `PROD-001` and `PROD-010`; closes neither |

## Release gate

M6 closes only after every `PROD-001` through `PROD-012` task in `TASKS.md` is
complete. This includes an approved production LLM provider, reproducible
infrastructure, production OAuth and networking, deployment and rollback drills,
backup restore evidence, operational alerts/runbooks, legal approval, staging
rehearsal, controlled launch, and post-launch review.

Phase 11 is not part of M6 and cannot block `v0.6.0`.
