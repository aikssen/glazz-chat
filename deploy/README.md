# Deployment and local orchestration

The deployment files separate topology from environment-specific policy:

| File                     | Purpose                                                               |
| ------------------------ | --------------------------------------------------------------------- |
| `compose.yaml`           | Complete base stack: services, builds, ports, and healthchecks        |
| `compose.dokploy.yaml`   | Base stack plus Dokploy resource limits                               |
| `compose.public.yaml`    | Self-contained stack for public exposure behind a single-origin proxy |
| `compose.e2e.yaml`       | Isolated visual-test override                                         |
| `docker/proxy.Caddyfile` | Caddy configuration for the public stack's reverse proxy              |

Container ports are stable implementation details: PostgreSQL `5432`, Redis
`6379`, API `8080`, and web `3000`. Only published host ports are configurable,
using the straightforward `${HOST_PORT}:container_port` mapping.

## Environment contract

`LOG_LEVEL` is a normal, non-secret setting shared by the API and worker and
compiled into the web client as `NEXT_PUBLIC_LOG_LEVEL`. Accepted values are
`debug`, `info`, `warn`, and `error`; use `debug` for local development and
`info` or `warn` for a small production demo. Logs never include prompts,
responses, credentials, cookies, or provider keys. HTTP requests use
`X-Correlation-ID`, while realtime commands and worker jobs use their request
or event ID as `correlation_id`.

Compose uses the same environment file in two distinct ways:

- `docker compose --env-file ...` supplies values used while resolving Compose;
- `env_file` supplies runtime configuration to API and worker containers.

Keep `GLAZZ_COMPOSE_ENV_FILE` aligned with the file passed to `--env-file`. The
path is relative to this `deploy` directory.

`deploy/.env.example` is the single committed Compose/Dokploy template. Copy it
and replace the environment-specific values; do not create parallel committed
templates for each environment.

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
cp deploy/.env.example deploy/.env
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
2. Copy the normal values from `.env.example` into the Compose environment
   editor and adapt them for the target environment.
3. Add secret values in Dokploy, preferably through project-level shared
   variables when several services need the same secret.
4. Mount the JWT private key as a file at `JWT_PRIVATE_KEY_PATH` for both API and
   worker.
5. Route the web domain to container port `3000` and the API domain to `8080`.

Dokploy writes its Compose environment beside the selected Compose file, so
`GLAZZ_COMPOSE_ENV_FILE=.env` lets API and worker consume it. PostgreSQL receives
only its explicitly mapped database variables, and web receives only its public
API URL.

## Public deployment behind a single-origin proxy

`compose.public.yaml` is the stack meant to face the Internet through a reverse proxy
such as Cloudflare Tunnel or Traefik. It differs from the Dokploy stack in three ways,
all driven by exposure:

1. a **Caddy `proxy` service** is the single entry point, so the browser sees one
   origin;
2. nothing else publishes a host port — PostgreSQL, Redis, the API, and web are
   reachable only through the proxy on the internal network;
3. the proxy joins an external reverse-proxy network, named by `PROXY_NETWORK` (for
   example `dokploy-network`), so the front proxy can route to it.

It is self-contained rather than an override, because a platform like Dokploy points a
Compose application at a single Compose path — a `-f base -f override` chain is not
expressible there.

### Why one origin

The browser reaches both the Next.js app and the Go API, and the chat rides a
WebSocket at `/api/v1/ws`. Serving them from one hostname removes CORS entirely, keeps
cookies same-site (so `SameSite=lax` and the existing CSRF design hold unchanged), and
lets `NEXT_PUBLIC_API_URL` be just the public origin. Splitting the API onto a second
hostname would force `SameSite=none` cookies and a wider CSRF surface.

### What Caddy is, and how it is used here

[Caddy](https://caddyserver.com) is a small web server and reverse proxy written in Go.
Its configuration — the [Caddyfile](./docker/proxy.Caddyfile) — is deliberately terse:
a handful of directives replace the dozen `proxy_set_header` lines an equivalent Nginx
config would need, it sets the `X-Forwarded-*` headers by default, and compression is a
single word.

In this stack Caddy does one job: split traffic by path on a single origin.

```caddyfile
:80 {
	encode zstd gzip
	@api path /api/*
	handle @api {
		reverse_proxy api:8080     # REST and the /api/v1/ws WebSocket
	}
	handle {
		reverse_proxy web:3000     # the Next.js application
	}
}
```

Two details matter and are commented in the file:

- **`reverse_proxy` upgrades the WebSocket transparently.** The `/api/v1/ws` handshake
  needs no special handling — Caddy detects the `Upgrade` header and proxies the
  connection, so realtime survives the hop.
- **The site address is `:80`, not a hostname.** Given a hostname, Caddy would try to
  obtain a certificate over ACME. That is wrong here: TLS terminates at the edge proxy,
  and the hop from it to Caddy is plain HTTP inside the host. Writing `:80` disables
  automatic HTTPS by design.

Caddy runs as the `proxy` service from the stock `caddy:2-alpine` image, mounting
`docker/proxy.Caddyfile` read-only. It is the only service the front proxy routes to;
everything else is internal.

### Production mode is stricter

Glazz refuses to start with `GLAZZ_ENV=production` unless four things hold, by design in
`internal/platform/config`:

- Google OAuth is configured (production assumes registered users);
- `JWT_PRIVATE_KEY_PATH` points at a **persistent** ed25519 PKCS#8 key — an ephemeral
  key would drop every session on restart;
- `COOKIE_SIGNING_KEY` decodes to at least 32 bytes;
- `COOKIE_SECURE=true`.

The JWT key is mounted read-only from a host path kept outside the repository. Its file
must be readable by the container user (`uid 100`, the `glazz` account in the image), so
`chown 100:101` it and keep mode `600` rather than making it world-readable. Guests can
chat without signing in; OAuth only has to be _configured_ for the service to start, not
_used_ by every visitor.

### Provider safety

Publish this stack with `LLM_PROVIDER_KIND=fake` unless the hostname is gated. The fake
provider is deterministic and costs nothing, and it still exercises the streaming,
cancellation, quota, and reconnect behaviour worth showing. An approved metered provider
reachable from the open Internet is a bill, not a demo — the four-prompt guest budget
exists precisely because unbounded guest access to a metered vendor is an invitation.

Reaching an approved provider on production infrastructure is **Phase 10 (M6)**, which is
separate: a tunnelled demo does not satisfy it, and the repository's "not approved for
public production traffic" statement still holds.

## Dokploy LAN rehearsal

The LAN rehearsal also uses `compose.dokploy.yaml`; the environment determines
its public addresses and published ports. Use `.env.example` as the complete
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
