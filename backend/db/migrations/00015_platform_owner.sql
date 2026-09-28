-- +goose Up
ALTER TABLE users
  ADD COLUMN platform_owner boolean NOT NULL DEFAULT false;

UPDATE users AS u
SET platform_owner = true
WHERE EXISTS (
  SELECT 1
  FROM user_entity_roles AS uer
  JOIN roles AS r ON r.id = uer.role_id
  WHERE uer.user_id = u.id
    AND uer.revoked_at IS NULL
    AND r.code = 'OWNER'
);

-- +goose Down
ALTER TABLE users DROP COLUMN platform_owner;
