# Cursor Handoff

Branch: `feat/v1-accounting-foundation`  
PR: `#1 feat: V1 multi-entity accounting foundation`

## What is already implemented

### Backend and database

- PostgreSQL 17 schema and Goose migrations through 00011
- UUIDv7 internal IDs / canonical ULID public IDs
- multi-entity books and per-entity roles
- hierarchical Chart of Accounts
- cash/bank/mobile-wallet/card/other financial accounts
- contacts/payees/payers
- income/expense entry with split categories
- deterministic double-entry posting
- manual journals
- same-entity transfers
- stored exchange-rate snapshots
- atomic inter-entity expense posting with due-to/due-from mappings
- accounting-period locking; OWNER-only unlock
- reversal-based correction of posted transactions
- append-only business/request/auth audit history
- durable JSON mutation idempotency
- dashboard and accounting reports
- entity/user/role administration
- Argon2id username/password credentials
- opaque database-backed sessions, HttpOnly cookies, Bearer-session support
- own-password change, OWNER password reset, active-session listing/revocation, and sign-out-other-devices
- login rate limiting and anti-enumeration dummy password verification
- private Cloudflare R2 transaction attachments
- multi-file upload, ordered metadata, reorder, audited soft-removal, and authenticated content streaming
- searchable/filterable transaction list with journal-derived functional-currency totals and running net
- database integrity triggers for entity boundaries, balance, lifecycle, locks, FX, and posted immutability

### Frontend

Working Next.js screens/workflows include:

- username/password login and logout
- password change
- dashboard
- searchable/filterable/paginated transaction list
- New Transaction dropdown: income, expense, transfer, manual journal, inter-entity
- transaction detail with actual journal debit/credit lines
- new income/expense entry with multiple attachments
- transfers with attachments
- inter-entity posting/mapping with attachments on the initiating transaction
- manual journals with attachments
- private attachment gallery
- fullscreen image preview with zoom, pan, keyboard navigation and reset
- inline PDF, text and CSV preview
- contacts
- Chart of Accounts + account-ledger drill-down
- financial accounts
- exchange rates
- accounting reports and date ranges
- audit log
- transaction locking
- entity settings
- user/entity-role settings including initial passwords and OWNER reset

## Validation

GitHub Actions validates:

- frontend TypeScript
- Next.js production build
- Go module resolution
- fresh Goose migrations on PostgreSQL 17
- Go unit and PostgreSQL integration tests
- production API Docker image build
- production non-root web Docker image build

Tests now cover:

- journal balancing and posting guards
- period locks
- cross-entity boundaries
- posted immutability
- FX snapshot immutability/math
- role authorization
- idempotency claim/replay/concurrency plus HTTP replay behavior
- reversal success and locked reversal rejection
- inter-entity success and forced late-failure rollback
- transaction search/running-total reversal semantics
- attachment metadata ordering
- Argon2/password/session-token helpers
- R2 configuration/canonical-path helpers
- username/password session lifecycle (login/me/logout) when the latest CI reaches that test

Do not merge if CI is red.

## Tasks Cursor should do next

1. **Dependency/format reproducibility**
   - run `gofmt -w backend` and commit the result
   - `cd backend && go mod tidy`; commit `go.sum`
   - `cd frontend && npm install`; commit `package-lock.json`
   - switch CI to `npm ci`
   - make formatting and `go mod tidy` checks non-mutating/fail-on-diff

2. **Real R2 acceptance**
   - configure a private production/staging R2 bucket
   - validate upload, image/PDF preview, error handling, and object cleanup with real credentials
   - confirm the chosen reverse-proxy request-size limits allow the configured attachment size

3. **Production operations**
   - secret management for PostgreSQL, bootstrap credentials, cookie/session configuration, and R2 keys
   - TLS/reverse proxy
   - `AUTH_COOKIE_SECURE=true`
   - exact production `CORS_ORIGIN`
   - PostgreSQL backups/PITR
   - migration/deployment runbook
   - structured logs/metrics and alerting

4. **Polish without changing accounting semantics**
   - mobile navigation drawer is already implemented; do final device QA
   - loading/skeleton states
   - accounting unlock confirmation is already implemented; preserve its OWNER-only audited flow
   - richer table sorting/export if desired
   - final accessibility/browser review
   - visual polish of forms and attachment gallery
   - preserve the supplied Chieftain logo; do not replace it with the old MK placeholder
   - the Chieftain logo supplied by the user is the canonical Finance brand asset for sidebar, login and app icon

5. **Optional future integrations**
   - Royal Masterpiece ingestion connector using integration events/external references
   - bank-feed ingestion
   - recurring/budgets (Phase 2)
   - more advanced approvals if needed

## Deployment probes

- `/health` — liveness only
- `/ready` — PostgreSQL readiness plus attachment-storage configuration state
- Compose starts the web service only after API readiness is healthy

## Do not reimplement

Authentication and R2 receipt storage are no longer placeholders. Do not replace them casually during UI polishing. The patterns were adapted from `profora/rm-floral-platform`.

## Accounting rules Cursor must not weaken

- Never edit/delete posted journal lines.
- Never fix posted accounting in place; reverse and repost.
- Never expose internal UUIDs as the normal public API identity.
- Never allow an entity to post another entity's account/financial account/contact.
- Never bypass entity lock validation.
- Never make unlock available to roles other than OWNER.
- Never split inter-entity posting into independent commits.
- Never use live/latest FX retroactively in historical reports; preserve stored snapshots.
- Keep R2 credentials backend-only.
- Do not expose raw stored session-token hashes or password hashes.
