# AGENTS.md

## Purpose and precedence

This file defines mandatory working rules for human contributors and coding agents
in the Glazz repository.

Instruction precedence:

1. Explicit task instructions from the repository owner
2. This `AGENTS.md`
3. Feature-local `AGENTS.md` files, if added later
4. Established patterns in adjacent code

When instructions conflict, stop and document the conflict instead of silently
choosing. Do not weaken security, privacy, API compatibility, or data integrity to
finish a task faster.

## Read before changing code

Read the relevant sources in this order:

1. [PROJECT.md](./PROJECT.md) for product scope and business rules
2. [ARCHITECTURE.md](./ARCHITECTURE.md) for system boundaries and contracts
3. [DESIGN.md](./DESIGN.md) for frontend behavior and visual rules
4. [TASKS.md](./TASKS.md) for sequencing and acceptance criteria
5. The OpenAPI/WebSocket contracts and ADRs relevant to the slice
6. Existing tests and neighboring implementation

Do not implement a later phase while a required contract or dependency task remains
open unless the owner explicitly approves it.

## Current documentation and dependency versions

Context7 is available to this repository through its MCP integration and the local
`context7-cli` skill registered in `skills-lock.json`. Use Context7 before selecting
or changing any language, framework, library, tool, or public API:

1. Resolve the canonical library ID.
2. Query the specific API or compatibility question.
3. Confirm the supported stable version from Context7 and the upstream official
   release source.
4. Record the decision date, selected version range, and source in the relevant ADR
   or dependency manifest.
5. Pin the exact version in code and CI; never use `latest` in committed production
   configuration.

Do not send secrets, private code, credentials, personal data, or proprietary
payloads to Context7. If Context7 is unavailable, use only primary official
documentation, record the fallback, and re-verify before merging. Search-engine
snippets and model memory are not sufficient sources for dependency decisions.

## Milestone versioning

Starting with M2, every completed milestone increments the semantic-version minor
component and receives an annotated Git tag: M2 is `v0.2.0`, M3 is `v0.3.0`, and
so on. Do not create or move a milestone tag until its acceptance checks and CI
pass. The repository owner may explicitly replace this rule for a future release.

## Product invariants

These rules are not implementation suggestions:

- A guest receives at most four user prompts and 2,000 output tokens in one
  short-lived conversation.
- Guest limits do not reset automatically.
- Guest data that is not migrated is removed by the daily cleanup job.
- Guest conversation migration during Google login is transactional and idempotent.
- Registered users start with 50 messages/day, 50,000 output tokens/day, and one
  concurrent generation.
- Administrators can change limits at runtime, with audit records.
- The browser never receives an LLM provider credential.
- OpenCode Go is a development provider, not a production dependency.
- Provider-specific types and errors do not escape the provider adapter.
- The MVP supports OpenAI-compatible Chat Completions only.
- The MVP does not include files, images, web search, tools, agents, public sharing,
  export, message editing, or conversation branching.
- Users may retry only the latest failed or cancelled generation.
- Account deletion revokes sessions immediately and completes data deletion within
  24 hours.
- Prompts and responses are never written to operational logs.

Changing an invariant requires updating the planning documents, contract, tests,
and an ADR where architectural behavior changes.

## API-first workflow

For any API-visible behavior:

1. Update `packages/contracts/openapi.yaml` or
   `packages/contracts/websocket.asyncapi.yaml`.
2. Add examples for success, validation, authorization, quota, and failure cases.
3. Validate the specifications.
4. Regenerate TypeScript/Go contract artifacts.
5. Review the generated diff; never edit generated files manually.
6. Add or update contract tests.
7. Implement the backend behavior.
8. Implement the frontend consumer.
9. Add integration and E2E coverage.
10. Update architecture/task documentation if assumptions changed.

The public API uses `/api/v1`. Additive compatible fields may enter v1. Breaking
changes require an explicit versioning decision.

## Repository boundaries

Expected top-level ownership:

- `apps/api`: Go API and worker
- `apps/web`: Next.js application
- `packages/contracts`: API sources, generated types, fixtures
- `deploy`: Docker and deployment assets
- `docs/adr`: architecture decisions
- `docs/runbooks`: operational procedures
- `docs/threat-model`: security analysis
- `scripts`: repository automation

Do not create a generic `utils`, `helpers`, `common`, or `shared` package as a
dumping ground. A shared abstraction must have a clear owner and at least two real
call sites.

## Vertical slicing

### Go

Organize backend code by business capability:

```text
internal/<slice>/
|-- domain.go
|-- ports.go
|-- service.go
|-- transport_http.go
|-- transport_ws.go       # only where needed
|-- repository_postgres.go
|-- service_test.go
`-- repository_test.go
```

This is a guideline, not a requirement to create empty files. Keep a slice compact
until separation improves clarity.

Allowed dependency direction:

```text
transport -> application service -> domain/ports
                                  <- adapters
```

- Domain/application code does not import HTTP, WebSocket, PostgreSQL, Redis, or
  provider SDK packages.
- Slices collaborate through narrow application interfaces or domain events.
- A slice does not query another slice's tables directly without an explicitly
  documented read model or transaction boundary.

### Next.js

Organize client behavior by feature:

```text
src/features/<feature>/
|-- components/
|-- api/
|-- model/
|-- hooks/
|-- messages/
`-- tests/
```

- Route files compose features and remain thin.
- Server Components are the default.
- Add `"use client"` only at the smallest interactive boundary.
- State local to a feature stays in that feature.
- Remote server state is not copied into a global client store without a concrete
  offline or cross-route requirement.

## Dependency injection

Go constructors receive every operational dependency explicitly. Inject:

- Repositories and transaction runner
- Clock and ID generator
- Token signer/verifier and random source
- Google identity client
- Chat provider
- Input/output safety policies
- Rate limiter and concurrency lease manager
- Telemetry interfaces where direct OpenTelemetry use would couple domain code

Avoid service locators, mutable package globals, hidden `init()` registration, and
environment reads below the composition root.

Frontend side effects are similarly isolated behind typed adapters for HTTP,
WebSocket, storage, clock, and browser APIs when deterministic tests require them.

## Go engineering rules

- Use the Go version pinned in `apps/api/go.mod`; CI uses the latest security patch
  for that supported minor release.
- Run `gofmt`, `go vet`, tests, and the race detector on affected packages.
- Wrap errors with operation context and preserve causes.
- Map errors to stable public codes at the transport boundary.
- Accept `context.Context` as the first parameter for I/O operations.
- Honor cancellation and set explicit external-call timeouts.
- Close response bodies and streams deterministically.
- Prefer small interfaces defined by the consumer.
- Use typed state values and exhaustive transition tests.
- Never log an entire request, provider payload, cookie, token, prompt, or response.
- Use `pgx` and generated `sqlc` queries for database access.
- Use transactions for ownership migration, token rotation, quota reservations, and
  generation state changes.
- Use parameterized SQL only.

## Database and migration rules

- SQL migrations are append-only once merged.
- Never edit a migration that could have run in another environment.
- Every migration has a clear forward action and an operational rollback/mitigation
  note, even when automatic down migration would risk data loss.
- Use expand/migrate/contract for production schema changes.
- Add database constraints for ownership, uniqueness, allowed states, and referential
  integrity.
- Indexes must be justified by a query in `sqlc` or an operational need.
- Test migrations from an empty database and from the previous release schema.
- Do not use Redis as a source of truth or job-completion ledger.

## TypeScript and Next.js rules

- TypeScript strict mode is mandatory.
- Avoid `any`; use `unknown` and validate at trust boundaries.
- Use generated OpenAPI types rather than duplicating DTOs.
- Validate runtime inputs/events even when static types exist.
- Keep server-only modules explicitly server-only.
- Do not expose secrets through `NEXT_PUBLIC_*`.
- Avoid unnecessary client components and effect-driven data fetching.
- Reserve dimensions for dynamic controls and loading states to prevent layout
  shifts.
- Sanitize model Markdown and disable raw HTML.
- Handle stale/duplicate WebSocket events by event ID, sequence, and delta offset.
- Preserve an unacknowledged draft when a send fails.
- All user-visible strings use typed localization keys.

## Frontend design rules

Follow [DESIGN.md](./DESIGN.md) exactly.

Mandatory:

- Chat is the first usable screen.
- Use semantic design tokens, not raw component-level colors.
- Use Outfit for restrained display roles, Work Sans for UI/body, and JetBrains
  Mono for code/data.
- Use Lucide icons and accessible labels.
- Keep cards at 8px radius or less and never nest cards.
- Keep message content mostly unframed.
- Use the orange live signal rail only for active generation; teal means complete.
- Maintain 44x44px minimum touch targets.
- Support keyboard, screen reader, 200% zoom, reduced motion, light/dark themes, and
  mobile safe areas.
- Verify 375, 768, 1024, and 1440px widths.

Forbidden:

- Generic purple AI styling
- Decorative gradient/orb backgrounds
- Emoji as structural icons
- Landing-page hero in place of the application
- Raw HTML from model output
- Hover-only actions
- Layout-shifting hover/press animation
- Visible implementation jargon when plain user language exists

## Authentication and security rules

- Access JWTs are short-lived and verified for issuer, audience, expiry, session,
  and key ID.
- Refresh tokens are opaque, hashed at rest, rotated atomically, and protected by
  reuse detection.
- Browser tokens remain in `Secure`, `HttpOnly`, `SameSite=Lax` cookies.
- Unsafe cookie-authenticated requests require CSRF protection.
- OAuth state and PKCE are server-generated, single-use, and browser-bound.
- OAuth return locations are allowlisted.
- WebSockets require short-lived single-use tickets and strict `Origin` checks.
- Authorization happens server-side for every owned resource and admin action.
- Account deletion and role changes require recent authentication.
- Rate limiting must fail safely when Redis is unavailable.
- Secrets come from environment/secret management and are never committed.
- `.env.example` contains names and safe placeholders only.

Run threat modeling before authentication, realtime, admin, or deletion slices are
considered complete.

## Realtime rules

- Persist command intent before sending `command.acknowledged`.
- Use idempotency keys for generation and retry.
- Permit one active generation per actor/conversation.
- Propagate client cancellation to the provider.
- Never automatically replay a provider call after partial output.
- Bound outbound buffers and disconnect slow clients with a retryable reason.
- Treat REST/PostgreSQL state as authoritative after missed replay windows.
- Test reconnect, duplicate commands, out-of-order deltas, Redis loss, cancellation,
  and server shutdown.

## Provider adapter rules

- Internal model IDs are stable and independent of upstream names.
- Provider sync imports metadata but never enables a model.
- Only expose models supported by an installed adapter.
- Normalize finish reasons, usage, timeouts, and error classes.
- Backoff is allowed before the first streamed chunk, not after partial output.
- Include a fake deterministic provider for unit/integration/E2E tests.
- Development may use OpenCode Go.
- Production code must not assume OpenCode URLs, model prefixes, limits, or account
  semantics.

## Testing expectations

Every behavioral change includes the smallest test set that proves it and prevents
likely regressions.

Required by risk:

- Domain rule: table-driven unit tests
- Database behavior: real PostgreSQL integration test
- Redis coordination: real Redis integration test plus unavailable-Redis behavior
- API change: OpenAPI examples and contract/handler tests
- WebSocket change: event fixture and ordering/reconnect tests
- Auth or quota change: adversarial and concurrency tests
- UI change: component interaction/accessibility test
- User journey: Playwright E2E
- Shared concurrency: Go race detector

Do not make tests pass by weakening assertions, adding arbitrary sleeps, or
exposing production internals. Use fake clocks, deterministic IDs, eventual
assertions with bounded time, and injected adapters.

## Observability rules

- Propagate request, event, generation, and trace IDs.
- Use structured fields with stable names.
- Metrics use bounded-cardinality labels.
- Hash or omit identifying actor data.
- Never attach message content to logs, spans, metrics, error trackers, or analytics.
- New background jobs include success, failure, retry, age, and dead-letter signals.
- New critical failure modes include an alert and runbook update.

## Documentation and decisions

Update documentation in the same change when behavior or assumptions change.

Create an ADR when choosing:

- A durable dependency or framework
- A security mechanism
- A contract/versioning approach
- A cross-slice data ownership rule
- A deployment topology
- A production provider

An ADR records context, decision, alternatives, consequences, and migration/rollback
considerations. It does not rewrite the implementation.

## Change discipline

- Keep changes scoped to one task or coherent vertical slice.
- Preserve unrelated user changes in a dirty worktree.
- Do not perform unrelated refactors.
- Do not commit generated artifacts that are not reproducible.
- Do not bypass hooks or CI checks.
- Use conventional commit prefixes when commits are requested:
  `feat`, `fix`, `docs`, `test`, `refactor`, `build`, `ci`, `chore`.
- Include migration, configuration, deployment, and rollback notes in pull requests
  when applicable.

## Definition of done

A task is done only when:

1. Acceptance criteria in [TASKS.md](./TASKS.md) are met.
2. Contracts and documentation match behavior.
3. Unit/integration/contract/E2E tests appropriate to risk pass.
4. Formatting, linting, type checks, vulnerability checks, and race tests pass where
   applicable.
5. Security, privacy, accessibility, localization, and observability impacts are
   addressed.
6. No prompt, response, token, or secret leaks through logs or client bundles.
7. Operational rollout and rollback are understood.
8. Generated files are reproducible and clean.

## Stop conditions

Stop and request a decision when:

- A task conflicts with a product invariant.
- A public breaking change is required.
- A migration risks destructive data loss.
- Production provider terms, privacy, or capacity are unclear.
- A security control would be removed or materially weakened.
- Required secrets or external credentials are unavailable for verification.
