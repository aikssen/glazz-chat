# ADR 0003: JWT Signing and Key Rotation

- Status: Accepted for M0
- Date: 2026-07-23

## Context

Access tokens are short-lived JWTs. Glazz needs asymmetric signing, explicit
algorithm restrictions, `kid` rotation, issuer/audience validation, emergency
revocation, deterministic tests, and a separate opaque refresh-token design.

Context7 research used `/golang-jwt/jwt` on 2026-07-23.

## Decision

- Use `github.com/golang-jwt/jwt/v5`.
- Sign access JWTs with Ed25519 (`alg=EdDSA`).
- Put a non-secret key identifier in `kid`.
- Require `iss`, `aud`, `sub`, `sid`, `iat`, `nbf`, `exp`, and token-version claims.
- Restrict parsing to `EdDSA`; enable strict decoding and required time claims.
- Allow at most 30 seconds clock leeway.
- Keep an active signing key and a bounded verification key ring.
- Access token lifetime is 15 minutes.
- Refresh tokens are opaque 256-bit random values, hashed at rest, rotated
  transactionally, and are not JWTs.

## Key lifecycle

- Local: generated development keys outside version control.
- CI: ephemeral keys injected as secrets.
- Production: secret-manager/KMS-backed private key material; public keys may be
  cached by `kid`.
- Rotation: publish new verification key, switch signer, retain old verification
  key for access-token lifetime plus leeway, then remove it.
- Emergency: disable compromised `kid`, increment affected session/token versions,
  and revoke refresh families.

## Consequences

- API replicas verify tokens without shared symmetric signing secrets.
- Immediate per-session revocation still checks session state for sensitive paths.
- Key management is explicit operational work and needs a runbook.

## Alternatives

- HS256: rejected because every verifier would possess signing capability.
- RS256: valid but larger/slower keys and signatures without a Glazz compatibility
  requirement.
- JWT refresh tokens: rejected because rotation/reuse and server-side revocation are
  clearer with opaque secrets.
- Custom JWT implementation: rejected as security-sensitive reinvention.

## Verification

Test algorithm confusion, missing/invalid claims, wrong audience/issuer, expiry,
future `nbf`, unknown/disabled `kid`, rotation overlap, and refresh-token races.

