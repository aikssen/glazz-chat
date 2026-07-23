# ADR 0001: HTTP Router and WebSocket Library

- Status: Accepted for M0
- Date: 2026-07-23

## Context

The Go API needs feature-local route composition, standard middleware, strict
OpenAPI handler integration, WebSocket origin controls, cancellation, read limits,
bounded writes, and predictable shutdown.

Context7 research used `/go-chi/docs` and `/coder/websocket` on 2026-07-23.

## Decision

Use:

- `github.com/go-chi/chi/v5` for HTTP routing.
- `github.com/coder/websocket` for WebSocket transport.
- Standard `net/http` middleware contracts throughout.

Each vertical slice exposes a route-registration function. Generated strict OpenAPI
handlers adapt to application services. WebSocket connections use an explicit
allowed-origin list, configured read limit, one bounded application writer queue,
context cancellation, heartbeat, and graceful close.

## Consequences

- Route and middleware code stays compatible with `net/http`.
- `oapi-codegen` can generate chi server bindings.
- Backpressure remains an application responsibility; slow connections are closed
  when their bounded queue fills.
- Cross-instance replay/pub-sub is outside the WebSocket library and remains a
  Glazz platform concern.

## Alternatives

- Go 1.22+ `http.ServeMux`: viable, but chi route groups/middleware composition and
  generated integration reduce glue for vertical slices.
- Gin/Echo/Fiber: rejected due to broader framework surface and non-stdlib coupling.
- Gorilla WebSocket: mature but less context-first; coder/websocket better matches
  cancellation and minimal-dependency goals.
- Handwritten WebSocket framing: rejected as unnecessary protocol/security risk.

## Verification

- Unit tests for origin, ticket, limits, and close reasons.
- Race tests for concurrent read/write/cancel.
- Load tests for bounded queues, slow clients, and reconnect storms.

