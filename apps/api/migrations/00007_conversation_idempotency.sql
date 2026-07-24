-- +goose Up
ALTER TABLE conversations
    ADD COLUMN creation_idempotency_key text CHECK (
        creation_idempotency_key IS NULL
        OR length(creation_idempotency_key) BETWEEN 16 AND 128
    ),
    ADD COLUMN deletion_idempotency_key text CHECK (
        deletion_idempotency_key IS NULL
        OR length(deletion_idempotency_key) BETWEEN 16 AND 128
    );

CREATE UNIQUE INDEX conversations_user_creation_idempotency_idx
    ON conversations (user_id, creation_idempotency_key)
    WHERE user_id IS NOT NULL AND creation_idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX conversations_guest_creation_idempotency_idx
    ON conversations (guest_session_id, creation_idempotency_key)
    WHERE guest_session_id IS NOT NULL AND creation_idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX conversations_guest_creation_idempotency_idx;
DROP INDEX conversations_user_creation_idempotency_idx;

ALTER TABLE conversations
    DROP COLUMN deletion_idempotency_key,
    DROP COLUMN creation_idempotency_key;
