-- +goose Up
-- +goose StatementBegin

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_transaction_type_check;
ALTER TABLE transactions
  ADD CONSTRAINT transactions_transaction_type_check
  CHECK (transaction_type IN (
    'INCOME','EXPENSE','ACCOUNT_TRANSFER','INTER_ENTITY','MANUAL_JOURNAL','ADJUSTMENT','REVERSAL','INTEGRATION'
  ));

ALTER TABLE transactions ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_user_source_requires_user;
ALTER TABLE transactions
  ADD CONSTRAINT transactions_user_source_requires_user
  CHECK (source_type <> 'USER' OR created_by IS NOT NULL);

ALTER TABLE journal_entries ALTER COLUMN created_by DROP NOT NULL;

CREATE OR REPLACE FUNCTION enforce_integration_journal_actor()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.created_by IS NULL THEN
    IF NEW.transaction_id IS NULL OR NOT EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.id = NEW.transaction_id
        AND t.entity_id = NEW.entity_id
        AND t.source_type = 'INTEGRATION'
    ) THEN
      RAISE EXCEPTION 'a journal without a user requires an integration transaction';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_enforce_integration_journal_actor ON journal_entries;
CREATE TRIGGER trg_enforce_integration_journal_actor
BEFORE INSERT OR UPDATE OF created_by, transaction_id ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION enforce_integration_journal_actor();

CREATE TABLE integration_mappings (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  integration_connection_id uuid NOT NULL REFERENCES integration_connections(id),
  mapping_kind varchar(40) NOT NULL CHECK (mapping_kind IN ('ACCOUNT','FINANCIAL_ACCOUNT','SALES_CHANNEL')),
  source_key varchar(160) NOT NULL,
  account_id uuid REFERENCES accounts(id),
  financial_account_id uuid REFERENCES financial_accounts(id),
  sales_channel_id uuid REFERENCES sales_channels(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (integration_connection_id, source_key),
  CHECK (
    (mapping_kind = 'ACCOUNT' AND account_id IS NOT NULL AND financial_account_id IS NULL AND sales_channel_id IS NULL)
    OR (mapping_kind = 'FINANCIAL_ACCOUNT' AND financial_account_id IS NOT NULL AND account_id IS NULL AND sales_channel_id IS NULL)
    OR (mapping_kind = 'SALES_CHANNEL' AND sales_channel_id IS NOT NULL AND account_id IS NULL AND financial_account_id IS NULL)
  )
);

CREATE INDEX idx_integration_mappings_connection ON integration_mappings(integration_connection_id, source_key);

ALTER TABLE integration_events
  ADD COLUMN payload jsonb,
  ADD COLUMN payload_version integer NOT NULL DEFAULT 1,
  ADD COLUMN accounting_date date;

CREATE OR REPLACE FUNCTION enforce_posted_integration_event_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status = 'POSTED' THEN
    IF NEW.payload IS DISTINCT FROM OLD.payload
       OR NEW.payload_hash IS DISTINCT FROM OLD.payload_hash
       OR NEW.payload_version IS DISTINCT FROM OLD.payload_version
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.external_event_id IS DISTINCT FROM OLD.external_event_id
       OR NEW.transaction_id IS DISTINCT FROM OLD.transaction_id
       OR NEW.accounting_date IS DISTINCT FROM OLD.accounting_date
    THEN
      RAISE EXCEPTION 'posted integration event is immutable';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_posted_integration_event_immutable ON integration_events;
CREATE TRIGGER trg_posted_integration_event_immutable
BEFORE UPDATE ON integration_events
FOR EACH ROW
EXECUTE FUNCTION enforce_posted_integration_event_immutable();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_posted_integration_event_immutable ON integration_events;
DROP FUNCTION IF EXISTS enforce_posted_integration_event_immutable();
ALTER TABLE integration_events DROP COLUMN IF EXISTS accounting_date;
ALTER TABLE integration_events DROP COLUMN IF EXISTS payload_version;
ALTER TABLE integration_events DROP COLUMN IF EXISTS payload;
DROP TABLE IF EXISTS integration_mappings;
DROP TRIGGER IF EXISTS trg_enforce_integration_journal_actor ON journal_entries;
DROP FUNCTION IF EXISTS enforce_integration_journal_actor();

-- +goose StatementEnd
