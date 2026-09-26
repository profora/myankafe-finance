-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION enforce_transaction_split_entity()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  transaction_entity uuid;
  account_entity uuid;
BEGIN
  SELECT entity_id INTO transaction_entity FROM transactions WHERE id = NEW.transaction_id;
  SELECT entity_id INTO account_entity FROM accounts WHERE id = NEW.account_id;

  IF transaction_entity IS NULL OR account_entity IS NULL OR transaction_entity <> account_entity THEN
    RAISE EXCEPTION 'transaction split account must belong to transaction entity';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_transaction_split_entity
BEFORE INSERT OR UPDATE ON transaction_splits
FOR EACH ROW
EXECUTE FUNCTION enforce_transaction_split_entity();

CREATE OR REPLACE FUNCTION enforce_inter_entity_mapping_accounts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  due_from_entity uuid;
  due_to_entity uuid;
  due_from_type varchar(20);
  due_to_type varchar(20);
BEGIN
  SELECT entity_id, account_type INTO due_from_entity, due_from_type
  FROM accounts WHERE id = NEW.due_from_account_id;

  SELECT entity_id, account_type INTO due_to_entity, due_to_type
  FROM accounts WHERE id = NEW.due_to_account_id;

  IF due_from_entity <> NEW.entity_id OR due_to_entity <> NEW.entity_id THEN
    RAISE EXCEPTION 'inter-entity mapping accounts must belong to mapping entity';
  END IF;

  IF due_from_type <> 'ASSET' OR due_to_type <> 'LIABILITY' THEN
    RAISE EXCEPTION 'inter-entity mapping requires ASSET due-from and LIABILITY due-to accounts';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_inter_entity_mapping_accounts
BEFORE INSERT OR UPDATE ON inter_entity_account_mappings
FOR EACH ROW
EXECUTE FUNCTION enforce_inter_entity_mapping_accounts();

CREATE OR REPLACE FUNCTION enforce_account_transfer_entity()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  transaction_entity uuid;
  from_entity uuid;
  to_entity uuid;
BEGIN
  SELECT entity_id INTO transaction_entity FROM transactions WHERE id = NEW.transaction_id;
  SELECT entity_id INTO from_entity FROM financial_accounts WHERE id = NEW.from_financial_account_id;
  SELECT entity_id INTO to_entity FROM financial_accounts WHERE id = NEW.to_financial_account_id;

  IF transaction_entity IS NULL OR from_entity IS NULL OR to_entity IS NULL
     OR transaction_entity <> from_entity OR transaction_entity <> to_entity THEN
    RAISE EXCEPTION 'account transfer financial accounts must belong to transaction entity';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_account_transfer_entity
BEFORE INSERT OR UPDATE ON account_transfer_details
FOR EACH ROW
EXECUTE FUNCTION enforce_account_transfer_entity();

CREATE OR REPLACE FUNCTION protect_posted_transaction_core()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status IN ('POSTED','VOIDED') THEN
    IF NEW.entity_id IS DISTINCT FROM OLD.entity_id
       OR NEW.transaction_type IS DISTINCT FROM OLD.transaction_type
       OR NEW.transaction_date IS DISTINCT FROM OLD.transaction_date
       OR NEW.description IS DISTINCT FROM OLD.description
       OR NEW.contact_id IS DISTINCT FROM OLD.contact_id
       OR NEW.primary_financial_account_id IS DISTINCT FROM OLD.primary_financial_account_id
       OR NEW.currency_code IS DISTINCT FROM OLD.currency_code
       OR NEW.total_amount IS DISTINCT FROM OLD.total_amount
       OR NEW.source_type IS DISTINCT FROM OLD.source_type
       OR NEW.source_system IS DISTINCT FROM OLD.source_system
       OR NEW.external_reference IS DISTINCT FROM OLD.external_reference
       OR NEW.original_transaction_id IS DISTINCT FROM OLD.original_transaction_id
       OR NEW.created_by IS DISTINCT FROM OLD.created_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
      RAISE EXCEPTION 'posted transaction accounting fields are immutable';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_protect_posted_transaction_core
BEFORE UPDATE ON transactions
FOR EACH ROW
EXECUTE FUNCTION protect_posted_transaction_core();

CREATE OR REPLACE FUNCTION protect_posted_journal_core()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF OLD.status IN ('POSTED','REVERSED') THEN
    IF NEW.entity_id IS DISTINCT FROM OLD.entity_id
       OR NEW.transaction_id IS DISTINCT FROM OLD.transaction_id
       OR NEW.journal_date IS DISTINCT FROM OLD.journal_date
       OR NEW.description IS DISTINCT FROM OLD.description
       OR NEW.functional_currency_code IS DISTINCT FROM OLD.functional_currency_code
       OR NEW.created_by IS DISTINCT FROM OLD.created_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.reversal_of_journal_id IS DISTINCT FROM OLD.reversal_of_journal_id THEN
      RAISE EXCEPTION 'posted journal accounting fields are immutable';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_protect_posted_journal_core
BEFORE UPDATE ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION protect_posted_journal_core();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_protect_posted_journal_core ON journal_entries;
DROP FUNCTION IF EXISTS protect_posted_journal_core();

DROP TRIGGER IF EXISTS trg_protect_posted_transaction_core ON transactions;
DROP FUNCTION IF EXISTS protect_posted_transaction_core();

DROP TRIGGER IF EXISTS trg_account_transfer_entity ON account_transfer_details;
DROP FUNCTION IF EXISTS enforce_account_transfer_entity();

DROP TRIGGER IF EXISTS trg_inter_entity_mapping_accounts ON inter_entity_account_mappings;
DROP FUNCTION IF EXISTS enforce_inter_entity_mapping_accounts();

DROP TRIGGER IF EXISTS trg_transaction_split_entity ON transaction_splits;
DROP FUNCTION IF EXISTS enforce_transaction_split_entity();

-- +goose StatementEnd
