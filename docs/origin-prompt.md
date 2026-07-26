# Origin Prompt

This is the request that started Glazz, before any file existed.

It is reproduced as it was written, translated from the original Spanish. Nothing
was added, removed, or reordered, and no decision was inserted after the fact. It is
kept here because the rest of this documentation set is large enough that a reader
deserves to know where it came from and how much of it was specified up front.

---

> I want you to help me create a production-ready application: a ChatGPT-style chat.
>
> For the technology stack I would like to use Next.js for the frontend, Golang for
> the backend, and Postgres for the database. Also use Tailwind CSS and shadcn. Use
> WebSockets for realtime communication, use Google as the login system, and use JWT
> for authentication. It is a monorepo with a dedicated folder for the frontend and
> one for the backend.
>
> As a business rule there are two kinds of user, registered and unregistered.
> Everything core revolves around the registered user, but to attract users we should
> offer the option to interact very briefly with the chat, using only a couple of
> prompts or some other mechanism, and once that limit is reached, ask them to log
> in.
>
> Use the OpenAI-compatible OpenCode API to provide the service, use DeepSeek as the
> default LLM, and allow its exposed models to be configured.
>
> We are going to make a complete plan, generating PROJECT.md, ARCHITECTURE.md,
> DESIGN.md and AGENTS.md. Along with a TASKS.md to list the step by step of how to
> develop the whole project. We will start by defining the API from the description
> of the endpoints, then build the API, then the frontend, integration, testing, and
> so on. Every step must be very well planned.
>
> I am leaving you some skills for the design and a `.env` file with the values for
> connecting to the AI provider.
>
> Ask me all the questions you consider necessary in order to define this project
> clearly.

---

## What this fixed, and what it did not

The prompt settled the constraints and the method. It did not settle the design.

Fixed up front:

- the stack, including the Go/TypeScript split and the monorepo layout;
- the guest-to-registered funnel as a business rule, including the idea of a
  deliberately short guest trial ending in a login gate;
- an OpenAI-compatible provider with a configurable exposed model catalog, rather
  than a hard dependency on one model;
- **contract-first sequencing**: define the API from its endpoints, then the API,
  then the web application, then integration and testing;
- the documents that would govern the work — `PROJECT.md`, `ARCHITECTURE.md`,
  `DESIGN.md`, `AGENTS.md` — plus `TASKS.md` as the step-by-step tracker;
- that every step had to be planned before being built.

Left open, and answered later through a requirements questionnaire before any code
was written:

- guest limits, their reset behavior, and how a guest conversation migrates on
  sign-in;
- registered quotas, roles, and administrative surface;
- session strategy, refresh handling, and revocation;
- retention and deletion, for guests and for accounts;
- realtime ordering, reconnect, replay, and cancellation semantics;
- observability, and the rule that prompt and response content never enters logs,
  traces, metrics, or audit records.

## How the plan turned into the repository

| Stage              | Artifact                                                                       |
| ------------------ | ------------------------------------------------------------------------------ |
| The request        | this document                                                                  |
| Requirements       | a questionnaire answered before implementation                                 |
| Product scope      | [PROJECT.md](../PROJECT.md)                                                    |
| Architecture       | [ARCHITECTURE.md](../ARCHITECTURE.md), [docs/adr/](./adr/)                     |
| Design system      | [DESIGN.md](../DESIGN.md)                                                      |
| Working rules      | [AGENTS.md](../AGENTS.md)                                                      |
| Delivery plan      | [TASKS.md](../TASKS.md)                                                        |
| Execution evidence | [WORKLOG.md](../WORKLOG.md), milestone records `docs/m0-*` through `docs/m6-*` |

The delivery model that followed — seven milestones, twelve phases, minor SemVer
tags from M2 onward, and release gates before production — is described in the
[README](../README.md#delivery-model). A feature being implemented does not mean its
milestone was accepted.

Glazz was built with coding agents throughout, under the rules in
[AGENTS.md](../AGENTS.md). That is why the specifications, the acceptance evidence,
and the verification gates are part of the repository rather than notes kept
elsewhere: they are what the work was steered by.
