-- +goose Up
-- +goose StatementBegin

ALTER TABLE journal_entries
  ADD CONSTRAINT uq_journal_entries_id_entity UNIQUE(id, entity_id),
  ADD CONSTRAINT fk_journal_entity_currency
    FOREIGN KEY(entity_id, functional_currency_code)
    REFERENCES entities(id, functional_currency_code);

ALTER TABLE exchange_rates
  ADD CONSTRAINT uq_exchange_rates_id_entity UNIQUE(id, entity_id);

ALTER TABLE journal_lines
  ADD CONSTRAINT fk_journal_line_journal_entity
    FOREIGN KEY(journal_entry_id, entity_id)
    REFERENCES journal_entries(id, entity_id),
  ADD CONSTRAINT fk_journal_line_account_entity
    FOREIGN KEY(account_id, entity_id)
    REFERENCES accounts(id, entity_id),
  ADD CONSTRAINT fk_journal_line_financial_account_entity
    FOREIGN KEY(financial_account_id, entity_id)
    REFERENCES financial_accounts(id, entity_id),
  ADD CONSTRAINT fk_journal_line_contact_entity
    FOREIGN KEY(contact_id, entity_id)
    REFERENCES contacts(id, entity_id),
  ADD CONSTRAINT fk_journal_line_exchange_rate_entity
    FOREIGN KEY(exchange_rate_id, entity_id)
    REFERENCES exchange_rates(id, entity_id);

ALTER TABLE journal_entries
  ADD CONSTRAINT fk_journal_transaction_entity
    FOREIGN KEY(transaction_id, entity_id)
    REFERENCES transactions(id, entity_id);

CREATE OR REPLACE FUNCTION enforce_posted_journal_balance()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  line_count integer;
  debit_total numeric(24,6);
  credit_total numeric(24,6);
BEGIN
  IF NEW.status = 'POSTED' AND OLD.status IS DISTINCT FROM 'POSTED' THEN
    SELECT count(*), COALESCE(sum(debit_amount),0), COALESCE(sum(credit_amount),0)
      INTO line_count, debit_total, credit_total
    FROM journal_lines
    WHERE journal_entry_id = NEW.id;

    IF line_count < 2 THEN
      RAISE EXCEPTION 'posted journal requires at least two lines';
    END IF;

    IF debit_total <> credit_total THEN
      RAISE EXCEPTION 'posted journal is unbalanced: debits %, credits %', debit_total, credit_total;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enforce_posted_journal_balance
BEFORE UPDATE OF status ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION enforce_posted_journal_balance();

CREATE OR REPLACE FUNCTION prevent_used_exchange_rate_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM journal_lines jl
    JOIN journal_entries je ON je.id = jl.journal_entry_id
    WHERE jl.exchange_rate_id = OLD.id
      AND je.status IN ('POSTED','REVERSED')
  ) THEN
    RAISE EXCEPTION 'exchange rate used by posted accounting is immutable';
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER trg_exchange_rate_immutable_when_used
BEFORE UPDATE OR DELETE ON exchange_rates
FOR EACH ROW
EXECUTE FUNCTION prevent_used_exchange_rate_mutation();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_exchange_rate_immutable_when_used ON exchange_rates;
DROP FUNCTION IF EXISTS prevent_used_exchange_rate_mutation();

DROP TRIGGER IF EXISTS trg_enforce_posted_journal_balance ON journal_entries;
DROP FUNCTION IF EXISTS enforce_posted_journal_balance();

ALTER TABLE journal_entries
  DROP CONSTRAINT IF EXISTS fk_journal_transaction_entity,
  DROP CONSTRAINT IF EXISTS fk_journal_entity_currency,
  DROP CONSTRAINT IF EXISTS uq_journal_entries_id_entity;

ALTER TABLE journal_lines
  DROP CONSTRAINT IF EXISTS fk_journal_line_exchange_rate_entity,
  DROP CONSTRAINT IF EXISTS fk_journal_line_contact_entity,
  DROP CONSTRAINT IF EXISTS fk_journal_line_financial_account_entity,
  DROP CONSTRAINT IF EXISTS fk_journal_line_account_entity,
  DROP CONSTRAINT IF EXISTS fk_journal_line_journal_entity;

ALTER TABLE exchange_rates
  DROP CONSTRAINT IF EXISTS uq_exchange_rates_id_entity;

-- +goose StatementEnd
