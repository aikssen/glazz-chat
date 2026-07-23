# Dependency and Documentation Policy

## Purpose

Glazz uses current, supported dependencies without allowing version drift or
unreviewed upgrades. This policy applies to languages, frameworks, libraries,
generators, CLIs, container images, GitHub Actions, and external API SDKs.

## Source order

1. Context7 library resolution and focused documentation query
2. Upstream official release notes, package registry, or language documentation
3. Security advisories and supported-version policy
4. An ADR when the dependency is architectural or security-sensitive

Context7 is available through the repository's `context7-cli` skill and may also be
exposed as an MCP tool by the active coding environment. The mechanism does not
change the source policy.

## Required workflow

1. State the concrete API, compatibility, or version question.
2. Run `ctx7 library <name> <query>` and select the canonical result.
3. Run `ctx7 docs <library-id> <focused-query>`.
4. Verify the stable release and support status upstream.
5. Prefer the latest supported stable release compatible with the selected stack.
6. Pin the exact version and checksum/lockfile.
7. Record the decision in an ADR or manifest comment when non-obvious.
8. Add automated update and vulnerability monitoring.

Never include secrets, internal payloads, private source, or personal data in a
Context7 query.

## M0 research record

Research date: 2026-07-23

| Concern | Context7 ID | Selected family | Evidence used |
| --- | --- | --- | --- |
| Go HTTP router | `/go-chi/docs` | `github.com/go-chi/chi/v5` | Standard `net/http`, composable middleware and route groups |
| Go WebSocket | `/coder/websocket` | `github.com/coder/websocket` current stable | Context cancellation, origin patterns, read limits, bounded slow-client pattern |
| Go OpenAPI generation | `/oapi-codegen/oapi-codegen` | `oapi-codegen/v2` | OpenAPI 3.1 and chi strict-server support |
| Contract linting | `/redocly/redocly-cli` | Redocly CLI v2 | OpenAPI and AsyncAPI 3.0 validation/linting |
| TypeScript contract types | `/openapi-ts/openapi-typescript` | `openapi-typescript` v7 | OpenAPI 3.1 runtime-free generated types |
| JWT | `/golang-jwt/jwt` | `golang-jwt/jwt/v5` | EdDSA plus strict algorithm/claim parser options |

Exact patch versions are intentionally pinned during Phase 1, immediately before
installation, after repeating the Context7/upstream check.

