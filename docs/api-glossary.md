# API Glossary

This glossary defines public and internal terms used by Glazz contracts.
Provider-specific vocabulary is excluded from public resources.

| Term | Definition |
| --- | --- |
| Actor | The authenticated principal for a request: a guest, registered user, or administrator. |
| Guest session | Signed-cookie-backed anonymous identity with one conversation and a finite allowance. |
| User | A person with a verified external identity and persistent Glazz account. |
| Administrator | A user authorized to manage runtime policy, models, roles, and operational views. |
| Auth session | Revocable device session containing a rotating refresh-token family. |
| Conversation | Ordered chat container owned by exactly one guest session or user. |
| Message | Durable user or assistant content within a conversation. |
| Generation | One attempt to produce an assistant message from an acknowledged user message. |
| Model | Stable Glazz model identity and user-visible capabilities. |
| Provider | Internal adapter-backed upstream inference service. It is never selected directly by clients. |
| Provider model | Mapping between an internal model and an upstream provider model ID. |
| Model exposure | Administrator policy that makes a supported model visible to an audience. |
| Quota | Configured maximum messages, output tokens, concurrency, or global spend in a time window. |
| Usage | Reserved or actual consumption attributed to an actor and generation. |
| Runtime setting | Typed, versioned administrator-controlled product policy. |
| WebSocket ticket | Single-use short-lived credential used only to establish a realtime connection. |
| Idempotency key | Client-generated opaque value that makes replay of a mutation return the original outcome. |
| Sequence | Monotonic server event number used for ordering and bounded replay. |
| Delta offset | Character/byte position used to deduplicate streamed assistant text. |
| Conversation summary | Internal versioned condensation covering a contiguous range of prior messages. |
| Deletion job | Durable asynchronous request to purge an account and its personal data. |
| Audit event | Immutable redacted record of an administrative or security-sensitive action. |
| Maintenance mode | Runtime state that blocks generation while preserving authentication and administration access. |

## State vocabulary

Conversation status: `active`, `archived`.

Message status: `pending`, `complete`, `cancelled`, `failed`.

Generation status: `accepted`, `streaming`, `completed`, `cancelled`, `failed`,
`rejected`.

Deletion status: `pending`, `processing`, `completed`, `failed`.

Session status is derived from expiry, revocation, and refresh-token reuse; it is
not client-controlled.

