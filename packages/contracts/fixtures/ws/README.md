# WebSocket Contract Fixtures

Each file is an ordered protocol transcript. Every array item must match exactly one
message payload under `channels.realtime.messages` in
`../../websocket.asyncapi.yaml`.

| Fixture | Required behavior |
| --- | --- |
| `guest-success.json` | Guest generation acknowledgement, ordered delta, terminal usage |
| `user-success.json` | Registered-user generation terminal flow |
| `cancel.json` | Idempotent cancellation with retained partial content |
| `quota-rejected.json` | Stable quota rejection followed by authoritative quota update |
| `reconnect.json` | Resume request outside replay window and REST resync instruction |
| `provider-failure.json` | Retryable normalized failure after partial output |

Phase 1 wires schema validation into CI using the exact toolchain in ADR 0002.
Until generated validators exist, M0 validation consists of:

1. Redocly validation of AsyncAPI and its component schemas.
2. JSON syntax validation for every fixture.
3. M0 baseline review of event discriminator, envelope direction, required fields,
   IDs, sequence, delta offset, and terminal semantics.

Adding or changing an event requires updating AsyncAPI, at least one fixture, and
the baseline review in the same change.

