-- +goose Up
ALTER TABLE users
    ADD COLUMN version integer NOT NULL DEFAULT 1 CHECK (version > 0);

CREATE TABLE account_deletion_jobs (
    id uuid PRIMARY KEY,
    user_id uuid UNIQUE REFERENCES users(id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    requested_at timestamptz NOT NULL,
    due_at timestamptz NOT NULL,
    started_at timestamptz,
    completed_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error_class text,
    CHECK (due_at <= requested_at + interval '24 hours'),
    CHECK (
        (status = 'completed' AND completed_at IS NOT NULL)
        OR (status <> 'completed' AND completed_at IS NULL)
    )
);
CREATE INDEX account_deletion_jobs_pending_idx
    ON account_deletion_jobs (due_at, requested_at)
    WHERE status IN ('pending', 'failed');

-- +goose Down
DROP TABLE account_deletion_jobs;
ALTER TABLE users DROP COLUMN version;
