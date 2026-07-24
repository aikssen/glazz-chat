-- +goose Up
INSERT INTO runtime_settings (key, value_kind, value)
VALUES ('quota.global.output_tokens', 'integer', '10000000'::jsonb);

ALTER TABLE models
    DROP CONSTRAINT models_check2,
    ADD CONSTRAINT models_enabled_exposure_check CHECK (
        NOT enabled OR (supported AND cardinality(audience) > 0)
    );

-- +goose Down
DELETE FROM runtime_settings WHERE key = 'quota.global.output_tokens';

UPDATE models
SET enabled = false, default_for = '{}'::text[], version = version + 1, updated_at = now()
WHERE enabled AND NOT available;

ALTER TABLE models
    DROP CONSTRAINT models_enabled_exposure_check,
    ADD CONSTRAINT models_check2 CHECK (
        NOT enabled OR (available AND supported AND cardinality(audience) > 0)
    );
