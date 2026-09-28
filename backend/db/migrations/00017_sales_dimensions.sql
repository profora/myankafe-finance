-- +goose Up
-- +goose StatementBegin
CREATE TABLE sales_channels (
  id uuid PRIMARY KEY,
  public_id public_ulid NOT NULL UNIQUE,
  entity_id uuid NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
  code varchar(40) NOT NULL,
  name varchar(120) NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(entity_id,code),
  UNIQUE(id,entity_id)
);

ALTER TABLE contacts
  ADD COLUMN customer_segment varchar(30)
  CHECK(customer_segment IS NULL OR customer_segment IN ('CONSUMER','RETAILER','DISTRIBUTOR','OTHER'));

ALTER TABLE transactions
  ADD COLUMN sales_channel_id uuid;

ALTER TABLE transactions
  ADD CONSTRAINT fk_transactions_sales_channel_entity
  FOREIGN KEY(sales_channel_id,entity_id)
  REFERENCES sales_channels(id,entity_id)
  ON DELETE RESTRICT;

CREATE INDEX idx_sales_channels_entity_active ON sales_channels(entity_id,active,name);
CREATE INDEX idx_transactions_sales_channel ON transactions(entity_id,sales_channel_id)
  WHERE sales_channel_id IS NOT NULL;
CREATE INDEX idx_contacts_customer_segment ON contacts(entity_id,customer_segment)
  WHERE customer_segment IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_contacts_customer_segment;
DROP INDEX IF EXISTS idx_transactions_sales_channel;
DROP INDEX IF EXISTS idx_sales_channels_entity_active;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS fk_transactions_sales_channel_entity;
ALTER TABLE transactions DROP COLUMN IF EXISTS sales_channel_id;
ALTER TABLE contacts DROP COLUMN IF EXISTS customer_segment;
DROP TABLE IF EXISTS sales_channels;
-- +goose StatementEnd
