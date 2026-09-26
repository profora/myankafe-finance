-- +goose Up
-- +goose StatementBegin

ALTER TABLE currencies
  ADD COLUMN symbol varchar(8) NOT NULL DEFAULT '';

UPDATE currencies
SET symbol = CASE code
  WHEN 'MMK' THEN 'K'
  WHEN 'USD' THEN '$'
  WHEN 'SGD' THEN 'S$'
  WHEN 'THB' THEN '฿'
  ELSE symbol
END
WHERE symbol = '';

CREATE OR REPLACE FUNCTION finance_random_ulid() RETURNS varchar(26)
LANGUAGE plpgsql AS $$
DECLARE
  alphabet text := '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
  result text := '';
  i int;
BEGIN
  FOR i IN 1..26 LOOP
    result := result || substr(alphabet, 1 + floor(random() * 32)::int, 1);
  END LOOP;
  RETURN result;
END;
$$;

CREATE TABLE contact_types (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id) ON DELETE RESTRICT,
  code varchar(40) NOT NULL,
  name varchar(120) NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (entity_id, code)
);

INSERT INTO contact_types (id, public_id, entity_id, code, name, active)
SELECT gen_random_uuid(), finance_random_ulid(), e.id, t.code, t.name, true
FROM entities e
CROSS JOIN (VALUES
  ('CUSTOMER', 'Customer'),
  ('SUPPLIER', 'Supplier'),
  ('EMPLOYEE', 'Employee'),
  ('OWNER', 'Owner'),
  ('OTHER', 'Other')
) AS t(code, name)
ON CONFLICT (entity_id, code) DO NOTHING;

DO $$
DECLARE
  constraint_name text;
BEGIN
  FOR constraint_name IN
    SELECT con.conname
    FROM pg_constraint con
    WHERE con.conrelid = 'contacts'::regclass
      AND con.contype = 'c'
      AND pg_get_constraintdef(con.oid) ILIKE '%contact_type%'
  LOOP
    EXECUTE format('ALTER TABLE contacts DROP CONSTRAINT %I', constraint_name);
  END LOOP;
END $$;

ALTER TABLE contacts
  ALTER COLUMN contact_type TYPE varchar(40);

ALTER TABLE contacts
  ADD CONSTRAINT contacts_entity_contact_type_fk
  FOREIGN KEY (entity_id, contact_type)
  REFERENCES contact_types (entity_id, code);

CREATE OR REPLACE FUNCTION seed_default_contact_types()
RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO contact_types (id, public_id, entity_id, code, name, active)
  VALUES
    (gen_random_uuid(), finance_random_ulid(), NEW.id, 'CUSTOMER', 'Customer', true),
    (gen_random_uuid(), finance_random_ulid(), NEW.id, 'SUPPLIER', 'Supplier', true),
    (gen_random_uuid(), finance_random_ulid(), NEW.id, 'EMPLOYEE', 'Employee', true),
    (gen_random_uuid(), finance_random_ulid(), NEW.id, 'OWNER', 'Owner', true),
    (gen_random_uuid(), finance_random_ulid(), NEW.id, 'OTHER', 'Other', true)
  ON CONFLICT (entity_id, code) DO NOTHING;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_seed_default_contact_types
AFTER INSERT ON entities
FOR EACH ROW
EXECUTE FUNCTION seed_default_contact_types();

ALTER TABLE account_transfer_details
  ADD COLUMN fee_amount numeric(24,6),
  ADD COLUMN fee_currency_code char(3) REFERENCES currencies(code),
  ADD COLUMN fee_account_id uuid REFERENCES accounts(id),
  ADD CONSTRAINT account_transfer_fee_complete CHECK (
    (fee_amount IS NULL AND fee_currency_code IS NULL AND fee_account_id IS NULL)
    OR (fee_amount > 0 AND fee_currency_code IS NOT NULL AND fee_account_id IS NOT NULL)
  );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE account_transfer_details
  DROP CONSTRAINT IF EXISTS account_transfer_fee_complete,
  DROP COLUMN IF EXISTS fee_account_id,
  DROP COLUMN IF EXISTS fee_currency_code,
  DROP COLUMN IF EXISTS fee_amount;

DROP TRIGGER IF EXISTS trg_seed_default_contact_types ON entities;
DROP FUNCTION IF EXISTS seed_default_contact_types();

ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_entity_contact_type_fk;

UPDATE contacts
SET contact_type = 'OTHER'
WHERE contact_type NOT IN ('CUSTOMER', 'SUPPLIER', 'EMPLOYEE', 'OWNER', 'OTHER');

ALTER TABLE contacts
  ALTER COLUMN contact_type TYPE varchar(30);

ALTER TABLE contacts
  ADD CONSTRAINT contacts_contact_type_check
  CHECK (contact_type IN ('CUSTOMER', 'SUPPLIER', 'EMPLOYEE', 'OWNER', 'OTHER'));

DROP TABLE IF EXISTS contact_types;
DROP FUNCTION IF EXISTS finance_random_ulid();

ALTER TABLE currencies DROP COLUMN IF EXISTS symbol;

-- +goose StatementEnd
