-- +goose Up
ALTER TABLE users
    ADD COLUMN consent_at        TIMESTAMPTZ,
    ADD COLUMN consent_version   TEXT,
    ADD COLUMN age_confirmed_at  TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users
    DROP COLUMN IF EXISTS age_confirmed_at,
    DROP COLUMN IF EXISTS consent_version,
    DROP COLUMN IF EXISTS consent_at;