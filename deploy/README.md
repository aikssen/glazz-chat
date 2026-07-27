# Deployment and local orchestration

The deployment files separate topology from environment-specific policy:

| File | Purpose |
| --- | --- |
| `compose.yaml` | Complete base stack: services, builds, ports, and healthchecks |
| `compose.dokploy.yaml` | Base stack plus Dokploy resource limits |
| `compose.e2e.yaml` | Isolated visual-test override |

Container ports are stable implementation details: PostgreSQL `5432`, Redis
`6379`, API `8080`, and web `3000`. Only published host ports are configurable,
using the straightforward `${HOST_PORT}:container_port` mapping.

## Environment contract

Compose uses the same environment file in two distinct ways:

- `docker compose --env-file ...` supplies values used while resolving Compose;
- `env_file` supplies runtime configuration to API and worker containers.

Keep `GLAZZ_COMPOSE_ENV_FILE` aligned with the file passed to `--env-file`. The
path is relative to this `deploy` directory.

The committed templates are:

- `.env.dev.example`: complete local-development values;
- `.env.stage.example`: normal staging values with blank secrets;
- `.env.prod.example`: normal production values with blank secrets.

Normal values may be stored in the deployment environment file. Supply these
secrets through Dokploy or the production secret manager:

- `POSTGRES_PASSWORD`;
- `DATABASE_URL`;
- `REDIS_URL` when Redis authentication is enabled;
- `GOOGLE_CLIENT_SECRET`;
- `COOKIE_SIGNING_KEY`;
- `LLM_PROVIDER_API_KEY`;
- the JWT private key file.

`JWT_PRIVATE_KEY_PATH` is not itself secret; the file contents are. Do not commit
a populated environment file or private key.

## Development stack

Create the untracked deployment environment:

```bash
cp deploy/.env.dev.example deploy/.env
```

Then run the base stack:

```bash
docker compose \
  --env-file deploy/.env \
  -f deploy/compose.yaml \
  up --build
```

Stop it with the same configuration:

```bash
docker compose \
  --env-file deploy/.env \
  -f deploy/compose.yaml \
  down
```

Development data remains in named volumes. Add `--volumes` only for an
intentional data reset.

## Dokploy staging and production

Use `compose.dokploy.yaml` when Dokploy should enforce the configured memory
limits. It extends the complete base stack instead of redefining it.

In Dokploy:

1. Select `./deploy/compose.dokploy.yaml` as the Compose path.
2. Copy the appropriate normal values from `.env.stage.example` or
   `.env.prod.example` into the Compose environment editor.
3. Add secret values in Dokploy, preferably through project-level shared
   variables when several services need the same secret.
4. Mount the JWT private key as a file at `JWT_PRIVATE_KEY_PATH` for both API and
   worker.
5. Route the web domain to container port `3000` and the API domain to `8080`.

Dokploy writes its Compose environment beside the selected Compose file, so
`GLAZZ_COMPOSE_ENV_FILE=.env` lets API and worker consume it. PostgreSQL receives
only its explicitly mapped database variables, and web receives only its public
API URL.

## Dokploy LAN rehearsal

The LAN rehearsal also uses `compose.dokploy.yaml`; the environment determines
its public addresses and published ports. Use `.env.dev.example` as the complete
key list, add the resource-limit variables from a stage template, and configure:

```dotenv
COMPOSE_PROJECT_NAME=glazz-dokploy-demo
GLAZZ_COMPOSE_ENV_FILE=.env
POSTGRES_HOST_PORT=5432
REDIS_HOST_PORT=6379
API_HOST_PORT=18080
WEB_HOST_PORT=13000
WEB_URL=http://192.168.68.211:13000
NEXT_PUBLIC_API_URL=http://192.168.68.211:18080
CORS_ALLOWED_ORIGINS=http://192.168.68.211:13000
GOOGLE_CALLBACK_URL=http://192.168.68.211:18080/api/v1/auth/google/callback
JWT_ISSUER=http://192.168.68.211:18080
```

PostgreSQL and Redis bind only to host loopback. API and web use their configured
host ports for LAN access. When either application port changes, update its
public origin variables to match.

Validate the rehearsal:

```bash
curl --fail http://192.168.68.211:18080/api/v1/health/live
curl --fail http://192.168.68.211:18080/api/v1/health/ready
curl --fail http://192.168.68.211:13000/
```

## Isolated visual E2E stack

The visual runner reads its project name, published ports, and origins from
`.env.test.example`. It uses the fake provider, deterministic OAuth, a
separate Compose project, loopback-only published ports, and disposable volumes.

Run the reviewed visual gate:

```bash
pnpm e2e:visual
```

Regenerate visual baselines only after an intentional design change:

```bash
pnpm e2e:visual:update
pnpm e2e:visual
```

To run parallel jobs, copy `.env.test.example`, select distinct port values, and
pass that file consistently to the runner/Compose configuration. Never point the
E2E origins at shared, production, or persistent development services.

## Inspect the resolved configuration

Before deploying an environment, verify the rendered model without starting it:

```bash
docker compose \
  --env-file deploy/.env \
  -f deploy/compose.yaml \
  config
```

Remember that rendered output can contain secrets. Do not paste or commit it.
