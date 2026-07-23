-- name: FindUserByGoogleSubject :one
SELECT sqlc.embed(users)
FROM users
JOIN user_identities ON user_identities.user_id = users.id
WHERE user_identities.provider = 'google'
  AND user_identities.provider_subject = $1;

-- name: FindUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: FindUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (id, email, display_name, avatar_url, locale, role)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateGoogleIdentity :exec
INSERT INTO user_identities (id, user_id, provider, provider_subject, verified_email)
VALUES ($1, $2, 'google', $3, $4);

-- name: RecordTermsAcceptance :exec
INSERT INTO terms_acceptances (user_id, terms_version, privacy_version, ip_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
    id, user_id, family_id, device_label, refresh_expires_at, recent_auth_at
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: InsertRefreshToken :exec
INSERT INTO auth_refresh_tokens (token_hash, session_id, expires_at)
VALUES ($1, $2, $3);

-- name: LockRefreshToken :one
SELECT token.*, session.user_id, session.family_id, session.token_version,
       session.revoked_at, session.refresh_expires_at
FROM auth_refresh_tokens AS token
JOIN auth_sessions AS session ON session.id = token.session_id
WHERE token.token_hash = $1
FOR UPDATE OF token, session;

-- name: MarkRefreshTokenUsed :execrows
UPDATE auth_refresh_tokens
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL;

-- name: RevokeSessionFamily :exec
UPDATE auth_sessions
SET revoked_at = COALESCE(revoked_at, $2),
    reuse_detected_at = COALESCE(reuse_detected_at, $2),
    token_version = token_version + 1
WHERE family_id = $1;

-- name: TouchAuthSession :exec
UPDATE auth_sessions SET last_seen_at = $2 WHERE id = $1;

-- name: GetAuthSession :one
SELECT * FROM auth_sessions WHERE id = $1;

-- name: ListAuthSessions :many
SELECT * FROM auth_sessions
WHERE user_id = $1 AND revoked_at IS NULL AND refresh_expires_at > $2
ORDER BY created_at DESC, id DESC;

-- name: RevokeAuthSession :execrows
UPDATE auth_sessions
SET revoked_at = COALESCE(revoked_at, $3), token_version = token_version + 1
WHERE id = $1 AND user_id = $2;

-- name: PromoteBootstrapAdmin :execrows
UPDATE users SET role = 'admin', updated_at = $2
WHERE id = $1 AND role <> 'admin';

-- name: InsertAdminAudit :exec
INSERT INTO admin_audit_log (
    id, actor_user_id, action, target_type, target_id, before_value, after_value, request_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
