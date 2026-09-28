-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION enforce_transaction_draft_period_lock()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  locked_through date;
  must_check boolean := false;
BEGIN
  IF TG_OP = 'INSERT' THEN
    must_check := true;
  ELSIF NEW.transaction_date IS DISTINCT FROM OLD.transaction_date THEN
    must_check := true;
  ELSIF OLD.status = 'DRAFT' AND NEW.status = 'VOIDED' THEN
    must_check := true;
  END IF;

  IF must_check THEN
    SELECT transactions_locked_through_date
      INTO locked_through
    FROM entity_accounting_controls
    WHERE entity_id = NEW.entity_id;

    IF locked_through IS NOT NULL AND NEW.transaction_date <= locked_through THEN
      RAISE EXCEPTION 'accounting period is locked through %', locked_through;
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_enforce_transaction_draft_period_lock
BEFORE INSERT OR UPDATE OF transaction_date,status ON transactions
FOR EACH ROW
EXECUTE FUNCTION enforce_transaction_draft_period_lock();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_enforce_transaction_draft_period_lock ON transactions;
DROP FUNCTION IF EXISTS enforce_transaction_draft_period_lock();

-- +goose StatementEnd
