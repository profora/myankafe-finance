-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION enforce_journal_period_lock()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  locked_through date;
BEGIN
  IF NEW.status = 'POSTED' THEN
    SELECT transactions_locked_through_date
      INTO locked_through
    FROM entity_accounting_controls
    WHERE entity_id = NEW.entity_id;

    IF locked_through IS NOT NULL AND NEW.journal_date <= locked_through THEN
      RAISE EXCEPTION 'accounting period is locked through %', locked_through;
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enforce_journal_period_lock
BEFORE INSERT OR UPDATE OF status ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION enforce_journal_period_lock();

CREATE OR REPLACE FUNCTION enforce_posted_transaction_has_journal()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status = 'POSTED' THEN
    IF NOT EXISTS (
      SELECT 1
      FROM journal_entries je
      WHERE je.transaction_id = NEW.id
        AND je.entity_id = NEW.entity_id
        AND je.status = 'POSTED'
    ) THEN
      RAISE EXCEPTION 'posted transaction requires a posted journal';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enforce_posted_transaction_has_journal
BEFORE INSERT OR UPDATE OF status ON transactions
FOR EACH ROW
EXECUTE FUNCTION enforce_posted_transaction_has_journal();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_enforce_posted_transaction_has_journal ON transactions;
DROP FUNCTION IF EXISTS enforce_posted_transaction_has_journal();

DROP TRIGGER IF EXISTS trg_enforce_journal_period_lock ON journal_entries;
DROP FUNCTION IF EXISTS enforce_journal_period_lock();

-- +goose StatementEnd
