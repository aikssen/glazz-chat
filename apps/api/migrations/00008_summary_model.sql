-- +goose Up
INSERT INTO runtime_settings (key, value_kind, value)
VALUES (
    'chat.summary_model_id',
    'string',
    to_jsonb('00000000-0000-7000-8000-000000000101'::text)
);

-- +goose Down
DELETE FROM runtime_settings WHERE key = 'chat.summary_model_id';
