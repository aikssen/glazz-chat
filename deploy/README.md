# Deployment and local orchestration

## Development stack

`compose.yaml` runs the persistent development topology: web, API, worker,
PostgreSQL, and Redis. It requires the separately managed root `.env`.

```bash
docker compose -f deploy/compose.yaml up --build
docker compose -f deploy/compose.yaml down
```

Development data lives in the `glazz_postgres-data` volume. Do not add
`--volumes` unless that data is intentionally being reset.

PostgreSQL and Redis use exact image versions. Local database backups and Redis
persistence are intentionally disabled; production persistence is designed and
verified in M6.

## Isolated visual E2E stack

`compose.e2e.yaml` is a committed override for deterministic visual regression.
It never reads the development `.env`; API and worker load the safe
`.env.test.example` template and force the fake provider.

The E2E topology:

- uses the separate Compose project `glazz-e2e`;
- binds PostgreSQL, Redis, API, and web only to `127.0.0.1`;
- publishes ports `15432`, `16379`, `18080`, and `13000` by default;
- uses a disposable PostgreSQL volume;
- enables deterministic OAuth with a test-only administrator;
- cannot consume the configured development LLM provider;
- is always removed with its volume after the runner exits.

Run the reviewed visual gate:

```bash
pnpm e2e:visual
```

Regenerate every visual baseline only after an intentional design change:

```bash
pnpm e2e:visual:update
pnpm e2e:visual
```

Extra Playwright arguments follow `--`:

```bash
pnpm e2e:visual -- --project=wide-1440
```

The runner rejects `GLAZZ_E2E_PROJECT=glazz` so cleanup cannot target the
persistent development stack. It prints API, worker, and web logs on failure, then
uses `down --volumes --remove-orphans` from an `EXIT` trap.

Ports and the isolated project name can be changed when parallel jobs require it:

```bash
GLAZZ_E2E_PROJECT=glazz-e2e-2 \
GLAZZ_E2E_WEB_PORT=23000 \
GLAZZ_E2E_API_PORT=28080 \
GLAZZ_E2E_POSTGRES_PORT=25432 \
GLAZZ_E2E_REDIS_PORT=26379 \
pnpm e2e:visual
```

Do not point `GLAZZ_E2E_WEB_ORIGIN` or `GLAZZ_E2E_API_ORIGIN` at shared,
production, or persistent development services.
