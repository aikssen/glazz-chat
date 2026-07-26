# Requirements Questionnaire

This is the requirements round that turned the [origin prompt](./origin-prompt.md)
into a specification. It ran **before any code was written**, and its answers are
what `PROJECT.md`, `ARCHITECTURE.md`, `DESIGN.md` and `TASKS.md` were built from.

## How to read this document

The prompt ended with "ask me all the questions you consider necessary." What came
back was a questionnaire: 61 questions, most of them carrying a **proposed default**.
Each default was reviewed before being accepted or overridden.

Two things follow from that, and both are visible below rather than smoothed over:

**"Accepted the default" is shorthand, not silence.** Where a proposal was sound,
the answer written at the time was simply that — a deliberate economy while working
through 61 decisions in one pass. It records agreement after review, not absence of
review.

**Where the proposal was wrong for this product, it was overridden.** Those answers
are the useful ones to read first: message editing, regeneration and branching were
cut from the MVP (23); content formats were narrowed (24); account deletion was
pulled _into_ the MVP instead of deferred (15); guest data retention was made
**stricter** than proposed, daily instead of seven days (53); the provider was routed
behind an internal interface rather than called directly (18); and the model catalog
came back with a real list plus a new requirement — an administrator must be able to
configure what is exposed (20).

The closing note of the questionnaire was also unprompted: make the code testable
from the start, use vertical slicing on both sides, and use dependency injection in
Go. That shaped the package layout the repository still has.

## About the rationale blocks

Twelve decisions carry a **Rationale** block. Those were **recorded later**, on
2026-07-26, while preparing the repository for public release — they are the reasoning
behind the original answer, written down after the fact, not text from the
questionnaire as it was filled in. They are marked so nobody has to guess which is
which. Decisions without a rationale block have none recorded; that is not an
invitation to assume one.

The original was written in Spanish. This is a translation; no answer was reworded to
sound better than it was.

---

## Product

### 1. Product name

What will the provisional product name and its domain be?

**Default:** `LLM Chat` as a temporary name.

**Answer:** Glazz, http://glazz.hlab.sh

### 2. Primary audience

Who is the primary audience: developers, companies, students, or the general public?

**Answer:** General public.

### 3. Languages

Will the interface be Spanish only, English only, or bilingual?

**Default:** Spanish and English, starting in Spanish.

**Answer:** Bilingual.

### 4. MVP scope

Will the MVP be chat only, or include files, images, web search, tools, or agents
from the start?

**Default:** Text chat with Markdown and code blocks; attachments and tools out of
scope for the MVP.

**Answer:** Accepted the default.

### 5. Conversation management

Which operations does a conversation need: create, rename, delete, archive, search,
share, export?

**Default:** Create, rename, delete, archive and search. No public links initially.

**Answer:** Accepted the default.

## Guest users

### 6. Guest identification

How is a guest identified: anonymous cookie, browser/device, or IP?

**Default:** A signed identifier in an `HttpOnly` cookie, complemented by per-IP rate
limiting.

**Answer:** Accepted the default.

### 7. Free limit

What will the free limit be: messages, tokens, conversations, or a time window?

**Default:** 4 user messages in a single conversation, with a maximum of 2,000
generated tokens in total.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): A hard trial limit that forces registration is
> normal and standard in the industry. Any starting number would have worked, but a
> low one serves to get the project up and running as fast as possible, and the values
> can be adjusted afterwards.

### 8. Limit reset

Does the limit reset?

**Default:** Not automatically; it requires signing in.

**Answer:** Accepted the default.

### 9. Anonymous conversation migration

Should the anonymous conversation be preserved after login?

**Default:** Yes, migrated to the registered account transactionally.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): It is much easier to work with a real structure
> that you will adopt as part of your business model than to create temporary assets
> you then have to transform to fit it. Giving the conversation its real structure from
> the start makes moving that data from an anonymous user to a registered one simple,
> and avoids failures, reprocessing, and compute cost.

### 10. Model selection for guests

Can a guest choose a model?

**Default:** No; it uses the default model configured by the server.

**Answer:** Accepted the default.

## Registered users

### 11. Plans and roles

Will all registered users share one plan, or should we prepare roles and plans?

**Default:** A single plan, with the schema prepared for `free`, `pro` and `admin`.

**Answer:** Accepted the default.

### 12. Registered user quotas

Will there be quotas for registered users?

**Default:** Configurable daily limits for messages, tokens and concurrency.

**Answer:** Accepted the default.

### 13. Authentication providers

Will Google be the only login, or should the architecture be prepared for more
providers?

**Default:** Google only in the MVP, with an extensible identities table.

**Answer:** Accepted the default.

### 14. Domain restrictions

Will access be restricted to specific email domains?

**Default:** Any verified Google account.

**Answer:** Accepted the default.

### 15. Account privacy

Do we need account deletion and data export?

**Default:** Both, though they can be implemented after the initial MVP.

**Answer:** _Deletion in the MVP._ — an override: deferring it was not acceptable.

## Provider and models

### 16. Provider identity

Confirm that "OpenCode" is the real provider and not "OpenRouter". What is its exact
name and documentation?

**Answer:** OpenCode, documentation: https://opencode.ai/docs

### 17. Default DeepSeek model

Which exact DeepSeek identifier should be used by default?

**Examples:** `deepseek-chat`, `deepseek-v3`, `deepseek/deepseek-chat`.

**Answer:** `DeepSeek V4 Flash`

### 18. API_URL format

Does `API_URL` point at the provider root or directly at `/v1/chat/completions`?

**Answer:** _Route the provider through our own URLs so the provider can be swapped
easily. A Go interface will allow switching to any provider compatible with the
OpenAI/ChatGPT API._

This answer, not a default, is why the repository has a provider gateway rather than
provider calls spread through the domain.

### 19. Provider streaming

Does compatibility include OpenAI-style streaming over SSE?

**Default:** Go consumes the provider stream and relays it to the browser over
WebSocket.

**Answer:** Accepted the default.

### 20. Exposed models

What does "allow its exposed models to be configured" mean?

**Default:** An allowlist managed through environment variables and configuration,
with display name, upstream identifier, limits, capabilities and a default model.

**Answer:** _OpenCode currently offers a set of models in its subscription — Grok 4.5,
GLM-5.2, GLM-5.1, Kimi K3, Kimi K2.7 Code, Kimi K2.6, MiMo-V2.5-Pro, MiMo-V2.5,
Qwen3.7 Max, Qwen3.7 Plus, Qwen3.6 Plus, MiniMax M2.7, MiniMax M3, DeepSeek V4 Pro,
DeepSeek V4 Flash and Hy3. These can be exposed to the user, but I want the exposed
list to be configurable by an administrator._

The administrator requirement was added here, and it is what later produced the
separation between discovering a model and exposing it.

### 21. Model per conversation

Can users select a model per conversation?

**Default:** Yes for registered users; no for guests.

**Answer:** Accepted the default.

### 22. LLM usage metrics

Should we store metrics for tokens, latency, model, errors and estimated cost?

**Default:** Yes, without storing payloads in logs.

**Answer:** Accepted the default.

## Chat

### 23. Editing, regeneration and branches

Will messages be editable and regenerable, creating branches, or only regenerate the
last response?

**Default:** Edit and regenerate with logical branching, even if the initial UI shows
a single active branch.

**Answer:** _No, only a basic conversation in the MVP._ — an override that removed a
substantial amount of scope.

### 24. Content formats

Which formats must the frontend render?

**Default:** Markdown, GFM, tables, highlighted code, LaTeX, copy-code and simple
references.

**Answer:** _Markdown, tables, code._ — narrower than proposed.

### 25. Stopping a generation

Do we need a stop button?

**Default:** Yes, with cancellation propagated to the provider.

**Answer:** Accepted the default.

### 26. Context window management

Will the full history be sent on every prompt, or will there be automatic
summarization and compaction?

**Default:** A configurable context budget and summarization once a threshold is
reached.

**Answer:** _Use summarization._

> **Rationale** (recorded 2026-07-26): This was also a development-phase decision. Given
> the subscription being used for testing, I did not want to burn through the quota at
> once knowing there would be a lot of test traffic. It is economics — and there has to
> be an adaptation phase before going to production anyway.

### 27. System prompts

Are system prompts configurable?

**Default:** One global prompt managed by the backend; no per-user editing in the MVP.

**Answer:** Accepted the default — _and this matters for adding safety guardrails._

## Security and API

### 28. JWT strategy

Do you accept short-lived access JWTs plus a rotating refresh token in `Secure`,
`HttpOnly`, `SameSite=Lax` cookies? This is preferable to storing JWTs in
`localStorage`.

**Default:** A 15-minute access token and a 30-day refresh token.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): This is a strategy I have used before — paired
> tokens, with a short-lived access token. Fifteen minutes is appropriate, and 30 days
> for the refresh token is appropriate too: it gives enough headroom to avoid excessive
> API calls, and it works well on mobile devices.

### 29. Session management

Do we need per-device sessions, remote sign-out and revocation?

**Default:** Yes.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): Because I already know how access and refresh
> pairs behave, I know how token revocation works — and it is easier when tokens are
> issued per device and stored in the platform's database. That way you know which
> tokens were handed out and how to revoke them.

### 30. Regulatory requirements

Are there regulatory or data-residency requirements?

**Default:** Basic GDPR practices, with no specific regional requirement.

**Answer:** Accepted the default.

### 31. Public API access

Will the API be public for third parties?

**Default:** No. It will be documented with OpenAPI but consumed only by the frontend
initially.

**Answer:** Accepted the default — _open for a mobile client in the future._

### 32. API versioning

Do you want versioning from the beginning?

**Default:** HTTP under `/api/v1`, and the WebSocket protocol versioned through event
types.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): I know the problem that migration and versioning
> represent in a product. I have lived through it many times: if you plan for it ahead
> of time, it should not become a pain point later.

## Infrastructure

### 33. Deployment

Where will it be deployed?

**Suggested default:** Frontend on Vercel; Go API, managed PostgreSQL and Redis on a
container-compatible platform.

**Answer:** Accepted the default — _and use Docker Compose to run the project locally
for development._

### 34. Redis

Can we include Redis? It is recommended for distributed rate limiting, presence,
WebSocket pub/sub and ephemeral jobs. PostgreSQL remains the source of truth.

**Default:** Yes.

**Answer:** Accepted the default.

### 35. Migrations

Which migration tool do you prefer?

**Default:** `goose` with explicit SQL, and queries through `sqlc`.

**Answer:** Accepted the default.

### 36. PostgreSQL access

ORM or typed SQL?

**Default:** `pgx` + `sqlc`, avoiding a heavyweight ORM.

**Answer:** Accepted the default.

### 37. Monorepo structure

A monorepo with `apps/web` and `apps/api`, or literally `frontend` and `backend`?

**Default:** `apps/web` and `apps/api`, plus `packages/contracts`, `deploy`, `docs`
and `scripts`.

**Answer:** Accepted the default.

### 38. Observability

Which observability provider will we use?

**Default:** OpenTelemetry, JSON logs, Prometheus, and Sentry optionally.

**Answer:** Accepted the default.

### 39. CI/CD

Will we use GitHub Actions for CI/CD?

**Default:** Lint, types, tests, verified migrations, security analysis, image builds
and E2E tests.

**Answer:** Accepted the default.

## Design

### 40. Visual identity

Do you have a brand, logo, colors, or visual references? The skill's initial automatic
recommendation produced the typical purple AI-application scheme; discarding it as
generic is proposed.

**Answer:** _I do not have a design right now, so I am asking for a suggestion. I am
thinking of strong colors, an orange as the primary color, and modern typefaces —
something with a fresh look and feel._

### 41. Themes

Should it support light, dark, or both?

**Default:** Both, respecting the system preference.

**Answer:** Accepted the default.

### 42. Visual relationship to ChatGPT

How close to ChatGPT should it feel: a familiar structure, or a clearly different
visual identity?

**Default:** A familiar flow, its own identity, and a quiet interface focused on
reading.

**Answer:** Accepted the default.

### 43. Platforms

Are desktop and mobile web enough, or do you need an installable PWA?

**Default:** A responsive application plus a basic PWA.

**Answer:** Accepted the default.

## Documentation

### 44. ARCHITECTURE.md name

Do you confirm we should correct `ARCHIRECTURE.md` to `ARCHITECTURE.md`?

**Default:** Use `ARCHITECTURE.md`.

**Answer:** Accepted the default.

### 45. Documentation language

Should the documents be written in Spanish or English?

**Default:** Technical English for documentation and contracts; a bilingual interface.

**Answer:** Accepted the default.

### 46. TASKS.md scope

Should `TASKS.md` cover only the MVP, or include later phases?

**Default:** Foundation, API contract, backend, frontend, integration, testing,
hardening, deployment and post-MVP, with acceptance criteria and dependencies per
task.

**Answer:** Accepted the default.

**Closing note, added unprompted:** _Make the code testable from the start, using
vertical slicing for the backend and the frontend, and use dependency injection in
Go._

---

## Second round: outstanding decisions

This section was added after reviewing the answers and checking the integration
against the provider's official documentation.

### 47. DeepSeek variant

The provider exposes two relevant variants: `deepseek-v4-flash`, pay per use, and
`deepseek-v4-flash-free`, a temporary free version whose requests may be collected
and used to improve the model. Which should be the Glazz default?

**Recommended default:** `deepseek-v4-flash`, avoiding the free endpoint for privacy
and stability.

**Answer:** Accepted the default — _and use OpenCode Go, not OpenCode Zen._

### 48. Model catalog

The list written in answer 20 does not fully match the provider's current catalog and
may change over time. Should Glazz synchronize the provider's model endpoint
periodically, or maintain a manually managed internal catalog?

**Recommended default:** Synchronize provider metadata periodically, keep an internal
catalog, and require an administrator to enable each model before it is exposed. A
provider addition never enables a new model automatically.

**Answer:** Accepted the default — _remember, we use OpenCode Go, not Zen._

> **Rationale** (recorded 2026-07-26): This is a business decision. You cannot depend
> one hundred percent on a provider: OpenCode can change its model catalog from one
> moment to the next, and on the product side we have to preserve some consistency. It
> also takes a proper evaluation of which models we can actually offer — a curated
> list rather than OpenCode's entire list — for cost, capacity and limitations.

### 49. Administration in the MVP

What should an administrator be able to configure from the application?

**Recommended default:** Visible models, default model, quotas, guest limits, system
prompt, maintenance state, and basic usage and error inspection. Every modification
must be audited.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): An administration section is not something you
> can bolt on later. You have to plan from the start what will be administered and how
> it lives inside the product. You cannot ship a product on defaults where changing
> anything means going through a full development and deployment cycle, when an
> administrator could sign in and change a parameter in a minute. Auditing also tells
> you how the product actually works and how it is used, and it is an integral part of
> identifying failures.

### 50. Creating the first administrator

How will the first `admin` role be assigned?

**Recommended default:** An email allowlist through an environment variable for
initial bootstrap. Administrators can promote or demote users afterwards.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): The project needs to be cloneable and runnable
> as fast as possible, and development has to stay simple. An environment variable also
> keeps the bootstrap administrator out of the repository: no user's email is carried in
> version control, the identity stays abstracted, and it can be changed quickly while
> developing.

### 51. Initial quota values

We need concrete values for registered users, even if an administrator can change them
later.

**Recommended default:** 50 user messages per day, 50,000 output tokens per day, one
concurrent generation per user, plus configurable global spend limits. Show consumption
and the reset time in the interface.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): Limiting the product was my own requirement, but
> I did not have clear starting values. From experience, 50,000 tokens is enough for a
> fluid conversation, and more than enough during development. That is the key point:
> being able to test in development and adjust in production.

### 52. Data retention and deletion

What should happen to conversations, metrics and logs when a user deletes their
account?

**Recommended default:** Revoke sessions immediately, delete personal data and
conversations through an asynchronous job within 24 hours, keep only non-identifiable
aggregate metrics, and retain security logs for 30 days. The action requires
reauthentication and explicit confirmation.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): Deleting everything instantly is intrusive, and
> it can interfere with the user's own decision — hence an asynchronous purge with a
> window rather than an immediate one. The 30 days of security logs is partly a legal
> consideration, but logs also carry business value: identifying user behaviors and
> trends, spotting failures, and finding opportunities to improve the product. That is
> the reason for keeping non-identifiable aggregate metrics after the personal data is
> gone.

### 53. Guest retention

How long is an anonymous conversation kept if the visitor never signs in?

**Recommended default:** 7 days since last activity, then automatic deletion. The quota
cookie may be kept longer without containing conversation text.

**Answer:** _Delete daily._ — an override that made retention stricter than proposed.

### 54. Content safety

A system prompt helps steer the model but is not a safety control on its own. What
level of moderation does the MVP need?

**Recommended default:** Size and format validation, automated abuse blocking, rate
limiting, configurable safety categories, input and output filters decoupled from the
provider, and a mechanism to report responses. Do not store prompts in operational
logs.

**Answer:** Accepted the default.

> **Rationale** (recorded 2026-07-26): I know how users can abuse a product, whether
> intentionally or not. Considering moderation early gives you at least a floor to
> operate from, and the ability to make adjustments on the fly.

### 55. Recovery from failed responses

Even though the MVP allows no editing, regeneration or branching, can a failed or
interrupted response be retried?

**Recommended default:** Yes. Allow retrying only the last failed or cancelled
generation, idempotently and without creating visible branches.

**Answer:** Accepted the default.

### 56. Summarization strategy

Which model performs context summarization, and when does it trigger?

**Recommended default:** A configurable, inexpensive summarization model, triggered
when the conversation reaches 70% of the effective context window. Store the summary
as versioned internal state, never as an editable message.

**Answer:** Accepted the default.

### 57. Production backend destination

Vercel covers the frontend, but where the Go API, PostgreSQL and Redis will live is
still open. This affects persistent WebSockets, private networking, backups and the
deployment pipeline.

**Recommended default:** Keep the architecture portable with containers and choose a
provider before implementing CI/CD.

**Answer:** Accepted the default.

### 58. Backups and operational objectives

What availability and recovery level does the MVP expect?

**Recommended default:** Daily PostgreSQL backups with 7-day retention,
point-in-time recovery where the provider allows it, a 99.5% availability target, a
24-hour RPO and a 4-hour RTO. Redis holds nothing irreplaceable.

**Answer:** Accepted the default — _disabled by default._

### 59. Google OAuth and domain

Will the domain be available over HTTPS in production, and can we create Google OAuth
credentials for its callback URLs?

**Required default:** HTTPS, `Secure` cookies, separate callbacks for development and
production, and OAuth secrets available only in the backend.

**Answer:** Accepted the default, _with options to configure this, since that is the
final domain._

### 60. Age, terms and privacy

As a general-public application that processes conversations through third parties,
will there be a minimum age and acceptance of terms and a privacy policy?

**Recommended default:** A minimum age of 18 for the MVP, visible links to terms and
privacy before using the chat, and explicit consent at account creation. Do not claim
full legal compliance until a legal review is done.

**Answer:** Accepted the default.

### 61. Provider protocols in the MVP

The provider catalog does not use a single protocol: some models use
`/v1/chat/completions`, others use the Anthropic protocol, others OpenAI Responses or
Google's. Should the MVP be limited to OpenAI Chat Completions models, or implement
adapters for all of those protocols from the start?

**Recommended default:** Limit the MVP to OpenAI Chat Completions, with an agnostic
internal interface and an architecture ready to add `responses`, `anthropic` and
`google` adapters later. The administration panel will only allow enabling models
supported by an installed adapter.

**Answer:** Accepted the default — _OpenCode Go._

---

## What happened next

These answers became [PROJECT.md](../PROJECT.md), [ARCHITECTURE.md](../ARCHITECTURE.md),
[DESIGN.md](../DESIGN.md), [AGENTS.md](../AGENTS.md) and the phase plan in
[TASKS.md](../TASKS.md), in that order, before implementation started. Several
decisions here are visible in the code as direct consequences: the provider gateway
(18), the discover-then-expose split in the model catalog (20, 48), per-device session
revocation (29), the audited administration surface (49), and daily guest cleanup in
the worker (53).

Where a decision was later revised, the revision lives in
[docs/adr/](./adr/) — an accepted architecture decision record outranks an answer in
this questionnaire.
