-- +goose Up
ALTER TABLE users ADD COLUMN username_changed_at TIMESTAMPTZ;

CREATE TABLE username_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    old_username TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_username_history_user ON username_history(user_id, changed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS username_history;
ALTER TABLE users DROP COLUMN IF EXISTS username_changed_at;