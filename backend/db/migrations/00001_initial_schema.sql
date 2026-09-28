-- +goose Up
-- +goose StatementBegin
CREATE DOMAIN public_ulid AS varchar(26)
  CHECK (VALUE ~ '^[0-9A-HJKMNP-TV-Z]{26}$');

CREATE TABLE users (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  username varchar(120) NOT NULL UNIQUE,
  display_name varchar(200) NOT NULL,
  email varchar(320),
  status varchar(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE roles (
  id uuid PRIMARY KEY,
  code varchar(50) NOT NULL UNIQUE,
  name varchar(120) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
  id uuid PRIMARY KEY,
  code varchar(100) NOT NULL UNIQUE,
  description text
);

CREATE TABLE role_permissions (
  role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
  PRIMARY KEY(role_id,permission_id)
);

CREATE TABLE currencies (
  code char(3) PRIMARY KEY,
  name varchar(120) NOT NULL,
  decimal_places smallint NOT NULL CHECK(decimal_places BETWEEN 0 AND 6),
  active boolean NOT NULL DEFAULT true
);

CREATE TABLE entities (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  code varchar(80) NOT NULL UNIQUE,
  name varchar(200) NOT NULL,
  entity_type varchar(20) NOT NULL CHECK(entity_type IN ('BUSINESS','PERSONAL','OTHER')),
  functional_currency_code char(3) NOT NULL REFERENCES currencies(code),
  timezone varchar(100) NOT NULL,
  fiscal_year_start_month smallint NOT NULL DEFAULT 1 CHECK(fiscal_year_start_month BETWEEN 1 AND 12),
  fiscal_year_start_day smallint NOT NULL DEFAULT 1 CHECK(fiscal_year_start_day BETWEEN 1 AND 31),
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(id,functional_currency_code)
);

CREATE TABLE user_entity_roles (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id),
  entity_id uuid NOT NULL REFERENCES entities(id),
  role_id uuid NOT NULL REFERENCES roles(id),
  granted_by uuid REFERENCES users(id),
  granted_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);
CREATE UNIQUE INDEX uq_user_entity_active_role
ON user_entity_roles(user_id,entity_id,role_id) WHERE revoked_at IS NULL;

CREATE TABLE accounts (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  code varchar(50) NOT NULL,
  name varchar(200) NOT NULL,
  parent_id uuid,
  account_type varchar(20) NOT NULL CHECK(account_type IN ('ASSET','LIABILITY','EQUITY','INCOME','EXPENSE')),
  account_subtype varchar(80),
  system_role varchar(80),
  is_postable boolean NOT NULL DEFAULT true,
  is_system boolean NOT NULL DEFAULT false,
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,code),
  UNIQUE(id,entity_id),
  FOREIGN KEY(parent_id,entity_id) REFERENCES accounts(id,entity_id)
);

CREATE TABLE financial_accounts (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  account_id uuid NOT NULL,
  code varchar(80) NOT NULL,
  name varchar(200) NOT NULL,
  kind varchar(30) NOT NULL CHECK(kind IN ('CASH','BANK','MOBILE_WALLET','CREDIT_CARD','OTHER')),
  currency_code char(3) NOT NULL REFERENCES currencies(code),
  institution_name varchar(200),
  account_reference varchar(200),
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,code),
  UNIQUE(id,entity_id),
  FOREIGN KEY(account_id,entity_id) REFERENCES accounts(id,entity_id)
);

CREATE TABLE contacts (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  contact_type varchar(30) NOT NULL DEFAULT 'OTHER' CHECK(contact_type IN ('CUSTOMER','SUPPLIER','EMPLOYEE','OWNER','OTHER')),
  display_name varchar(200) NOT NULL,
  phone varchar(80),
  email varchar(320),
  notes text,
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(id,entity_id)
);

CREATE TABLE exchange_rates (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  rate_date date NOT NULL,
  from_currency_code char(3) NOT NULL REFERENCES currencies(code),
  to_currency_code char(3) NOT NULL REFERENCES currencies(code),
  rate numeric(28,12) NOT NULL CHECK(rate>0),
  source varchar(20) NOT NULL CHECK(source IN ('MANUAL','INTEGRATION','SYSTEM')),
  source_reference varchar(250),
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(from_currency_code<>to_currency_code),
  UNIQUE(entity_id,rate_date,from_currency_code,to_currency_code,source)
);

CREATE TABLE transactions (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  transaction_type varchar(30) NOT NULL CHECK(transaction_type IN ('INCOME','EXPENSE','ACCOUNT_TRANSFER','INTER_ENTITY','MANUAL_JOURNAL','ADJUSTMENT','REVERSAL')),
  status varchar(20) NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','POSTED','VOIDED')),
  transaction_date date NOT NULL,
  description text NOT NULL,
  contact_id uuid,
  primary_financial_account_id uuid,
  currency_code char(3) NOT NULL REFERENCES currencies(code),
  total_amount numeric(24,6) NOT NULL CHECK(total_amount>=0),
  source_type varchar(20) NOT NULL DEFAULT 'USER' CHECK(source_type IN ('USER','INTEGRATION','SYSTEM')),
  source_system varchar(120),
  external_reference varchar(250),
  original_transaction_id uuid REFERENCES transactions(id),
  reversal_transaction_id uuid REFERENCES transactions(id),
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  posted_by uuid REFERENCES users(id),
  posted_at timestamptz,
  voided_by uuid REFERENCES users(id),
  voided_at timestamptz,
  void_reason text,
  UNIQUE(id,entity_id),
  FOREIGN KEY(contact_id,entity_id) REFERENCES contacts(id,entity_id),
  FOREIGN KEY(primary_financial_account_id,entity_id) REFERENCES financial_accounts(id,entity_id)
);

CREATE TABLE transaction_splits (
  id uuid PRIMARY KEY,
  transaction_id uuid NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
  line_no integer NOT NULL CHECK(line_no>0),
  account_id uuid NOT NULL REFERENCES accounts(id),
  amount numeric(24,6) NOT NULL CHECK(amount>0),
  description text,
  UNIQUE(transaction_id,line_no)
);

CREATE TABLE journal_entries (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  transaction_id uuid REFERENCES transactions(id),
  journal_date date NOT NULL,
  description text NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','POSTED','REVERSED')),
  functional_currency_code char(3) NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  posted_by uuid REFERENCES users(id),
  posted_at timestamptz,
  reversal_of_journal_id uuid REFERENCES journal_entries(id),
  reversed_by_journal_id uuid REFERENCES journal_entries(id),
  UNIQUE(transaction_id)
);

CREATE TABLE journal_lines (
  id uuid PRIMARY KEY,
  journal_entry_id uuid NOT NULL REFERENCES journal_entries(id) ON DELETE RESTRICT,
  entity_id uuid NOT NULL REFERENCES entities(id),
  line_no integer NOT NULL CHECK(line_no>0),
  account_id uuid NOT NULL REFERENCES accounts(id),
  contact_id uuid REFERENCES contacts(id),
  financial_account_id uuid REFERENCES financial_accounts(id),
  description text,
  transaction_currency_code char(3) NOT NULL,
  transaction_debit_amount numeric(24,6) NOT NULL DEFAULT 0,
  transaction_credit_amount numeric(24,6) NOT NULL DEFAULT 0,
  functional_currency_code char(3) NOT NULL,
  fx_rate_to_functional numeric(28,12) NOT NULL CHECK(fx_rate_to_functional>0),
  debit_amount numeric(24,6) NOT NULL DEFAULT 0,
  credit_amount numeric(24,6) NOT NULL DEFAULT 0,
  exchange_rate_id uuid REFERENCES exchange_rates(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK((debit_amount>0 AND credit_amount=0) OR (credit_amount>0 AND debit_amount=0)),
  CHECK((transaction_debit_amount>0 AND transaction_credit_amount=0) OR (transaction_credit_amount>0 AND transaction_debit_amount=0)),
  UNIQUE(journal_entry_id,line_no)
);

CREATE TABLE entity_accounting_controls (
  entity_id uuid PRIMARY KEY REFERENCES entities(id),
  transactions_locked_through_date date,
  locked_by uuid REFERENCES users(id),
  locked_at timestamptz,
  lock_reason text,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE accounting_lock_events (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  action varchar(20) NOT NULL CHECK(action IN ('LOCK','UNLOCK')),
  previous_lock_date date,
  new_lock_date date,
  reason text NOT NULL,
  actor_user_id uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE inter_entity_account_mappings (
  id uuid PRIMARY KEY,
  entity_id uuid NOT NULL REFERENCES entities(id),
  counterparty_entity_id uuid NOT NULL REFERENCES entities(id),
  due_from_account_id uuid NOT NULL REFERENCES accounts(id),
  due_to_account_id uuid NOT NULL REFERENCES accounts(id),
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,counterparty_entity_id),
  CHECK(entity_id<>counterparty_entity_id)
);

CREATE TABLE inter_entity_transactions (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  initiating_entity_id uuid NOT NULL REFERENCES entities(id),
  counterparty_entity_id uuid NOT NULL REFERENCES entities(id),
  purpose varchar(40) NOT NULL,
  initiating_amount numeric(24,6) NOT NULL CHECK(initiating_amount>0),
  initiating_currency_code char(3) NOT NULL,
  counterparty_amount numeric(24,6) NOT NULL CHECK(counterparty_amount>0),
  counterparty_currency_code char(3) NOT NULL,
  initiating_transaction_id uuid NOT NULL REFERENCES transactions(id),
  counterparty_transaction_id uuid NOT NULL REFERENCES transactions(id),
  elimination_behavior varchar(30) NOT NULL DEFAULT 'AUTO_ELIMINATE',
  description text NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  posted_at timestamptz,
  CHECK(initiating_entity_id<>counterparty_entity_id)
);

CREATE TABLE integration_connections (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id),
  system_code varchar(100) NOT NULL,
  name varchar(200) NOT NULL,
  secret_reference varchar(300),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,system_code)
);

CREATE TABLE integration_events (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  integration_connection_id uuid NOT NULL REFERENCES integration_connections(id),
  external_event_id varchar(250) NOT NULL,
  event_type varchar(120) NOT NULL,
  external_reference varchar(250),
  occurred_at timestamptz,
  received_at timestamptz NOT NULL DEFAULT now(),
  payload_hash varchar(128),
  status varchar(20) NOT NULL DEFAULT 'RECEIVED' CHECK(status IN ('RECEIVED','PROCESSING','POSTED','FAILED','IGNORED')),
  transaction_id uuid REFERENCES transactions(id),
  error_code varchar(120),
  error_message text,
  processed_at timestamptz,
  UNIQUE(integration_connection_id,external_event_id)
);

CREATE TABLE idempotency_records (
  id uuid PRIMARY KEY,
  scope varchar(120) NOT NULL,
  idempotency_key varchar(200) NOT NULL,
  request_hash varchar(128),
  response_status integer,
  response_body jsonb,
  resource_public_id public_ulid,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  UNIQUE(scope,idempotency_key)
);

CREATE TABLE audit_events (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  actor_type varchar(20) NOT NULL CHECK(actor_type IN ('USER','SYSTEM','INTEGRATION','ANONYMOUS')),
  actor_user_id uuid REFERENCES users(id),
  actor_role_snapshot varchar(80),
  entity_id uuid REFERENCES entities(id),
  action varchar(120) NOT NULL,
  resource_type varchar(100),
  resource_id uuid,
  resource_public_id public_ulid,
  outcome varchar(20) NOT NULL CHECK(outcome IN ('SUCCESS','FAILED','DENIED')),
  source varchar(20) NOT NULL CHECK(source IN ('WEB','API','INTEGRATION','SYSTEM')),
  request_id varchar(120),
  ip_address inet,
  user_agent text,
  before_data jsonb,
  after_data jsonb,
  reason text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity_time ON audit_events(entity_id,occurred_at DESC);
CREATE INDEX idx_tx_entity_date ON transactions(entity_id,transaction_date DESC);
CREATE INDEX idx_journal_entity_date ON journal_entries(entity_id,journal_date DESC);

CREATE OR REPLACE FUNCTION prevent_append_only_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'append-only table'; END $$;
CREATE TRIGGER trg_audit_no_update BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION prevent_append_only_mutation();
CREATE TRIGGER trg_lock_events_no_update BEFORE UPDATE OR DELETE ON accounting_lock_events FOR EACH ROW EXECUTE FUNCTION prevent_append_only_mutation();

CREATE OR REPLACE FUNCTION protect_posted_journal_lines() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s varchar(20);
BEGIN
  SELECT status INTO s FROM journal_entries WHERE id=COALESCE(NEW.journal_entry_id,OLD.journal_entry_id);
  IF s IN ('POSTED','REVERSED') THEN RAISE EXCEPTION 'posted journal lines are immutable'; END IF;
  RETURN COALESCE(NEW,OLD);
END $$;
CREATE TRIGGER trg_journal_lines_insert BEFORE INSERT ON journal_lines FOR EACH ROW EXECUTE FUNCTION protect_posted_journal_lines();
CREATE TRIGGER trg_journal_lines_update BEFORE UPDATE OR DELETE ON journal_lines FOR EACH ROW EXECUTE FUNCTION protect_posted_journal_lines();

CREATE OR REPLACE FUNCTION protect_account_type_if_used() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.account_type IS DISTINCT FROM OLD.account_type AND EXISTS(
    SELECT 1 FROM journal_lines jl JOIN journal_entries je ON je.id=jl.journal_entry_id
    WHERE jl.account_id=OLD.id AND je.status IN ('POSTED','REVERSED')
  ) THEN RAISE EXCEPTION 'account type cannot change after posted activity'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER trg_account_type BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION protect_account_type_if_used();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_events,idempotency_records,integration_events,integration_connections,inter_entity_transactions,inter_entity_account_mappings,accounting_lock_events,entity_accounting_controls,journal_lines,journal_entries,transaction_splits,transactions,exchange_rates,contacts,financial_accounts,accounts,user_entity_roles,entities,currencies,role_permissions,permissions,roles,users CASCADE;
DROP FUNCTION IF EXISTS protect_account_type_if_used() CASCADE;
DROP FUNCTION IF EXISTS protect_posted_journal_lines() CASCADE;
DROP FUNCTION IF EXISTS prevent_append_only_mutation() CASCADE;
DROP DOMAIN IF EXISTS public_ulid CASCADE;
-- +goose StatementEnd
