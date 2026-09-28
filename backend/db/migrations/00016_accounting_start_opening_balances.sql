-- +goose Up
ALTER TABLE entities
  ADD COLUMN accounting_start_date date NULL;

ALTER TABLE transactions DROP CONSTRAINT transactions_transaction_type_check;
ALTER TABLE transactions
  ADD CONSTRAINT transactions_transaction_type_check
  CHECK (transaction_type IN (
    'INCOME',
    'EXPENSE',
    'ACCOUNT_TRANSFER',
    'INTER_ENTITY',
    'MANUAL_JOURNAL',
    'ADJUSTMENT',
    'REVERSAL',
    'OPENING_BALANCE',
    'OPENING_BALANCE_ADJUSTMENT'
  ));

ALTER TABLE accounts
  ADD CONSTRAINT accounts_code_four_digits CHECK (code ~ '^[0-9]{4}$');

CREATE UNIQUE INDEX uq_accounts_system_role
  ON accounts (entity_id, system_role)
  WHERE system_role IS NOT NULL;

ALTER TABLE financial_accounts
  ADD CONSTRAINT uq_financial_accounts_id_entity_account UNIQUE (id, entity_id, account_id);

CREATE TABLE opening_balance_sets (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL UNIQUE REFERENCES entities(id),
  accounting_start_date date NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (id, entity_id)
);

CREATE TABLE opening_balance_lines (
  id uuid PRIMARY KEY,
  opening_balance_set_id uuid NOT NULL,
  entity_id uuid NOT NULL REFERENCES entities(id),
  account_id uuid NOT NULL,
  financial_account_id uuid,
  currency_code char(3) NOT NULL REFERENCES currencies(code),
  direction varchar(10) NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
  amount numeric(24,6) NOT NULL CHECK (amount > 0),
  fx_rate_to_functional numeric(28,12) NOT NULL CHECK (fx_rate_to_functional > 0),
  exchange_rate_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (opening_balance_set_id, entity_id) REFERENCES opening_balance_sets (id, entity_id),
  FOREIGN KEY (account_id, entity_id) REFERENCES accounts (id, entity_id),
  FOREIGN KEY (financial_account_id, entity_id, account_id) REFERENCES financial_accounts (id, entity_id, account_id),
  FOREIGN KEY (exchange_rate_id, entity_id) REFERENCES exchange_rates (id, entity_id)
);

CREATE UNIQUE INDEX uq_opening_balance_account_line
  ON opening_balance_lines (opening_balance_set_id, account_id)
  WHERE financial_account_id IS NULL;

CREATE UNIQUE INDEX uq_opening_balance_financial_line
  ON opening_balance_lines (opening_balance_set_id, financial_account_id)
  WHERE financial_account_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS opening_balance_lines;
DROP TABLE IF EXISTS opening_balance_sets;
ALTER TABLE financial_accounts DROP CONSTRAINT IF EXISTS uq_financial_accounts_id_entity_account;
DROP INDEX IF EXISTS uq_accounts_system_role;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_code_four_digits;
ALTER TABLE entities DROP COLUMN IF EXISTS accounting_start_date;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_transaction_type_check;
ALTER TABLE transactions
  ADD CONSTRAINT transactions_transaction_type_check
  CHECK (transaction_type IN (
    'INCOME',
    'EXPENSE',
    'ACCOUNT_TRANSFER',
    'INTER_ENTITY',
    'MANUAL_JOURNAL',
    'ADJUSTMENT',
    'REVERSAL'
  ));
