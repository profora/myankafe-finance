-- +goose Up
-- +goose StatementBegin

ALTER TABLE transaction_attachments
  ADD COLUMN display_order integer NOT NULL DEFAULT 0 CHECK(display_order >= 0),
  ADD COLUMN sha256_hex char(64),
  ADD COLUMN deleted_at timestamptz,
  ADD COLUMN deleted_by uuid REFERENCES users(id);

CREATE INDEX idx_transaction_attachments_transaction_order
  ON transaction_attachments(transaction_id,display_order,created_at)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_transaction_attachment_active_order
  ON transaction_attachments(transaction_id,display_order)
  WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION protect_attachment_identity_after_create()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.transaction_id IS DISTINCT FROM OLD.transaction_id
     OR NEW.storage_key IS DISTINCT FROM OLD.storage_key
     OR NEW.original_filename IS DISTINCT FROM OLD.original_filename
     OR NEW.mime_type IS DISTINCT FROM OLD.mime_type
     OR NEW.size_bytes IS DISTINCT FROM OLD.size_bytes
     OR NEW.sha256_hex IS DISTINCT FROM OLD.sha256_hex
     OR NEW.uploaded_by IS DISTINCT FROM OLD.uploaded_by
     OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'attachment identity and stored content metadata are immutable';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_protect_attachment_identity
BEFORE UPDATE ON transaction_attachments
FOR EACH ROW
EXECUTE FUNCTION protect_attachment_identity_after_create();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_protect_attachment_identity ON transaction_attachments;
DROP FUNCTION IF EXISTS protect_attachment_identity_after_create();

DROP INDEX IF EXISTS uq_transaction_attachment_active_order;
DROP INDEX IF EXISTS idx_transaction_attachments_transaction_order;

ALTER TABLE transaction_attachments
  DROP COLUMN IF EXISTS deleted_by,
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS sha256_hex,
  DROP COLUMN IF EXISTS display_order;

-- +goose StatementEnd
