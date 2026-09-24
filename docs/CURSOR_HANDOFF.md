# Cursor Handoff

Branch: `feat/v1-accounting-foundation`  
PR: `#1 feat: V1 multi-entity accounting foundation`

## What is already implemented

### Backend and database

- PostgreSQL 17 schema and Goose migrations
- UUIDv7 internal IDs
- canonical ULID public IDs
- multi-entity books and per-entity roles
- hierarchical Chart of Accounts
- cash/bank/mobile-wallet/card/other financial accounts
- contacts/payees/payers
- income and expense entry with split categories
- deterministic double-entry posting
- manual journals
- same-entity account transfers
- exchange-rate snapshots
- atomic inter-entity expense posting with due-to/due-from mappings
- entity transaction locking
- OWNER-only unlock
- reversal-based correction of posted transactions
- append-only business/request audit history
- durable API idempotency support
- dashboard and accounting reports
- entity/user/role administration
- database integrity triggers that enforce entity boundaries and posted immutability

### Frontend

Working Next.js admin screens exist for:

- dashboard
- transactions and posting
- new income/expense entry
- transfers
- inter-entity posting/mapping
- contacts
- Chart of Accounts
- financial accounts
- exchange rates
- manual journals
- reports
- audit log
- transaction locking
- entity settings
- user/entity-role settings
- development identity selector

## CI state

GitHub Actions currently validates:

- frontend TypeScript
- Next.js production build
- Go module resolution
- Go tests
- fresh Goose migration of PostgreSQL 17

Do not merge if those checks are red.

## Tasks Cursor should do next

1. Run a local formatting/dependency normalization pass and commit generated lock metadata:
   - `gofmt -w backend`
   - `cd backend && go mod tidy` and commit `go.sum`
   - `cd frontend && npm install` and commit `package-lock.json`
   - once lockfiles exist, change CI to `npm ci` and make `go mod tidy`/format checks fail on dirty output rather than mutating the runner checkout.

2. Add PostgreSQL integration tests for the critical invariants:
   - posted unbalanced journal rejected,
   - locked-date post/reversal rejected,
   - cross-entity split rejected,
   - posted journal-line mutation rejected,
   - used FX rate mutation rejected,
   - duplicate idempotency key cannot double-post,
   - inter-entity pair is all-or-nothing,
   - reversal preserves original and produces opposite journal.

3. Polish the admin UI without changing accounting semantics:
   - stronger responsive layout,
   - loading/skeleton states,
   - accessible dialogs instead of `window.prompt`,
   - confirmation dialog for reversal/unlock,
   - better table pagination/filtering,
   - account-ledger drill-down from COA/report rows,
   - user-friendly number/date formatting.

4. Production authentication:
   - replace the dev header boundary with the chosen private admin authentication mechanism,
   - retain the existing per-entity authorization checks,
   - never permit `AUTH_MODE=dev` in production.

5. Production operations:
   - secret management,
   - TLS/reverse proxy,
   - PostgreSQL backups/PITR,
   - migration runbook,
   - structured logs/metrics,
   - object storage before implementing real receipt upload bytes.

## Deferred intentionally

These are not blockers for the accounting foundation and should not be improvised during UI polishing:

- real receipt/object upload storage
- Royal Masterpiece ingestion connector
- automatic bank feed ingestion
- statutory consolidated financial statements
- tax filing automation
- advanced approval workflows

The schema already contains integration event/idempotency foundations and attachment metadata for future work.

## Accounting rules Cursor must not weaken

- Never edit/delete posted journal lines.
- Never "fix" posted accounting in place; reverse and repost.
- Never expose internal UUIDs as the normal public API identity.
- Never allow an entity to post another entity's account/financial account/contact.
- Never bypass entity lock validation.
- Never make unlock available to roles other than OWNER.
- Never split inter-entity posting into independent commits.
- Never use live/latest FX retroactively in historical reports; preserve stored snapshots.
