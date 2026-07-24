-- +goose Up
ALTER TABLE guest_sessions
    DROP CONSTRAINT guest_sessions_prompt_count_check,
    DROP CONSTRAINT guest_sessions_output_token_count_check,
    ADD CONSTRAINT guest_sessions_prompt_count_check
        CHECK (prompt_count BETWEEN 0 AND 20),
    ADD CONSTRAINT guest_sessions_output_token_count_check
        CHECK (output_token_count BETWEEN 0 AND 10000);

DROP INDEX account_deletion_jobs_pending_idx;
CREATE INDEX account_deletion_jobs_pending_idx
    ON account_deletion_jobs (due_at, requested_at)
    WHERE status IN ('pending', 'failed', 'processing');

-- +goose Down
DROP INDEX account_deletion_jobs_pending_idx;
CREATE INDEX account_deletion_jobs_pending_idx
    ON account_deletion_jobs (due_at, requested_at)
    WHERE status IN ('pending', 'failed');

ALTER TABLE guest_sessions
    DROP CONSTRAINT guest_sessions_prompt_count_check,
    DROP CONSTRAINT guest_sessions_output_token_count_check,
    ADD CONSTRAINT guest_sessions_prompt_count_check
        CHECK (prompt_count BETWEEN 0 AND 4),
    ADD CONSTRAINT guest_sessions_output_token_count_check
        CHECK (output_token_count BETWEEN 0 AND 2000);
