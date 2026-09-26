# API V1

Base path: `/api/v1`

## Authentication

Public:

- `POST /auth/login`

Authenticated:

- `GET /auth/me`
- `POST /auth/logout`
- `POST /auth/change-password`
- `GET /auth/sessions`
- `POST /auth/sessions/revoke-others`
- `DELETE /auth/sessions/{session_ulid}`

Normal web authentication uses an HttpOnly session cookie. Opaque Bearer session tokens are also accepted. Browser login uses only the HttpOnly cookie. A raw session token is returned only when the client explicitly requests Bearer transport with `X-Session-Transport: bearer`.

Development-only auth is available only when the backend is explicitly configured with `AUTH_MODE=dev`:

`Authorization: Bearer dev:<USER_ULID>`

Optional mutation safety for JSON requests:

`Idempotency-Key: <client-generated-key>`

Multipart attachment uploads intentionally bypass the generic idempotency-body buffer.

## Global

- `GET /entities`
- `POST /entities` — requires OWNER on an existing entity, or `platform_owner` on the signed-in user. The bootstrap user is a platform owner, so a blank installation can create its first entity and manage platform settings (currencies, users, and the storage probe) before any entity exists. That create grants OWNER on the new entity. It does not grant access to entities the user is not a member of. Ordinary user creation does not set `platform_owner`. `GET /auth/me` returns `platform_owner`.
- `GET /users`
- `POST /users`
- `POST /users/{user_ulid}/reset-password` — OWNER; revokes target sessions
- `GET /dashboard/combined?currency=MMK&from=YYYY-MM-DD&to=YYYY-MM-DD`
- `GET /financial-account-balances` — financial accounts for every entity the signed-in user can access. Balances stay in each account currency.
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
- `GET /transactions/export.csv`
- `POST /transactions`
- `PUT /transactions/{transaction_ulid}` — edit an `INCOME`/`EXPENSE` draft only
- `POST /transactions/{transaction_ulid}/cancel` — mark a draft `VOIDED`; requires an audit reason
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

The list response includes `income_total`, `expense_total`, `net_total`, `functional_currency`, `count`, `has_more`, and per-row `attachment_count`. Summary totals are functional-currency P&L for the filtered set. Per-row running net and functional effect are not returned. Use `limit` and `offset`; the default page size in the UI is 25. CSV export walks every matching row, not only the visible page.

`GET /api/v1/entities/{entity}/audit-actions` returns the distinct audit `action` values for that entity, sorted alphabetically. `GET /audit-events?action=` filters by that exact value. Search remains a separate free-text filter.

`GET /api/v1/currencies?active=1` lists currencies that can be selected for new entities, financial accounts, and exchange rates. Currency create, update, and delete require OWNER. A referenced currency cannot be deleted; deactivate it instead. Historical rows keep their stored currency code.

`/entities/{entity}/contact-types` is the entity-scoped contact type master. New contacts must use an active type from that entity. Deactivating a type leaves existing contacts valid.

Same-currency transfers require `from amount = to amount`, or `from amount = to amount + fee` when `fee_amount` and an EXPENSE `fee_expense_account` are supplied. The fee is an explicit debit. Cross-currency transfers still balance in functional currency using the stored FX snapshot and do not accept a transfer fee.

Running/summary values are journal-derived in entity functional currency. Reversals unwind prior income/expense effects.

Draft creation, draft re-dating, and draft cancellation are rejected for dates at or before the entity's accounting lock. Posted transactions cannot be edited; corrections use reversal.

### Attachments

Transaction attachments are private R2 objects.

- `GET /transactions/{transaction_ulid}/attachments`
- `POST /transactions/{transaction_ulid}/attachments` — multipart field `files`, up to 10 per request
- `PUT /transactions/{transaction_ulid}/attachments/reorder`
- `DELETE /transactions/{transaction_ulid}/attachments/{attachment_ulid}` — audited soft-removal plus best-effort object cleanup
- `GET /transactions/{transaction_ulid}/attachments/{attachment_ulid}/content` — authenticated streaming content

Current per-file limit defaults to 20 MB and is configurable with `ATTACHMENT_MAX_MB`. A transaction may have at most 50 active attachments.

Supported upload types currently include JPEG, PNG, WebP, GIF, PDF, text, CSV, DOCX, and XLSX. Images/PDF/text/CSV are previewable inline; other supported types download as attachments.

### Inter-entity

Daily posting and setup are separate.

- `GET /inter-entity-status/{counterparty_entity_ulid}` — whether the pair is configured, plus currencies and the due-account names needed for a read-only preview. Does not accept mapping changes.
- `POST /inter-entity-transactions` — atomic payment for another entity. The caller must be allowed to operate the ledger on both entities. Same functional currency requires one equal amount.
- `GET /inter-entity-setup`
- `PUT /inter-entity-pairs/{counterparty_entity_ulid}` — saves both directions in one database transaction. OWNER or ADMIN on both entities.
- `GET /inter-entity-mappings`
- `PUT /inter-entity-mappings/{counterparty_entity_ulid}`

ACCOUNTANT, BOOKKEEPER, and VIEWER receive 403 on setup routes. A missing pair returns a setup message rather than posting.

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

- `GET /health` — process liveness
- `GET /ready` — PostgreSQL readiness and attachment-storage configuration state
