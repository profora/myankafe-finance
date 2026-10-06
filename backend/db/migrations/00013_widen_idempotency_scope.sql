-- +goose Up
-- Entity-scoped mutation URLs include two ULIDs. The previous varchar(120)
-- scope (user ULID + method + request URI) overflowed and returned HTTP 500
-- for cancellation, reversal, and inter-entity mapping.
ALTER TABLE idempotency_records
  ALTER COLUMN scope TYPE varchar(300);

-- +goose Down
ALTER TABLE idempotency_records
  ALTER COLUMN scope TYPE varchar(120);
