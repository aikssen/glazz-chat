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

## Research record

Research date: 2026-07-23

| Concern                   | Context7 ID                      | Selected version          | Evidence used                                                                                   |
| ------------------------- | -------------------------------- | ------------------------- | ----------------------------------------------------------------------------------------------- |
| Go                        | official release source          | 1.26.5                    | Current supported Go release selected for API and worker                                        |
| Node.js                   | official release source          | 24.13.0 LTS               | Supported LTS runtime for Next.js and tooling                                                   |
| pnpm                      | `/pnpm/pnpm.io`                  | 11.17.0                   | Workspace and supply-chain policy support                                                       |
| Next.js                   | `/vercel/next.js`                | 16.2.11                   | App Router, React 19, and supported Node runtime                                                |
| React                     | `/websites/react_dev`            | 19.2.8                    | Version selected by the supported Next.js release                                               |
| TypeScript                | `/microsoft/typescript`          | 5.9.3                     | Current stable line supported by Next.js; TypeScript 7 is not selected before framework support |
| Tailwind CSS              | `/websites/tailwindcss`          | 4.3.3                     | CSS-first configuration and PostCSS integration                                                 |
| shadcn CLI                | `/shadcn-ui/ui`                  | 4.14.1                    | Base UI primitives, CSS variables, and Lucide integration                                       |
| Go HTTP router            | `/go-chi/docs`                   | `chi/v5` 5.3.1            | Standard `net/http`, composable middleware and route groups                                     |
| Go WebSocket              | `/coder/websocket`               | `coder/websocket` 1.8.15  | Selected in M0; installation begins with realtime implementation                                |
| Go OpenAPI generation     | `/oapi-codegen/oapi-codegen`     | 2.8.0                     | OpenAPI 3.1 and chi server generation                                                           |
| Contract linting          | `/redocly/redocly-cli`           | 2.40.0                    | OpenAPI and AsyncAPI validation                                                                 |
| TypeScript contract types | `/openapi-ts/openapi-typescript` | 7.13.0                    | Runtime-free OpenAPI 3.1 types                                                                  |
| AsyncAPI parsing          | `/asyncapi/parser-js`            | 3.6.0                     | Resolves message payload schemas before fixture validation                                      |
| JSON Schema validation    | `/ajv-validator/ajv`             | 8.20.0                    | Compiled payload validators with format support                                                 |
| Unit tests                | `/vitest-dev/vitest`             | 4.1.10                    | TypeScript-native fast unit runner                                                              |
| Browser tests             | `/microsoft/playwright`          | 1.61.1                    | Chromium smoke coverage with managed web server                                                 |
| JWT                       | `/golang-jwt/jwt`                | `golang-jwt/jwt/v5` 5.3.1 | Selected in M0; EdDSA and strict algorithm/claim parsing                                        |

All installed versions are exact in manifests and lockfiles. GitHub Actions are
pinned to immutable commit SHAs and annotated with their release line.
