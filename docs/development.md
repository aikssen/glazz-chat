# Development Foundation

## Prerequisites

Versions are pinned in `.tool-versions`, `package.json`, `go.work`, and the lockfiles:

- Node.js 24.13.0
- pnpm 11.17.0
- Go 1.26.5

Install JavaScript dependencies with `pnpm install --frozen-lockfile`. Go modules
are downloaded automatically by the Go commands.

## Environment

Copy the values you need from `.env.example` into the local `.env`; never commit
that file. The web application only receives variables prefixed with
`NEXT_PUBLIC_`. Provider credentials are server-only.

`LLM_PROVIDER_BASE_URL` and `LLM_PROVIDER_API_KEY` are the canonical provider
variables. `API_URL` and `API_KEY` remain temporary server-side aliases for the
development configuration supplied before M1. Explicit variables take precedence.
OpenCode Go is a development adapter, not the production provider.

## Root Commands

| Command                   | Purpose                                                                               |
| ------------------------- | ------------------------------------------------------------------------------------- |
| `pnpm dev`                | Start the Next.js application                                                         |
| `pnpm contracts:lint`     | Lint HTTP/realtime specs and validate WebSocket fixtures                              |
| `pnpm contracts:generate` | Generate TypeScript and Go HTTP contract code                                         |
| `pnpm check`              | Run the complete fast presubmit suite                                                 |
| `pnpm test:integration`   | Run Go tests marked for local-infrastructure integration                              |
| `pnpm e2e`                | Run browser tests; install Chromium with `pnpm exec playwright install chromium` once |
| `pnpm format`             | Format web and Go sources                                                             |

The API and worker can be started with `go run ./cmd/api` and
`go run ./cmd/worker` from `apps/api`.

The complete M2 stack starts from the repository root with:

```bash
docker compose -f deploy/compose.yaml up --build
```

Use `pnpm db:migrate` for forward migrations, `pnpm db:reset` for an intentional
non-production schema rebuild, `pnpm db:generate` after changing SQL queries, and
`pnpm test:integration` while Postgres and Redis are available. The integration
suite requires explicit `DATABASE_URL` and `REDIS_URL` values and fails closed when
Redis protection is unavailable.

Generated files under `packages/contracts/generated` and
`apps/api/internal/platform/api/generated.gen.go` are committed. CI regenerates
them and rejects drift.
