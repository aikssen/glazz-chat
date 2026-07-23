# ADR 0002: Contract Validation and Generation

- Status: Accepted for M0
- Date: 2026-07-23

## Context

Glazz requires OpenAPI 3.1 for HTTP and AsyncAPI 3.0 for WebSocket messages. Go
server bindings and TypeScript client types must be deterministic and generated
from contracts without sharing runtime business logic.

Context7 research used `/oapi-codegen/oapi-codegen`,
`/redocly/redocly-cli`, and `/openapi-ts/openapi-typescript` on 2026-07-23.

## Decision

- Author `packages/contracts/openapi.yaml` as OpenAPI 3.1.0.
- Author `packages/contracts/websocket.asyncapi.yaml` as AsyncAPI 3.0.0.
- Lint and bundle both with Redocly CLI v2.
- Generate chi strict-server types/bindings with `oapi-codegen/v2`.
- Generate runtime-free TypeScript types with `openapi-typescript` v7.
- Handwrite small HTTP/WebSocket transport adapters against generated types.
- Validate canonical JSON fixtures against the schemas in CI.

Exact patch versions are pinned in Phase 1 after repeating Context7 and upstream
release checks.

## Consequences

- Contracts remain the source of truth.
- Generated code is reproducible and never manually edited.
- Unique stable `operationId` values are mandatory.
- AsyncAPI event runtime validation needs a selected JSON Schema validator in Phase
  1; generated TypeScript types alone are not runtime validation.

## Alternatives

- Protobuf/gRPC: rejected for browser-first public HTTP semantics and MVP scope.
- Swagger Codegen/OpenAPI Generator: broader output but more generated surface than
  needed.
- Handwritten DTOs: rejected because drift would undermine API-first delivery.
- A single generator for every language/protocol: rejected; specialized tools have
  clearer outputs and independent replaceability.

## Verification

- Redocly lint has zero errors.
- Every fixture validates.
- Generation twice yields no diff.
- Handler and client contract tests consume canonical examples.

