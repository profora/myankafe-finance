# MyanKafe Finance Architecture

## Purpose

MyanKafe Finance is a private multi-entity accounting platform for MyanKafe, Royal Masterpiece, Personal, and future entities. Each entity has independent books, functional currency, accounts, financial accounts, contacts, lock state, permissions, and reporting.

## Runtime

- Backend: Go 1.23, chi, pgx
- Database: PostgreSQL 17
- Migrations: Goose, append-only numbered SQL
- Frontend: Next.js 15, React 19, TypeScript
- Internal relational IDs: UUIDv7
- Public API IDs: ULID
- Deployment baseline: Docker Compose

## Non-negotiable accounting invariants

1. A posted journal must contain at least two lines and balance in the entity functional currency.
2. Posted journal lines are immutable.
3. Posted transaction and journal accounting fields are immutable. Corrections use reversal entries.
4. Every account referenced by a journal line belongs to the same entity as the journal.
5. Simple income/expense entries create deterministic double-entry journals.
6. Transfers create debit/credit movements between financial accounts and must balance in functional currency.
7. Inter-entity expense posting commits both entities in one PostgreSQL transaction or commits neither.
8. Inter-entity mappings use an ASSET due-from account and a LIABILITY due-to account.
9. Entity accounting lock dates block posting and reversal on or before the locked-through date.
10. Only OWNER may unlock an entity period.
11. Exchange-rate snapshots used by posted accounting are immutable.
12. Business audit events and lock events are append-only.
13. Public routes use ULIDs; internal UUIDv7 identifiers are not normal API identifiers.
14. Mutating requests may use `Idempotency-Key`; equal retries replay, mismatched reuse is rejected.

## Accounting layers

### Transactions

`transactions` is the user/business event layer. It stores the entity, event type, date, description, contact, financial account, currency, amount, lifecycle state, and reversal relationship.

### Journals

`journal_entries` and `journal_lines` are the accounting source of truth. Reports are generated from posted/reversed journal history, not from transaction totals.

### Functional and transaction currency

Each journal line preserves both transaction-currency values and functional-currency debit/credit values plus the FX snapshot used to derive the functional value.

### Reversals

A correction never rewrites posted accounting. Reversal creates:
- a new `REVERSAL` transaction,
- a new opposite journal,
- links between original and reversal,
- original transaction state `VOIDED`,
- original journal state `REVERSED`.

Both journals remain reportable so the accounting history is explicit.

## Entity isolation

The application service validates entity ownership before mutation. PostgreSQL additionally enforces entity boundaries with composite foreign keys and integrity triggers for:
- journal lines,
- transaction splits,
- financial accounts,
- contacts,
- account transfers,
- inter-entity mappings.

## Audit model

Two layers are intentional:

- Business audit events: richer atomic events written inside important accounting/database transactions.
- HTTP request audit events: authenticated API actions recorded with method, path, status, entity when resolvable, and outcome.

The audit log itself is append-only.

## Idempotency

For authenticated POST/PUT/PATCH/DELETE requests, callers may supply `Idempotency-Key`.

The key is scoped by user + HTTP method + request URI. The request body SHA-256 is stored.

- First request claims the key for 24 hours.
- Identical completed retry returns the stored response with `Idempotency-Replayed: true`.
- Same key with a different body returns HTTP 409.
- Concurrent duplicate while the first is active returns HTTP 409.
- 5xx responses release the key so the caller can retry.

## Reports

Implemented from journal history:

- entity dashboard
- all-entity dashboard translated into an explicit reporting currency
- Profit & Loss
- Balance Sheet with current earnings
- Trial Balance
- General Ledger
- Account Ledger API
- cash movement
- inter-entity balances

Combined reporting translates each entity's functional totals using stored entity FX rates. It is management reporting, not a statutory consolidation engine.

## Authentication boundary

The normal authentication mode is username/password with database-backed opaque sessions, adapted from the Royal Masterpiece platform pattern.

- usernames are normalized lowercase identifiers
- passwords use Argon2id
- raw 32-byte session tokens are returned only to the client
- PostgreSQL stores only the SHA-256 session-token hash
- web clients use an HttpOnly SameSite=Strict cookie
- Bearer session tokens are accepted for future native clients
- sessions expire and may be revoked
- password changes revoke previous sessions and issue a replacement session
- OWNER password reset revokes all sessions for the target user
- login failures/rate limiting and auth lifecycle events are audited
- production requires `AUTH_MODE=password` and secure cookies

The legacy `Authorization: Bearer dev:<USER_ULID>` path remains only for explicit development/test mode; production rejects it.

## Attachment storage

Transaction attachments are private Cloudflare R2 objects accessed only through the Go backend.

- R2 credentials never reach Next.js/browser code.
- A transaction may have multiple ordered active attachments.
- The API accepts multi-file multipart uploads and records SHA-256, MIME type, size, storage key, filename, uploader, and display order.
- Object identity/content metadata becomes immutable after registration.
- Images and PDFs are streamed through authenticated endpoints for inline preview.
- Other supported document types are served as attachments.
- Image previews support fullscreen zoom/navigation in the web client.
- Upload cleanup is best-effort if database registration fails.
- Multipart requests bypass generic body-buffer idempotency to avoid duplicating large receipt bodies in memory.

## Transaction operations view

The transaction list is a server-filtered accounting operations view.

Filters include search text, status, type, date range, and financial account. Search includes descriptions, ULIDs, contacts, financial accounts, external references, and attachment filenames. Financial-account filtering also matches financial accounts represented on journal lines, so both sides of transfers can be found.

Summary and running figures are calculated from posted/reversed journal effects in the entity functional currency. This avoids adding unlike transaction currencies. Reversal journals unwind income/expense summaries instead of leaving original amounts overstated.


## Role model

Authorization is enforced server-side. UI visibility is not a security boundary.

- **OWNER**
  - full entity access
  - reset user passwords and revoke their sessions
  - create platform users and new entities
  - configure accounting
  - operate the ledger
  - post manual journals
  - reverse posted accounting
  - lock and unlock accounting periods
  - assign entity roles

- **ADMIN**
  - configure accounting
  - operate the ledger
  - view entity users and audit history
  - cannot unlock periods
  - cannot reverse posted accounting solely by being ADMIN
  - cannot create platform users/entities unless also OWNER somewhere

- **ACCOUNTANT**
  - configure accounting
  - operate the ledger
  - post manual journals
  - reverse posted accounting
  - lock accounting periods
  - view audit history
  - cannot unlock periods

- **BOOKKEEPER**
  - create contacts
  - create/post ordinary income and expense entries
  - post account transfers and permitted inter-entity operations
  - cannot configure COA, financial accounts, FX rates, or inter-entity mappings
  - cannot reverse posted accounting
  - cannot view audit history
  - cannot lock/unlock periods

- **VIEWER**
  - read-only access to entity data and reports
  - no ledger mutations

A user with no entity assignment cannot self-create an entity. New entity creation requires the actor to already hold OWNER on at least one entity.
