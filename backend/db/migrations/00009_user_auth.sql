-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
  ADD CONSTRAINT users_username_normalized CHECK (username = lower(username)),
  ADD CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,63}$');

CREATE TABLE user_credentials (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  password_hash text NOT NULL,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  user_agent text,
  ip_address inet,
  expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT user_sessions_token_hash_sha256 CHECK (octet_length(token_hash) = 32),
  CONSTRAINT user_sessions_expiry_valid CHECK (expires_at > created_at),
  CONSTRAINT user_sessions_revoked_at_valid CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX idx_user_sessions_user_active
  ON user_sessions(user_id,expires_at DESC)
  WHERE revoked_at IS NULL;

CREATE INDEX idx_user_sessions_expiry
  ON user_sessions(expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS user_credentials;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_username_format,
  DROP CONSTRAINT IF EXISTS users_username_normalized;

-- +goose StatementEnd
