-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION enforce_journal_line_fx_integrity()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  journal_currency char(3);
  journal_date_value date;
  rate_from char(3);
  rate_to char(3);
  stored_rate numeric(28,12);
  stored_rate_date date;
BEGIN
  SELECT functional_currency_code, journal_date
    INTO journal_currency, journal_date_value
  FROM journal_entries
  WHERE id = NEW.journal_entry_id
    AND entity_id = NEW.entity_id;

  IF journal_currency IS NULL THEN
    RAISE EXCEPTION 'journal line must reference a journal in the same entity';
  END IF;

  IF NEW.functional_currency_code <> journal_currency THEN
    RAISE EXCEPTION 'journal line functional currency must match journal functional currency';
  END IF;

  IF NEW.transaction_currency_code = NEW.functional_currency_code THEN
    IF NEW.fx_rate_to_functional <> 1 THEN
      RAISE EXCEPTION 'same-currency journal line must use FX rate 1';
    END IF;
    IF NEW.exchange_rate_id IS NOT NULL THEN
      RAISE EXCEPTION 'same-currency journal line must not reference an exchange-rate snapshot';
    END IF;
  ELSE
    IF NEW.exchange_rate_id IS NULL THEN
      RAISE EXCEPTION 'foreign-currency journal line requires an exchange-rate snapshot';
    END IF;

    SELECT from_currency_code, to_currency_code, rate, rate_date
      INTO rate_from, rate_to, stored_rate, stored_rate_date
    FROM exchange_rates
    WHERE id = NEW.exchange_rate_id
      AND entity_id = NEW.entity_id;

    IF rate_from IS NULL THEN
      RAISE EXCEPTION 'exchange-rate snapshot not found for journal-line entity';
    END IF;

    IF rate_from <> NEW.transaction_currency_code
       OR rate_to <> NEW.functional_currency_code THEN
      RAISE EXCEPTION 'exchange-rate currency pair does not match journal line';
    END IF;

    IF stored_rate <> NEW.fx_rate_to_functional THEN
      RAISE EXCEPTION 'journal line FX rate must equal stored exchange-rate snapshot';
    END IF;

    IF stored_rate_date > journal_date_value THEN
      RAISE EXCEPTION 'journal line cannot use an exchange-rate snapshot from the future';
    END IF;
  END IF;

  IF round(NEW.transaction_debit_amount * NEW.fx_rate_to_functional, 6) <> NEW.debit_amount
     OR round(NEW.transaction_credit_amount * NEW.fx_rate_to_functional, 6) <> NEW.credit_amount THEN
    RAISE EXCEPTION 'journal line functional amounts do not match transaction amounts and FX rate';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_journal_line_fx_integrity
BEFORE INSERT OR UPDATE ON journal_lines
FOR EACH ROW
EXECUTE FUNCTION enforce_journal_line_fx_integrity();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_journal_line_fx_integrity ON journal_lines;
DROP FUNCTION IF EXISTS enforce_journal_line_fx_integrity();

-- +goose StatementEnd
