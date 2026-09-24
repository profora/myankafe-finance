# API V1

Base path: `/api/v1`

## Authentication

Public:

- `POST /auth/login`

Authenticated:

- `GET /auth/me`
- `POST /auth/logout`
- `POST /auth/change-password`

Normal web authentication uses an HttpOnly session cookie. Opaque Bearer session tokens are also accepted. The response from login includes the session token for future native-client use; browser code should rely on the HttpOnly cookie.

Development-only auth is available only when the backend is explicitly configured with `AUTH_MODE=dev`:

`Authorization: Bearer dev:<USER_ULID>`

Optional mutation safety for JSON requests:

`Idempotency-Key: <client-generated-key>`

Multipart attachment uploads intentionally bypass the generic idempotency-body buffer.

## Global

- `GET /entities`
- `POST /entities`
- `GET /users`
- `POST /users`
- `POST /users/{user_ulid}/reset-password` — OWNER; revokes target sessions
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

Report endpoints accept applicable `from`, `to`, or `through` date query parameters.

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
- `GET /transactions/{transaction_ulid}`
- `POST /transactions`
- `POST /transactions/{transaction_ulid}/post`
- `POST /transactions/{transaction_ulid}/reverse`
- `POST /transfers`
- `POST /manual-journals`

Transaction-list query parameters:

- `q` — description, ULID, contact, financial account, external reference, or attachment filename
- `status` — `DRAFT`, `POSTED`, `VOIDED`
- `type` — `INCOME`, `EXPENSE`, `ACCOUNT_TRANSFER`, `INTER_ENTITY`, `MANUAL_JOURNAL`, `ADJUSTMENT`, `REVERSAL`
- `from`, `to` — inclusive transaction dates
- `financial_account_id` — matches primary account or journal-line financial accounts
- `limit` — maximum 500
- `offset`

The list response includes `income_total`, `expense_total`, `net_total`, `functional_currency`, count/pagination metadata, and per-row `functional_effect`, `running_net`, and `attachment_count`.

Running/summary values are journal-derived in entity functional currency. Reversals unwind prior income/expense effects.

### Attachments

Transaction attachments are private R2 objects.

- `GET /transactions/{transaction_ulid}/attachments`
- `POST /transactions/{transaction_ulid}/attachments` — multipart field `files`, up to 10 per request
- `GET /transactions/{transaction_ulid}/attachments/{attachment_ulid}/content`

Current per-file limit defaults to 20 MB and is configurable with `ATTACHMENT_MAX_MB`. A transaction may have at most 50 active attachments.

Supported upload types currently include JPEG, PNG, WebP, GIF, PDF, text, CSV, DOCX, and XLSX. Images/PDFs are served inline; other supported types download as attachments.

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
