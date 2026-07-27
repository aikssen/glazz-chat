# Remote Development Runbook

## Purpose

Use this runbook when the source code must remain visible on a workstation while
CPU-, memory-, disk-, or container-intensive work runs on a server reached over
SSH.

The model has two workspaces:

- **Authoritative workspace:** the contributor's local Git checkout. Source
  editing, review, commits, tags, and pushes happen here.
- **Execution workspace:** a disposable remote clone used for dependency
  installation, generation, builds, tests, databases, and development services.

This separation prevents split-brain Git history while keeping heavy workloads
off the workstation. Installing the coding agent on the server is not required.

## Non-negotiable rules

1. Treat the local checkout as the only source of truth.
2. Never commit, tag, push, rebase, or merge from the execution workspace.
3. Never synchronize `.git`, credentials, `.env`, dependency directories, build
   output, coverage, or test artifacts.
4. Preview every source synchronization before applying it.
5. Do not use `StrictHostKeyChecking=no`.
6. Do not connect as `root` when the server provides an unprivileged user with
   Docker access.
7. Do not run destructive Git or filesystem recovery commands without explicit
   owner approval. Recreate a disposable execution workspace instead.
8. Keep databases and internal services off the LAN unless temporary direct
   access is explicitly required.
9. Copy generated source back only through explicit, narrow paths and inspect the
   local diff before accepting it.
10. Run release Git operations only after the authoritative local checkout and CI
    are green.

## Tested Glazz environment

Validated on 2026-07-23:

| Setting             | Value                                                       |
| ------------------- | ----------------------------------------------------------- |
| SSH target          | `deploy@dev-server.local`                                   |
| Hostname            | `dev-server`                                                |
| Remote project root | `/srv/glazz`                                                |
| Remote checkout     | `/srv/glazz/glazz-chat`                                     |
| Operating system    | Ubuntu Linux, x86-64                                        |
| Capacity            | 8 CPU, 15 GiB RAM, 53 GiB free disk at validation time      |
| Tool manager        | mise 2026.7.12                                              |
| Project toolchain   | Go 1.26.5, Node.js 24.13.0, pnpm 11.17.0                    |
| Containers          | Docker 29.6.2, Docker Compose 5.3.1                         |
| Validated revision  | `v0.2.0`, commit `8a8b3fd7158c9f5648a1292b77bc8d0344f57ca6` |

The server rejects direct `root` login. Use the `deploy` account.

## Variables for another project

Define these values before applying this runbook elsewhere:

```bash
REMOTE_HOST=dev-server.local
REMOTE_USER=deploy
REMOTE_PROJECT_ROOT=/srv/glazz
REPOSITORY_NAME=glazz-chat
REPOSITORY_SSH_URL=git@github.com:aikssen/glazz-chat.git
REMOTE_CHECKOUT="$REMOTE_PROJECT_ROOT/$REPOSITORY_NAME"
```

Commands below use the tested Glazz values explicitly so they can be audited
before execution. Replace all values consistently for another repository.

## One-time bootstrap

### 1. Validate SSH and server capacity

Run read-only checks:

```bash
ssh -o BatchMode=yes -o ConnectTimeout=8 \
  deploy@dev-server.local \
  'hostname; id; nproc; free -h; df -h /; docker version; docker compose version'
```

Expected:

- SSH exits without a password prompt.
- The effective user is not `root`.
- `docker version` shows both client and server.
- Available memory and disk are sufficient for the intended stack.

Stop if the host identity, user, or filesystem differs from the approved target.

### 2. Verify the Git host

GitHub publishes its SSH host fingerprints and `known_hosts` entries at:

<https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints>

At the validation date, the GitHub Ed25519 fingerprint was:

```text
SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU
```

Do not trust an unverified `ssh-keyscan` result. Compare the observed fingerprint
with GitHub's current official documentation before adding the host key.

Test repository access after host verification:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'git ls-remote git@github.com:aikssen/glazz-chat.git HEAD'
```

This must return a commit ID without an interactive prompt.

### 3. Clone without replacing existing data

First confirm the destination does not exist:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'test ! -e /srv/glazz/glazz-chat'
```

Then clone:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'git clone git@github.com:aikssen/glazz-chat.git /srv/glazz/glazz-chat'
```

If the destination already exists, inspect its owner, origin, branch, revision,
and status. Do not delete or overwrite it automatically.

### 4. Load the project toolchain through mise

Non-interactive SSH commands may not load the user's normal PATH. The server uses
Zsh and activates mise in a login shell, so run tool commands through `zsh -lic`:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && mise install"'
```

Verify the effective versions from inside the project:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && node --version && pnpm --version && go version"'
```

Use the project-pinned versions. Do not substitute globally installed versions
when mise reports missing project tools.

### 5. Install dependencies and establish a baseline

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && pnpm install --frozen-lockfile && pnpm check"'
```

The baseline must pass before synchronizing uncommitted local changes. This
distinguishes environment failures from changes introduced by the current task.

## Routine agent workflow

### 1. Preflight both workspaces

Local:

```bash
git status --short
git rev-parse HEAD
git branch --show-current
```

Remote:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && git status --short && git rev-parse HEAD && git branch --show-current'
```

Record existing local and remote changes. Never discard changes merely because
they were not created by the current agent.

### 2. Preview source synchronization

Run `rsync` from the repository root. The trailing slash on both paths is
significant.

```bash
rsync --archive --compress --checksum \
  --no-perms --no-times --omit-dir-times \
  --prune-empty-dirs --delete-delay --itemize-changes --dry-run \
  --exclude='.git/' \
  --exclude='.pnpm-store/' \
  --include='.env.example' \
  --include='.env.*.example' \
  --exclude='.env' \
  --exclude='.env.*' \
  --exclude='node_modules/' \
  --exclude='.next/' \
  --exclude='*.tsbuildinfo' \
  --exclude='coverage/' \
  --exclude='test-results/' \
  --exclude='playwright-report/' \
  --exclude='deploy/.data/' \
  ./ \
  deploy@dev-server.local:/srv/glazz/glazz-chat/
```

Review every proposed deletion and unexpected path. `--delete-delay` is useful
because local deletions must also disappear from the execution workspace, but it
makes the preview mandatory. Never add `--delete-excluded`. Keep include rules
before the broader exclude rules they override. Prefer a committed
repository-specific filter file when this process is automated.

`--checksum` prevents unrelated timestamp differences between macOS and Linux
from causing full-tree transfers. The metadata overrides keep rsync from changing
remote ownership-compatible permissions and timestamps. When adding a genuinely
executable source file, verify its executable bit separately in Git and on the
execution workspace.

### 3. Apply source synchronization

Run the same command without `--dry-run` only after the preview is correct:

```bash
rsync --archive --compress --checksum \
  --no-perms --no-times --omit-dir-times \
  --prune-empty-dirs --delete-delay --itemize-changes \
  --exclude='.git/' \
  --exclude='.pnpm-store/' \
  --include='.env.example' \
  --include='.env.*.example' \
  --exclude='.env' \
  --exclude='.env.*' \
  --exclude='node_modules/' \
  --exclude='.next/' \
  --exclude='*.tsbuildinfo' \
  --exclude='coverage/' \
  --exclude='test-results/' \
  --exclude='playwright-report/' \
  --exclude='deploy/.data/' \
  ./ \
  deploy@dev-server.local:/srv/glazz/glazz-chat/
```

Do not infer remote Git cleanliness after synchronization. The remote `.git`
directory remains at its cloned baseline while the worktree mirrors local source.

### 4. Execute checks remotely

Full presubmit:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && pnpm check"'
```

Integration tests require PostgreSQL and Redis plus explicit connection URLs:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat/apps/api && DATABASE_URL=postgres://glazz:glazz@localhost:5432/glazz?sslmode=disable REDIS_URL=redis://localhost:6379/0 go test -tags=integration ./..."'
```

Use a PTY for long-running commands when progress output is helpful. Always wait
for remote commands to exit and report their status; do not leave an unidentified
SSH process running.

### 4.1 Run isolated visual regression

Visual verification has a committed, disposable topology. Do not create temporary
Compose override files. After synchronizing source, run:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && mise exec -- pnpm e2e:visual'
```

The command uses `deploy/compose.e2e.yaml` and
`scripts/run-visual-e2e.sh`. It builds a production web image, starts fake-provider
API/worker services plus isolated PostgreSQL and Redis, executes the 20-image
Playwright matrix, and removes the `glazz-e2e` containers, network, and database
volume from an `EXIT` trap. Ports bind only to the remote loopback interface, so no
tunnel or LAN exposure is required.

For an approved visual change, update baselines remotely:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && mise exec -- pnpm e2e:visual:update'
```

Then copy back only the generated snapshots to the authoritative workstation:

```bash
rsync --archive --compress \
  deploy@dev-server.local:/srv/glazz/glazz-chat/apps/web/tests/e2e/visual.spec.ts-snapshots/ \
  apps/web/tests/e2e/visual.spec.ts-snapshots/

git diff --stat -- apps/web/tests/e2e/visual.spec.ts-snapshots/
```

Inspect representative images at original resolution, rerun `pnpm e2e:visual`
remotely without update mode, and verify cleanup:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'docker ps -a --filter label=com.docker.compose.project=glazz-e2e --format "{{.Names}}"; \
   docker volume ls --filter label=com.docker.compose.project=glazz-e2e --format "{{.Name}}"'
```

Both commands must print no resource names. The runner rejects the persistent
project name `glazz`; do not bypass that guard.

### 5. Run the application stack

The repository intentionally ignores `deploy/.env`, so Git and the regular source
sync do not update provider credentials or other local secrets. Before starting a
fresh remote clone, transfer the approved development environment separately and
preserve owner-only permissions:

```bash
rsync --archive --compress deploy/.env \
  deploy@dev-server.local:/srv/glazz/glazz-chat/deploy/.env
ssh -o BatchMode=yes deploy@dev-server.local \
  'chmod 600 /srv/glazz/glazz-chat/deploy/.env'
```

After changing provider configuration, recreate both `api` and `worker`; a restart
without container recreation does not reload `env_file`. Confirm the effective
non-secret selection with `docker inspect` before interpreting usage metrics.

The default command is suitable when the browser connects through an SSH tunnel:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && docker compose --env-file deploy/.env -f deploy/compose.yaml -f deploy/compose.dev.yaml up --build -d"'
```

For direct LAN access, browser-visible and OAuth URLs must use the server address.
`localhost` inside browser code refers to the contributor workstation, not the
execution server:

```bash
# Set these values in the separately managed remote deploy/.env:
WEB_URL=http://dev-server.local:3000
CORS_ALLOWED_ORIGINS=http://dev-server.local:3000,http://localhost:3000
GOOGLE_CALLBACK_URL=http://dev-server.local:8080/api/v1/auth/google/callback

ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && docker compose --env-file deploy/.env -f deploy/compose.yaml -f deploy/compose.dev.yaml up --build --force-recreate -d api worker web"'
```

Do not duplicate externally configurable values under a Compose service's
`environment` block when the same service uses `env_file`: `environment` has
higher precedence, including values produced by an interpolation default, and can
silently replace the remote `deploy/.env`. Keep Compose `environment` entries
limited to truly service-specific overrides.

When `NEXT_PUBLIC_API_URL` is empty, the web client derives the API hostname from
the current browser location and uses port `8080`. Set it explicitly only when the
public API uses another host. For OAuth testing without real Google credentials,
also provide `OAUTH_TEST_MODE=true`, a unique `OAUTH_TEST_EMAIL`, and the same email
in `BOOTSTRAP_ADMIN_EMAILS`. Never enable deterministic OAuth in production.

The direct-IP callback above is valid only for deterministic OAuth. Google OAuth
web clients reject raw non-loopback IP addresses and require HTTPS except for
`localhost`. Use one of these two development modes:

- Direct LAN access at `http://dev-server.local:3000`: set
  `OAUTH_TEST_MODE=true` and use the deterministic authorization screen.
- Real Google OAuth: tunnel both browser ports and use `localhost` consistently.

For real Google OAuth, set the remote environment to:

```dotenv
WEB_URL=http://localhost:3000
CORS_ALLOWED_ORIGINS=http://localhost:3000
GOOGLE_CALLBACK_URL=http://localhost:8080/api/v1/auth/google/callback
OAUTH_TEST_MODE=false
```

Register that exact callback in Google Cloud, recreate `api` and `worker`, then
keep this tunnel running on the Mac:

```bash
ssh -N -o ExitOnForwardFailure=yes \
  -L 3000:127.0.0.1:3000 \
  -L 8080:127.0.0.1:8080 \
  deploy@dev-server.local
```

Open `http://localhost:3000`. Do not mix a page opened through the LAN IP with a
`localhost` callback: cookies are host-scoped, so the resulting session would not
belong to the LAN origin.

Production TLS is terminated at the ingress/reverse proxy, which must also set
HSTS. Do not add CSP `upgrade-insecure-requests` to the Next.js application:
that directive upgrades the HTTP LAN page's own JavaScript, CSS, API calls, and
WebSocket to unavailable HTTPS endpoints. The production E2E security test
asserts that the directive stays absent.

Inspect:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && docker compose --env-file deploy/.env -f deploy/compose.yaml -f deploy/compose.dev.yaml ps'
```

Verify the LAN origin explicitly; health alone does not exercise browser CORS:

```bash
curl --fail-with-body --silent --show-error --dump-header - --output /dev/null \
  -H 'Origin: http://dev-server.local:3000' \
  http://dev-server.local:8080/api/v1/config/public
```

The response must contain
`Access-Control-Allow-Origin: http://dev-server.local:3000`. For development OAuth,
also inspect only the non-secret effective values inside the API container and run
the opt-in browser recovery test; a `200` health response does not prove that the
OAuth service was composed.

After every remote web deployment, run the browser recovery smoke from the
server. Unlike a browser opened against `localhost`, this uses the public LAN
origin and catches invalid CSP upgrades, CORS mistakes, disconnected WebSockets,
blocked modal controls, OAuth composition failures, and provider fallback:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && \
   E2E_BASE_URL=http://dev-server.local:3000 \
   E2E_PREVIEW_RECOVERY=true \
   E2E_PREVIEW_OAUTH=true \
   mise exec -- pnpm --filter @glazz/web exec playwright test \
     tests/e2e/preview-recovery.spec.ts \
     --project=desktop-1024 \
     --reporter=line'
```

The test performs one real model generation. Set `E2E_PREVIEW_OAUTH=false` only
when OAuth is intentionally disabled; do not treat that reduced smoke as OAuth
validation.

Readiness:

```bash
curl --fail-with-body --silent --show-error \
  http://dev-server.local:8080/api/v1/health/ready

curl --fail-with-body --silent --show-error --output /dev/null \
  --write-out '%{http_code}\n' \
  http://dev-server.local:3000
```

At the tested M2 revision:

- Web: `http://dev-server.local:3000`
- API: `http://dev-server.local:8080`
- Readiness: `http://dev-server.local:8080/api/v1/health/ready`

### 6. Inspect logs and stop services

Recent logs:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && docker compose --env-file deploy/.env -f deploy/compose.yaml -f deploy/compose.dev.yaml logs --tail=200'
```

Stop containers while preserving the PostgreSQL volume:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'cd /srv/glazz/glazz-chat && docker compose --env-file deploy/.env -f deploy/compose.yaml -f deploy/compose.dev.yaml down'
```

Do not add `--volumes` unless the owner explicitly requests destruction of remote
development data.

### 7. Release workstation memory after cutover

Only do this after remote checks and services are healthy. Inventory Docker
resources first and identify them by Compose project, image repository, labels, and
volume name. Remove only resources owned by the project being moved; a workstation
may contain unrelated databases and images.

For Docker Desktop on macOS:

```bash
docker compose --env-file deploy/.env \
  -f deploy/compose.yaml -f deploy/compose.dev.yaml down --remove-orphans
docker image ls
docker volume ls
docker desktop stop
docker desktop status
```

Delete project images and named volumes only with explicit owner approval. Never
use a global `docker system prune --all --volumes` as a routine cutover step.
Verify that project ports no longer listen locally and that the remote stack
remains healthy. Ignored dependency/build directories such as `node_modules`,
`.next`, coverage, and Playwright output may be removed locally because they are
reproducible; retain source, lockfiles, Git metadata, and `deploy/.env`.

## Generated files

Generation is heavy enough to run remotely, but generated source belongs in the
authoritative local checkout.

1. Synchronize local source to the remote workspace.
2. Run the generator remotely.
3. Inspect `git status --short` remotely to identify generated paths.
4. Copy back only the expected paths, never the entire remote checkout.
5. Inspect the local diff and rerun the appropriate validation remotely.

Glazz generated paths include:

```text
packages/contracts/generated/
apps/api/internal/platform/api/generated.gen.go
apps/api/internal/platform/store/
```

Example for a contract generation:

```bash
ssh -tt -o BatchMode=yes deploy@dev-server.local \
  'zsh -lic "cd /srv/glazz/glazz-chat && pnpm contracts:generate"'

rsync --archive --itemize-changes \
  deploy@dev-server.local:/srv/glazz/glazz-chat/packages/contracts/generated/ \
  packages/contracts/generated/

rsync --archive --itemize-changes \
  deploy@dev-server.local:/srv/glazz/glazz-chat/apps/api/internal/platform/api/generated.gen.go \
  apps/api/internal/platform/api/generated.gen.go
```

Do not copy all of `apps/api/internal/platform/store/` unless the generator's
expected output set has been confirmed. A narrow file list is safer when source
and generated files share a directory.

## Secrets and environment files

- Never include `.env` in routine synchronization.
- Never print secret values to logs or agent output.
- Provision remote secrets separately from an approved source.
- Set remote secret-file permissions to `0600`.
- Keep browser-exposed `NEXT_PUBLIC_*` values distinct from server credentials.
- Confirm the remote host is trusted before transferring any credential.
- Obtain explicit approval before copying an existing local secret file to the
  server.

Before starting API or worker containers for the first time, create
`$REMOTE_REPO/.env` from `.env.example`, generate a unique cookie signing key, and
set the remote host's own provider and authentication values. The Compose
definition requires this file. Do not reuse or synchronize the workstation
`.env`.

The Go loader accepts an absolute `GLAZZ_ENV_FILE` for CI or an explicitly managed
environment file. Without that override, it walks upward to the repository
`go.work` marker and loads the root `.env`. Existing process variables always take
precedence over file values.

For temporary commands, prefer an approved remote secret file or environment
manager over embedding secrets in SSH command history.

## Network exposure

The M2 Compose file publishes ports on `0.0.0.0`, which made the validation URLs
reachable directly over the trusted LAN. This is convenient but not the preferred
long-term posture.

For routine remote development:

1. Bind web, API, PostgreSQL, and Redis to `127.0.0.1` on the server through a
   Compose override.
2. Forward only web and API to the workstation:

```bash
ssh -N \
  -L 3000:127.0.0.1:3000 \
  -L 8080:127.0.0.1:8080 \
  deploy@dev-server.local
```

3. Access `http://localhost:3000` and `http://localhost:8080`.
4. Do not forward PostgreSQL or Redis unless a specific local tool requires them.

Production credentials or production data must never be used in this development
stack.

## Git lifecycle

The authoritative local workspace owns all Git history:

```text
edit locally
  -> preview sync
  -> sync to execution workspace
  -> run remote checks
  -> inspect local diff
  -> commit locally
  -> push locally
  -> wait for CI
  -> tag locally when milestone policy allows
```

The remote clone may become stale relative to its `.git` metadata after repeated
worktree synchronization. That does not affect execution. At a clean milestone,
prefer cloning a fresh execution workspace over force-resetting the existing one.

## Troubleshooting

### `go`, `node`, or `pnpm` is not found

The SSH command did not load mise. Execute through:

```bash
zsh -lic 'cd /srv/glazz/glazz-chat && <command>'
```

Then run `mise install` if the project reports missing pinned versions.

### `Host key verification failed`

Verify the current GitHub fingerprint against official GitHub documentation and
add the approved host key. Do not disable host checking.

### `Permission denied` from Docker

Run `id` and `docker version`. The approved user must have access to the Docker
socket. Do not switch to `root` as a shortcut.

### A deleted file still affects remote tests

The synchronization omitted deletion handling or was not run from the repository
root. Preview and apply the documented `rsync --delete-delay` command.

### Remote tests differ from CI

Check:

- Exact commit and local diff
- mise-selected versions inside the project
- Frozen lockfile installation
- Required environment variables
- Docker image versions
- Generated-file drift
- CPU architecture differences

### A port is already in use

Inspect the existing listener and Docker projects before changing ports:

```bash
ssh -o BatchMode=yes deploy@dev-server.local \
  'docker ps --format "table {{.Names}}\t{{.Ports}}"'
```

Do not stop unrelated services.

### Remote disk or memory is low

Collect evidence with `free -h`, `df -h`, `docker system df`, and Compose status.
Do not prune Docker globally without explicit approval because other projects may
share the server.

### Docker dependency downloads hang

Compare connectivity from the host and build network before changing source:

```bash
curl --fail --max-time 10 https://proxy.golang.org/
docker build --network=host -f deploy/docker/api.Dockerfile .
```

Treat `--network=host` as a Linux development-server diagnostic, not a portable CI
default. A transient DNS failure must not trigger dependency-version changes. Keep
the currently healthy containers running until a replacement image is built and
passes health checks.

## Agent acceptance checklist

An agent may report the remote workflow ready only when:

- [ ] SSH works non-interactively with the approved non-root account.
- [ ] The remote host identity was verified.
- [ ] The repository origin, branch, commit, and existing status were inspected.
- [ ] mise selected the project-pinned toolchain.
- [ ] Dependency installation used the frozen lockfile.
- [ ] The clean baseline check passed remotely.
- [ ] Source synchronization was previewed before application.
- [ ] No secrets or local Git metadata were synchronized.
- [ ] Required Compose services are running and healthy.
- [ ] API readiness succeeds from the intended access path.
- [ ] The frontend returns a successful HTTP response.
- [ ] The authoritative local checkout retains all source changes.
- [ ] No remote command or SSH session required by the task remains running
      unintentionally.

## Validated result

The initial Glazz trial completed successfully:

- The full `pnpm check` suite passed remotely.
- Go race tests and production builds passed.
- Next.js production build passed.
- PostgreSQL, Redis, API, worker, and web started through Docker Compose.
- PostgreSQL, Redis, API, and web reported healthy.
- API readiness reported PostgreSQL and Redis as `up`.
- The web endpoint returned HTTP 200 from the workstation.
- The running stack used approximately 1 GiB of server RAM at observation time.
- Both local and remote Git worktrees remained clean after validation.
