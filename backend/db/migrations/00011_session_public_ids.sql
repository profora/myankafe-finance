-- +goose Up
-- +goose StatementBegin

ALTER TABLE user_sessions
  ADD COLUMN public_id char(26);

ALTER TABLE user_sessions
  ADD CONSTRAINT user_sessions_public_id_format
  CHECK (public_id IS NULL OR public_id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$');

CREATE UNIQUE INDEX uq_user_sessions_public_id
  ON user_sessions(public_id)
  WHERE public_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS uq_user_sessions_public_id;

ALTER TABLE user_sessions
  DROP CONSTRAINT IF EXISTS user_sessions_public_id_format,
  DROP COLUMN IF EXISTS public_id;

-- +goose StatementEnd
