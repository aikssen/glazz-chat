# Contributing to Glazz

Thank you for considering a contribution. Glazz is a portfolio-grade reference
project with deliberate constraints, so please read this before opening a pull
request.

## Before you start

Open an issue first for anything beyond an obvious fix. Glazz follows a documented
delivery plan, and a change that conflicts with the current milestone or with an
accepted architecture decision is likely to be declined regardless of code quality.

- Product scope and business rules: [PROJECT.md](./PROJECT.md)
- Canonical architecture: [ARCHITECTURE.md](./ARCHITECTURE.md)
- Delivery plan and status: [TASKS.md](./TASKS.md)
- Architecture decisions: [docs/adr/](./docs/adr/)
- Contributor and coding-agent rules: [AGENTS.md](./AGENTS.md)

`AGENTS.md` is normative. It applies to human contributors and to coding agents
alike.

## Environment

Exact versions live in [`.tool-versions`](./.tool-versions), the manifests, and the
lockfiles. Install them with [mise](https://mise.jdx.dev/) or match them manually.
Do not upgrade a pinned dependency as a side effect of an unrelated change.

The Docker Compose path in the [README](./README.md#run-the-complete-stack) needs
only Docker and Git. Contributors running checks outside containers need Node, pnpm,
Go, PostgreSQL, and Redis.

```bash
cp .env.example .env          # then set COOKIE_SIGNING_KEY, see the README
pnpm install --frozen-lockfile
docker compose -f deploy/compose.yaml up -d postgres redis
pnpm db:migrate
```

Start with `LLM_PROVIDER_KIND=fake`. It is deterministic and calls no external
model, and it is what the automated tests expect.

## Checks

Run the fast suite before every push:

```bash
pnpm check
```

Run the layer that matches your change:

| Change                        | Command                                           |
| ----------------------------- | ------------------------------------------------- |
| Anything                      | `pnpm check`                                      |
| Repositories, workers, quotas | `pnpm test:integration`                           |
| Browser behavior              | `pnpm e2e`                                        |
| Visual surface                | `pnpm e2e:visual`                                 |
| OpenAPI or AsyncAPI           | `pnpm contracts:lint` + `pnpm contracts:generate` |
| SQL queries                   | `pnpm db:generate`                                |
| Formatting                    | `pnpm format`                                     |

CI runs `presubmit`, `e2e`, and `security`. All three must pass.

## Contracts and migrations

The Go backend and the TypeScript frontend meet at a generated boundary. Generated
artifacts are committed, and CI fails when they drift from their source.

- Change [`packages/contracts/openapi.yaml`](./packages/contracts/openapi.yaml) or
  [`websocket.asyncapi.yaml`](./packages/contracts/websocket.asyncapi.yaml) first,
  then regenerate. Never hand-edit generated files.
- Database changes are forward-only goose migrations. Do not edit an applied
  migration; add a new one.
- A contract change that breaks an existing client needs an issue and an explicit
  decision before implementation.

## Tests

New behavior needs evidence proportional to its risk, in the spirit of the
[test strategy](./README.md#test-strategy). Concurrency, shutdown, ownership, and
quota changes need a test that actually exercises the dangerous path — a test that
passes without driving the risky code adds coverage, not confidence.

Run `go test -race` for anything touching goroutines, channels, or shared state.

## Secrets and personal data

- Never commit `.env`, credentials, tokens, or private keys. Secret scanning and
  push protection are enabled.
- Never add real hostnames, LAN addresses, usernames, or absolute paths from your
  own machine to documentation. Use placeholders such as `deploy`,
  `dev-server.local`, and `/srv/glazz`.
- Never log, trace, or store prompt or response content. This is an architectural
  rule, not a preference.

## Pull requests

- Branch from `main` using a short prefixed name, for example
  `feat/model-catalog-filter` or `fix/ws-replay-window`.
- Commit messages follow Conventional Commits: `feat:`, `fix:`, `docs:`, `test:`,
  `refactor:`, `chore:`, `ci:`.
- Keep one logical change per pull request.
- Update the documentation affected by your change in the same pull request.
- Fill in the pull request template and confirm which checks you ran.
- Expect review comments about architecture boundaries and evidence, not only style.

## What is unlikely to be merged

- A new runtime dependency without a justification against the
  [dependency policy](./docs/dependency-policy.md).
- Prompt or response content added to logs, metrics, traces, or audit records.
- Provider-specific behavior leaking out of the provider gateway.
- Direct browser access to provider credentials.
- Features that expand scope ahead of the current milestone in `TASKS.md`.

## Code of conduct

Participation is governed by the [Code of Conduct](./CODE_OF_CONDUCT.md).

## License

Contributions are accepted under the [Apache License 2.0](./LICENSE). The Glazz
name and visual identity are excluded; see [TRADEMARKS.md](./TRADEMARKS.md).
