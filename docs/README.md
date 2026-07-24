# Glazz Documentation

This directory contains the explanatory, decision, delivery, and operational
documentation for Glazz. The documents are designed for four audiences:

- evaluators reviewing the project as an architecture and engineering portfolio;
- contributors implementing or reviewing a vertical slice;
- operators running development, staging, or production environments;
- product and security reviewers validating scope and release readiness.

## Start here

| Goal                                                     | Read                                                                    |
| -------------------------------------------------------- | ----------------------------------------------------------------------- |
| Understand the product and run it                        | [Root README](../README.md)                                             |
| Review features, roles, and product constraints          | [Product Capabilities](./product-capabilities.md)                       |
| Review system boundaries, flows, tradeoffs, and diagrams | [Technical Architecture](./technical-architecture.md)                   |
| Review the relational model and lifecycle                | [Data Model and ERD](./data-model.md)                                   |
| Review security controls and the production plan         | [Security and Production Readiness](./security-production-readiness.md) |
| Understand local development                             | [Development Foundation](./development.md)                              |
| Execute work on the shared server                        | [Remote Development Runbook](./runbooks/remote-development.md)          |
| Review current delivery status                           | [Canonical Task Tracker](../TASKS.md)                                   |

## Canonical sources

Glazz separates normative sources from explanatory views so duplicated prose does
not silently redefine the product.

| Source                                  | Authority                                                                     |
| --------------------------------------- | ----------------------------------------------------------------------------- |
| [`PROJECT.md`](../PROJECT.md)           | Product scope, roles, business rules, goals, non-goals, release gates         |
| [`ARCHITECTURE.md`](../ARCHITECTURE.md) | Detailed runtime boundaries, protocols, data responsibilities, and invariants |
| [`DESIGN.md`](../DESIGN.md)             | Information architecture, visual system, interaction behavior, accessibility  |
| [`TASKS.md`](../TASKS.md)               | Canonical milestone/phase relationship, task status, dependencies, acceptance |
| [`AGENTS.md`](../AGENTS.md)             | Mandatory contributor and coding-agent rules                                  |
| OpenAPI and AsyncAPI                    | Executable HTTP and realtime interface contracts                              |
| SQL migrations                          | Executable database schema                                                    |

The portfolio-oriented documents in this directory explain those sources with
diagrams and review narratives. When an explanatory document conflicts with an
executable contract or canonical source, the contract/canonical source wins and
the explanatory document must be corrected.

## Architecture decisions

Architecture Decision Records capture decisions that are difficult or expensive to
reverse:

| ADR                                                 | Decision                                    |
| --------------------------------------------------- | ------------------------------------------- |
| [ADR-0001](./adr/0001-http-router-and-websocket.md) | chi and coder/websocket transport selection |
| [ADR-0002](./adr/0002-contract-toolchain.md)        | OpenAPI/AsyncAPI validation and generation  |
| [ADR-0003](./adr/0003-jwt-signing-and-rotation.md)  | JWT signing and rotation model              |
| [ADR-0004](./adr/0004-goroutine-leak-detection.md)  | Goroutine leak detection in verification    |

Decisions still required before production include hosting, the production LLM
provider, error tracking, backup/restore design, and production scaling policy.

## Milestone evidence

| Milestone | Scope                                | Evidence                                              |
| --------- | ------------------------------------ | ----------------------------------------------------- |
| M0        | API and architecture contract        | [M0 contract baseline](./m0-contract-baseline.md)     |
| M1        | Monorepo and engineering foundation  | [M1 foundation](./m1-foundation.md)                   |
| M2        | Platform, data, identity             | [M2 platform and identity](./m2-platform-identity.md) |
| M3        | Complete chat backend                | [M3 chat backend](./m3-chat-backend.md)               |
| M4        | Web application and administration   | [M4 web application](./m4-web-application.md)         |
| M5        | Release candidate hardening          | [M5 release candidate](./m5-release-candidate.md)     |
| M6        | Production infrastructure and launch | [M6 production launch](./m6-production-launch.md)     |

Milestone documents are acceptance ledgers. They do not replace `TASKS.md`.
`WORKLOG.md` records chronological evidence and never defines pending scope.

## Supporting references

- [API Glossary](./api-glossary.md)
- [Dependency and Documentation Policy](./dependency-policy.md)
- [M0 Threat Model](./threat-model/m0-threat-model.md)
- [Deployment Notes](../deploy/README.md)

## Documentation still required

The documentation baseline is current through the start of M5. The following
artifacts must be produced from real acceptance evidence rather than written in
advance as if the work had passed.

### M5 evidence

- visual regression matrix and approved baseline procedure;
- application security review with finding severity, ownership, and closure;
- realtime load/soak model, results, bottlenecks, and capacity budget;
- dependency failure matrix and recovery evidence;
- privacy/deletion inspection report covering logs, traces, analytics, and stores;
- web bundle/Core Web Vitals performance report;
- final implementation-to-OpenAPI/AsyncAPI compatibility report.

### M6 decisions and runbooks

- ADR for production API/PostgreSQL/Redis hosting;
- ADR for the approved production LLM provider;
- environment provisioning and secret-rotation procedures;
- migration, rollout, rollback, and release runbook;
- backup, point-in-time recovery, and tested restore runbook;
- incident runbooks for provider, database, Redis, authentication, realtime, data
  leakage, and deletion backlog failures;
- dashboards, alert thresholds, SLO/error-budget, and on-call ownership;
- data-retention schedule and legally approved Terms/Privacy Policy;
- production architecture diagram updated with selected vendors, regions, and
  recovery topology.

These are tracked by Phase 9 and Phase 10 in `TASKS.md`. They should link their
evidence from the M5/M6 acceptance ledgers when completed.

## Diagram convention

Architecture diagrams are committed as Mermaid source inside Markdown:

- `flowchart` for context, container, trust-boundary, and deployment views;
- `sequenceDiagram` for stateful cross-component interactions;
- `erDiagram` for relational ownership and cardinality.

Diagrams are explanatory. Names should match the contracts and SQL schema, and
secrets or production identifiers must never appear in diagram source.
