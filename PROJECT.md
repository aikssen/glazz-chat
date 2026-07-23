# Glazz

## Document status

- Status: Planning baseline
- Product domain: `https://glazz.hlab.sh`
- Primary audience: General public
- Interface languages: Spanish and English
- MVP target: Production-ready web application
- Last reviewed: 2026-07-23

This document defines what Glazz is, why it exists, and what the MVP must deliver.
Technical implementation details belong in [ARCHITECTURE.md](./ARCHITECTURE.md),
visual and interaction rules in [DESIGN.md](./DESIGN.md), contributor rules in
[AGENTS.md](./AGENTS.md), and execution order in [TASKS.md](./TASKS.md).

## Product statement

Glazz is a focused, bilingual AI chat application for the general public. It lets
visitors experience a short, useful conversation before asking them to sign in,
then gives registered users persistent conversations, model choice, predictable
usage limits, and privacy controls.

Glazz must feel familiar enough to use without instruction, but it must not be a
visual clone of ChatGPT. The product should be fast, readable, fresh, and explicit
about generation state, limits, errors, and data handling.

## Product goals

1. Let a first-time visitor reach a useful AI response with no account.
2. Convert an engaged guest into a registered user without losing the conversation.
3. Provide reliable streamed chat with cancellation and recoverable failures.
4. Keep the LLM provider replaceable through a provider-neutral Go interface.
5. Give administrators runtime control over models, quotas, safety settings, and
   maintenance state.
6. Make privacy, deletion, authentication, quotas, observability, and testing part
   of the MVP rather than later patches.
7. Support a future mobile client without redesigning the API.

## Non-goals for the MVP

- File or image uploads
- Image generation
- Web search
- Tools, function calling, agents, or MCP
- User-defined system prompts
- Public conversation sharing
- Conversation export
- Message editing, branching, or general regeneration
- Billing or paid Glazz plans
- Native mobile applications
- Provider protocols other than OpenAI-compatible Chat Completions
- Legal certification or claims of complete regulatory compliance

## Users and roles

### Guest

A guest can create one short-lived conversation and send at most four user
messages, subject to a total response budget of 2,000 output tokens. A signed,
`HttpOnly` cookie identifies the guest; IP-based controls supplement it for abuse
prevention. The limit does not reset automatically.

A guest:

- Uses the server-selected default model.
- Cannot select a model.
- Can stop an active generation.
- Can retry only the latest failed or cancelled generation.
- Is prompted to sign in when the guest allowance is exhausted.
- Can migrate the current conversation into a registered account after login.
- Has conversation data deleted by a daily cleanup job if it was not migrated.

### Registered user

A registered user authenticates with a verified Google account. The initial plan is
`free`; the schema remains extensible for `pro` and `admin`.

A registered user can:

- Create, rename, archive, search, and delete conversations.
- Select an enabled model per conversation.
- Read streamed Markdown, tables, and syntax-highlighted code.
- Stop an active generation.
- Retry only the latest failed or cancelled generation.
- Review quota usage and its reset time.
- Manage and revoke device sessions.
- Delete the account in the MVP.
- Use light, dark, or system theme.

Initial registered-user limits:

- 50 user messages per day
- 50,000 output tokens per day
- One concurrent generation per user
- Additional global cost and concurrency limits controlled by administrators

### Administrator

An administrator can:

- Enable and disable models supported by an installed provider adapter.
- Select the default guest and registered-user models.
- Configure guest and registered-user quotas.
- Configure the global system prompt and safety categories.
- Put the service into maintenance mode.
- Inspect aggregate usage and operational errors without reading prompt bodies.
- Promote or demote users.
- Review an immutable audit trail of administrative changes.

The first administrator is bootstrapped from an environment-based email allowlist.

## Core journeys

### Guest activation

1. A visitor opens Glazz and sees the chat composer immediately.
2. The backend creates or resumes an anonymous guest session.
3. The visitor sends a message and receives a streamed response.
4. Remaining guest usage is visible but not distracting.
5. At the limit, the composer becomes an authentication gate rather than silently
   failing.
6. Google login migrates the guest conversation transactionally.

### Registered chat

1. The user signs in with Google.
2. The most recent conversation or a clean new-chat state opens.
3. The user selects an enabled model and sends a message.
4. Glazz acknowledges the request, streams the response, and updates usage.
5. The user can cancel, retry a failed response, rename, archive, search, or delete
   the conversation.

### Account deletion

1. The user opens privacy settings and requests deletion.
2. Glazz requires recent Google reauthentication and explicit confirmation.
3. Sessions are revoked immediately and the account enters deletion state.
4. A worker removes personal data and conversations within 24 hours.
5. Only non-identifying aggregate metrics remain; security logs expire after 30
   days.

## Content behavior

- Messages support Markdown, tables, and fenced code blocks.
- Raw HTML from model output is not rendered.
- The system prompt is controlled by the backend and cannot be overridden by the
  client.
- Context is summarized when a conversation reaches 70% of the effective model
  context window.
- The summarization model is configurable and should be cheaper than the active
  chat model where possible.
- Summaries are internal, versioned state and are never presented as editable user
  messages.

## Authentication and privacy policy

- Google is the only MVP identity provider.
- Access JWT lifetime: 15 minutes.
- Refresh-token lifetime: 30 days with rotation and reuse detection.
- Browser credentials use `Secure`, `HttpOnly`, `SameSite=Lax` cookies.
- Tokens are never stored in browser local storage.
- Sessions are per device and independently revocable.
- Prompts and model responses are not written to operational logs.
- Account creation requires acceptance of Terms and Privacy Policy.
- MVP minimum age: 18.
- The interface must not claim complete legal compliance without legal review.

## Provider strategy

The product domain must not depend on any provider-specific type, model prefix, or
error shape.

For development:

- OpenCode Go is the initial provider.
- Base URL: configured through environment; currently
  `https://opencode.ai/zen/go/v1`.
- Default upstream model ID: `deepseek-v4-flash`.
- Model metadata may be synchronized from the provider and copied into Glazz's
  internal catalog.

For production:

- OpenCode Go is not the intended final provider.
- A provider must pass privacy, capacity, commercial-use, reliability, and cost
  review before launch.
- Configuration chooses the active provider without changing the public API.
- The first adapter contract supports OpenAI-compatible `/chat/completions`.
- Adapters for OpenAI Responses, Anthropic, or Google are post-MVP work.

## Technical baseline

- Monorepo: `apps/web`, `apps/api`, `packages/contracts`, `deploy`, `docs`, `scripts`
- Web: Next.js App Router, TypeScript, Tailwind CSS, shadcn/ui
- API: Go, vertical slices, dependency injection
- Primary database: PostgreSQL
- Ephemeral coordination: Redis
- Realtime transport: WebSocket between browser and Go API
- Provider transport: upstream streaming protocol, adapted by Go
- API description: OpenAPI 3.1 plus a versioned WebSocket event schema
- Local development: Docker Compose
- CI/CD: GitHub Actions
- Observability: structured JSON logs, OpenTelemetry, Prometheus; Sentry optional

Dependencies are pinned at scaffold time. The repository uses supported stable
releases and automated dependency updates; it does not use floating versions in
production.

Context7 is the required first source for current framework/library documentation
and version discovery, followed by confirmation against the upstream official
release source. The local `context7-cli` skill is locked in `skills-lock.json`.
Exact versions and the date/source of each decision are recorded when dependencies
are introduced.

## Product quality targets

- WCAG 2.2 AA for core journeys
- Responsive at 375, 768, 1024, and 1440 CSS pixels
- Keyboard-complete operation
- Visible and announced generation, error, quota, and connection states
- First streamed content target: p95 under 3 seconds, excluding provider outages
- API availability objective: 99.5% for the MVP
- No known critical or high security findings at release
- No loss of committed user messages after an acknowledged write
- Idempotent retry for chat submission and destructive commands

## Success measures

Instrumentation must support these measures without logging conversation content:

- Guest prompt completion rate
- Guest-to-registered conversion after reaching the limit
- First response and full response latency by model/provider
- Generation success, cancellation, and retry rates
- Daily active registered users
- Messages and output tokens by anonymous plan/model cohort
- Quota rejection rate
- WebSocket reconnect rate
- Account deletion completion time

Numeric business targets are deferred until a baseline cohort exists.

## Operational assumptions

- Frontend production hosting begins on Vercel.
- API, PostgreSQL, and Redis remain container-portable until a hosting provider is
  selected.
- PostgreSQL is the source of truth.
- Redis may be flushed without losing durable data.
- Backups are disabled in local development. Production backup activation is a
  release gate and depends on the selected managed database.
- Planned production objective: daily backups, seven-day retention, RPO 24 hours,
  RTO 4 hours, and point-in-time recovery when supported.

## Principal risks

| Risk | Mitigation |
| --- | --- |
| Development provider differs from production | Provider-neutral domain interface, contract tests, production-provider approval gate |
| Guest limits bypassed through cookies or IP rotation | Signed identity, IP and device signals, Redis rate limits, global budget circuit breaker |
| WebSocket disconnect duplicates generations | Idempotency keys, persisted generation state, resume cursor, single active generation |
| JWT or refresh-token theft | HttpOnly cookies, rotation, reuse detection, session revocation, CSRF and origin controls |
| Model catalog changes | Internal catalog, explicit admin enablement, capability validation |
| System prompt treated as a safety boundary | Separate input/output policy pipeline, limits, reporting, monitoring |
| Context summaries distort prior intent | Versioned summaries, thresholds, tests, original message retention |
| Public launch exceeds provider capacity | Global concurrency/cost limits, circuit breaker, approved fallback adapter |
| Bilingual copy diverges | Typed translation keys, parity checks, English fallback |

## Release gates

The MVP is releasable only when:

1. API and WebSocket contracts are reviewed and versioned.
2. The production provider is approved and load-tested.
3. Google OAuth production credentials and HTTPS callbacks work.
4. Database migrations, rollback procedures, backups, and restore drills pass.
5. Guest limits and account migration pass adversarial tests.
6. Refresh rotation, reuse detection, CSRF, CORS, CSP, and WebSocket origin checks
   pass security review.
7. Account deletion finishes within the stated deadline.
8. Accessibility, responsive, unit, integration, contract, race, and E2E suites pass.
9. Dashboards, alerts, runbooks, and on-call ownership exist.
10. Terms and Privacy Policy receive legal review.

## Open decisions

These decisions do not block API-first implementation but must close before the
corresponding release gate:

- Production hosting provider for API, PostgreSQL, and Redis
- Final production LLM provider and commercial agreement
- Sentry or alternative hosted error-tracking provider
- Numeric business targets after baseline usage
