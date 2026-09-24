# API V1

Base path: `/api/v1`

Development auth:

`Authorization: Bearer dev:<USER_ULID>`

Optional mutation safety:

`Idempotency-Key: <client-generated-key>`

## Global

- `GET /entities`
- `POST /entities`
- `GET /users`
- `POST /users`
- `GET /dashboard/combined?currency=MMK&from=YYYY-MM-DD&to=YYYY-MM-DD`
- `POST /inter-entity-transactions`

## Entity-scoped

Prefix: `/entities/{entity_ulid}`

### Dashboard and reports

- `GET /dashboard`
- `GET /reports/profit-loss`
- `GET /reports/balance-sheet`
- `GET /reports/trial-balance`
- `GET /reports/general-ledger`
- `GET /reports/account-ledger?account_id=<account_ulid>`
- `GET /reports/cash-movement`
- `GET /reports/inter-entity-balances`

Report endpoints accept the applicable `from`, `to`, or `through` date query parameters.

### Chart of Accounts

- `GET /accounts`
- `POST /accounts`

### Financial accounts

- `GET /financial-accounts`
- `POST /financial-accounts`

### Contacts

- `GET /contacts`
- `POST /contacts`

### FX

- `GET /exchange-rates`
- `POST /exchange-rates`

### Transactions

- `GET /transactions`
- `POST /transactions`
- `POST /transactions/{transaction_ulid}/post`
- `POST /transactions/{transaction_ulid}/reverse`
- `POST /transfers`
- `POST /manual-journals`

### Inter-entity

- `GET /inter-entity-mappings`
- `PUT /inter-entity-mappings/{counterparty_entity_ulid}`

Both sides must have the appropriate mapping before an inter-entity expense can post.

### Access

- `GET /users`
- `PUT /users/role`

Entity roles: `OWNER`, `ADMIN`, `ACCOUNTANT`, `BOOKKEEPER`, `VIEWER`.

### Accounting lock

- `GET /accounting-lock`
- `POST /accounting-lock`
- `POST /accounting-lock/unlock`

Only OWNER can unlock.

### Audit

- `GET /audit-events?limit=200`

## Health

Outside the API auth boundary:

- `GET /health`
