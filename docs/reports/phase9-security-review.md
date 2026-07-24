# Phase 9 Application Security Review

- Date: 2026-07-24
- Task: SEC-002
- Result: Accepted with medium production hardening items assigned

## Scope

The review followed the threat cases in `docs/threat-model/m0-threat-model.md` and
covered OAuth state/PKCE/nonce, CSRF, Ed25519 JWT validation, refresh rotation,
WebSocket origin and tickets, ownership/IDOR, administrator authorization, guest
limits, Markdown XSS, CSP, secrets, dependency vulnerabilities, and resource
bounds.

Automated evidence:

- `pnpm audit --prod --audit-level high`: no known vulnerabilities;
- `govulncheck ./...`: no reachable Go vulnerabilities;
- Gitleaks Git-history scan: 19 commits and 3.35 MB, no leaks;
- Gitleaks sanitized source scan: 3.45 MB, no leaks;
- security packages under `go test -race`: pass;
- Playwright browser-header and service-worker cache tests: pass;
- malicious Markdown/IME browser case: inert and pass.

The sanitized source scan excludes ignored runtime material such as `.env`,
`.next`, dependency directories, and test output. A raw working-directory scan
correctly detects local environment credentials, which confirms that `.env` must
remain ignored and must never be copied into artifacts.

## Findings

| ID    | Severity | Finding                                                                                                                               | Resolution / owner                                                                                                                                         |
| ----- | -------- | ------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| S9-01 | High     | Next.js responses lacked CSP and browser security headers and exposed `X-Powered-By`                                                  | Fixed. Production build now sets CSP, COOP, CORP, referrer, permissions, nosniff, frame denial, and disables the powered-by header. Playwright gate added. |
| S9-02 | Medium   | Development/LAN `connect-src` permits HTTP, HTTPS, WS, and WSS destinations                                                           | Accepted only for current configurable local topology. PROD-004 must reduce this to selected production HTTPS/WSS origins.                                 |
| S9-03 | Medium   | CSP permits inline styles/scripts because Next.js and the pre-hydration theme bootstrap require them in the current static deployment | PROD-004 owns evaluation of a nonce-based CSP at the selected hosting boundary.                                                                            |
| S9-04 | Medium   | The metrics handler is mounted on the API router and must not be internet-accessible in production                                    | PROD-003/PROD-007 must place metrics behind private ingress or authenticated observability access.                                                         |

There are no open critical or high findings. The medium items do not weaken the
local release-candidate boundary and are explicit production infrastructure
requirements.

## Control conclusions

- OAuth state is single-use and binds PKCE, nonce, return path, consent, and guest
  migration context.
- Access tokens validate signature, issuer, audience, expiry, and active key.
- Refresh reuse revokes the token family; sensitive operations require recent auth.
- Mutations require actor-aware CSRF; CORS and WebSocket origins are allowlisted.
- WebSocket tickets are short-lived, actor-bound, and single-use.
- Conversation reads and writes return not-found across ownership boundaries.
- Administrator endpoints require the role and never expose conversation content.
- Guest quota and rate controls fail closed when Redis is unavailable.
- Logs record bounded route/error metadata, not prompt, response, credential, or
  raw identity content.
