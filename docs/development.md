# Development Foundation

## Prerequisites

Versions are pinned in `.tool-versions`, `package.json`, `go.work`, and the lockfiles:

- Node.js 24.13.0
- pnpm 11.17.0
- Go 1.26.5

Install JavaScript dependencies with `pnpm install --frozen-lockfile`. Go modules
are downloaded automatically by the Go commands.

## Environment

Copy the complete `.env.example` to the local `.env`, replace every
environment-specific value, and never commit that file. Generate a unique
`COOKIE_SIGNING_KEY` with the command documented in the example. The Go
configuration loader reads `.env` from the repository root when commands run from
the root or `apps/api`; variables already injected by the shell, CI, or the
orchestrator take precedence.

Every declared key is required, including keys whose valid value is empty. Runtime
configuration has no development URL, credential, policy, legal-version, model, or
service-name defaults embedded in Go. Tests use `.env.test.example`; production
must inject its own values and must not use either example file.

The web application only receives variables prefixed with `NEXT_PUBLIC_`. Provider
credentials are server-only.

`LLM_PROVIDER_BASE_URL` and `LLM_PROVIDER_API_KEY` are the canonical provider
variables. `API_URL` and `API_KEY` remain temporary server-side aliases for the
development configuration supplied before M1. Explicit variables take precedence.
OpenCode Go is a development adapter, not the production provider.

Compose requires the untracked `deploy/.env` for interpolation and for API and
worker runtime configuration. Create it from `deploy/.env.example`. A remote
execution host must provision its own file; do not copy workstation secrets to it
as part of source synchronization.

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

The complete development stack starts from the repository root with:

```bash
docker compose --env-file deploy/.env \
  -f deploy/compose.yaml up --build
```

Use `pnpm db:migrate` for forward migrations, `pnpm db:reset` for an intentional
non-production schema rebuild, `pnpm db:generate` after changing SQL queries, and
`pnpm test:integration` while Postgres and Redis are available. The integration
suite requires explicit `DATABASE_URL` and `REDIS_URL` values and fails closed when
Redis protection is unavailable.

Generated files under `packages/contracts/generated` and
`apps/api/internal/platform/api/generated.gen.go` are committed. CI regenerates
them and rejects drift.

## Remote execution

Use [Remote Development Runbook](./runbooks/remote-development.md) when source
remains on a contributor workstation but dependencies, generation, checks, Docker,
and development services run on the shared development server. The local checkout
remains authoritative for Git history; the remote clone is a disposable execution
workspace.
