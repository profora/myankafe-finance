-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION enforce_transaction_status_transition()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status IS NOT DISTINCT FROM OLD.status THEN
    RETURN NEW;
  END IF;

  IF OLD.status = 'DRAFT' AND NEW.status IN ('POSTED','VOIDED') THEN
    RETURN NEW;
  END IF;

  IF OLD.status = 'POSTED' AND NEW.status = 'VOIDED' THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'invalid transaction status transition: % -> %', OLD.status, NEW.status;
END;
$$;

CREATE TRIGGER trg_transaction_status_transition
BEFORE UPDATE OF status ON transactions
FOR EACH ROW
EXECUTE FUNCTION enforce_transaction_status_transition();

CREATE OR REPLACE FUNCTION enforce_journal_status_transition()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.status IS NOT DISTINCT FROM OLD.status THEN
    RETURN NEW;
  END IF;

  IF OLD.status = 'DRAFT' AND NEW.status = 'POSTED' THEN
    RETURN NEW;
  END IF;

  IF OLD.status = 'POSTED' AND NEW.status = 'REVERSED' THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'invalid journal status transition: % -> %', OLD.status, NEW.status;
END;
$$;

CREATE TRIGGER trg_journal_status_transition
BEFORE UPDATE OF status ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION enforce_journal_status_transition();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_journal_status_transition ON journal_entries;
DROP FUNCTION IF EXISTS enforce_journal_status_transition();

DROP TRIGGER IF EXISTS trg_transaction_status_transition ON transactions;
DROP FUNCTION IF EXISTS enforce_transaction_status_transition();

-- +goose StatementEnd
