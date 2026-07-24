# M1 Foundation Record

## Ownership

- **Milestone:** M1
- **Owned phase:** Phase 1
- **Release:** No standalone tag; this milestone predates the M2 tagging rule.

Date: 2026-07-23

## Outcome

M1 establishes a reproducible pnpm/Go monorepo with a Next.js App Router frontend,
Go API and worker composition roots, generated HTTP types, executable realtime
fixtures, quality commands, safe environment templates, and CI.

The accepted M0 baseline remains tagged `v0.1.0`. M1 does not change product
behavior or introduce a production LLM provider.

## Contract Corrections

Stricter validation with `@asyncapi/parser` found that the AsyncAPI channel grouped
messages through objects containing `oneOf`; AsyncAPI 3 requires each channel and
operation message to be a direct message reference. The channel now enumerates the
same concrete messages. Event names, payload schemas, and canonical fixtures did
not change.

Two valid JSON Schema boolean schemas in OpenAPI were replaced with equivalent
empty schemas because oapi-codegen 2.8.0 cannot decode boolean schemas in
`properties`. This preserves the runtime-setting value semantics while keeping
generation deterministic.

## Verification

- Redocly validates OpenAPI and AsyncAPI with zero errors.
- The AsyncAPI parser and AJV validate 14 events in six canonical fixtures.
- OpenAPI generation succeeds for TypeScript and Go and is drift-checked in CI.
- Vitest covers Tailwind class merging and WCAG AA contrast pairs in light/dark
  tokens.
- `go vet`, race-enabled Go tests, Go builds, ESLint, TypeScript, Vitest, the
  Next.js production build, and the Playwright Chromium smoke test pass.
- CI pins third-party actions to immutable commits and runs dependency and secret
  scans.
