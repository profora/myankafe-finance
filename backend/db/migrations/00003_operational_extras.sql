-- +goose Up
CREATE TABLE account_transfer_details (
  transaction_id uuid PRIMARY KEY REFERENCES transactions(id) ON DELETE RESTRICT,
  from_financial_account_id uuid NOT NULL REFERENCES financial_accounts(id),
  to_financial_account_id uuid NOT NULL REFERENCES financial_accounts(id),
  from_amount numeric(24,6) NOT NULL CHECK(from_amount>0),
  from_currency_code char(3) NOT NULL REFERENCES currencies(code),
  to_amount numeric(24,6) NOT NULL CHECK(to_amount>0),
  to_currency_code char(3) NOT NULL REFERENCES currencies(code),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(from_financial_account_id<>to_financial_account_id)
);

CREATE TABLE transaction_attachments (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  transaction_id uuid NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
  storage_key varchar(500) NOT NULL,
  original_filename varchar(300) NOT NULL,
  mime_type varchar(150) NOT NULL,
  size_bytes bigint NOT NULL CHECK(size_bytes>=0),
  uploaded_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tags (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id) ON DELETE RESTRICT,
  name varchar(120) NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,name)
);

CREATE TABLE transaction_tag_links (
  transaction_id uuid NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
  PRIMARY KEY(transaction_id,tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS transaction_tag_links;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS transaction_attachments;
DROP TABLE IF EXISTS account_transfer_details;
