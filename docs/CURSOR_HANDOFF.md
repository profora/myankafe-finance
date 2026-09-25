# Cursor Handoff

Branch: `feat/v1-accounting-foundation`  
PR: `#1 feat: V1 multi-entity accounting foundation`

## What is already implemented

### Backend and database

- PostgreSQL 17 schema and Goose migrations through 00013
- UUIDv7 internal IDs / canonical ULID public IDs
- multi-entity books and deterministic effective per-entity roles
- hierarchical Chart of Accounts
- cash/bank/mobile-wallet/card/other financial accounts
- contacts/payees/payers
- income/expense entry with split categories
- editable income/expense drafts with audited cancellation; posted transactions remain immutable
- locked-period protection on draft creation, re-dating, cancellation, posting and reversal
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
- structured JSON request/lifecycle logs with request IDs and latency/status fields
- Prometheus-compatible `/metrics` with production bearer protection
- OWNER-only System diagnostics page
- audited R2 write/read/delete acceptance probe
- database integrity triggers for entity boundaries, balance, lifecycle, locks, FX, and posted immutability

### Frontend

Working Next.js screens/workflows include:

- username/password login and logout
- password change
- dashboard
- searchable/filterable/paginated transaction list
- role-aware mutation/configuration controls matching the server authorization matrix
- New Transaction dropdown: income, expense, transfer, manual journal, inter-entity
- transaction detail with actual journal debit/credit lines
- transaction-detail draft editor with split editing, controlled posting and reason-required cancellation
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

- committed `backend/go.sum` / `frontend/package-lock.json`
- fail-on-diff Go formatting
- fail-on-diff `go mod tidy -diff`
- frontend install with `npm ci`
- frontend TypeScript
- Next.js production build
- fresh Goose migrations on PostgreSQL 17
- Go unit and PostgreSQL integration tests
- production API Docker image build
- production non-root web Docker image build

Tests now cover:

- journal balancing and posting guards
- period locks, including database-level draft creation/re-date/cancellation guards
- cross-entity boundaries
- posted immutability
- FX snapshot immutability/math
- role authorization and effective-role precedence
- idempotency claim/replay/concurrency plus HTTP replay behavior
- reversal success and locked reversal rejection
- inter-entity success and forced late-failure rollback
- transaction search/running-total reversal semantics
- attachment metadata ordering
- Argon2/password/session-token helpers
- R2 configuration/canonical-path helpers
- username/password session lifecycle (login/me/logout)
- draft edit/cancel lifecycle and locked-date rejection
- operational metrics authentication/rendering
- R2 write/read/delete probe behavior

Do not merge if CI is red. CI run #772 passed the complete frontend/backend/container gate on the implementation head after the operational logging/metrics and System/R2-probe work.

## Tasks Cursor should do next

1. **Real R2 acceptance**
   - configure a private production/staging R2 bucket
   - validate upload, image/PDF preview, error handling, and object cleanup with real credentials
   - confirm the chosen reverse-proxy request-size limits allow the configured attachment size

2. **Production operations**
   - production Compose, secure env template, Caddy/TLS example, explicit migrate→API-ready→web deployment script/runbook, and logical backup script are now committed and CI-validated
   - still configure real PostgreSQL/R2/bootstrap secrets on the target host; never commit filled `.env.production`
   - keep `AUTH_COOKIE_SECURE=true` and set exact production `CORS_ORIGIN`
   - enable provider PostgreSQL backups/PITR and restore-test them; the logical dump script is only an additional portable backup
   - ship the already-structured JSON logs to the chosen log platform
   - scrape the already-implemented `/metrics` endpoint with its bearer token and configure alerting

3. **Polish without changing accounting semantics**
   - mobile navigation drawer is already implemented; do final device QA
   - loading/skeleton states (dashboard balances/metrics and transaction list now use accessible skeleton feedback; continue only where UX still feels abrupt)
   - accounting unlock confirmation is already implemented; preserve its OWNER-only audited flow
   - richer table sorting/export if desired (CSV export is already shipped for transactions, reports, General Ledger and Account Ledger)
   - final accessibility/browser review
   - visual polish of forms and attachment gallery
   - preserve the supplied Chieftain logo; do not replace it with the old MK placeholder
   - the Chieftain logo supplied by the user is the canonical Finance brand asset for sidebar, login and app icon

4. **Optional future integrations**
   - Royal Masterpiece ingestion connector using integration events/external references
   - bank-feed ingestion
   - recurring/budgets (Phase 2)
   - more advanced approvals if needed

## Deployment probes

- `/health` — liveness only
- `/ready` — PostgreSQL readiness plus attachment-storage configuration state
- `/metrics` — Prometheus-compatible HTTP counters/latency; hidden in production unless `METRICS_BEARER_TOKEN` is set
- OWNER `Settings → System` can run an audited R2 write/read/delete probe
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


## 2026-09-25 continuation note

- accessibility pass added skip-to-content navigation, visible keyboard focus, current-page semantics, labelled confirmation dialogs, password-toggle state, reduced-motion behavior, and alert/live-region improvements
- dashboard now avoids showing stale financial values while entity data is loading and exposes skeleton loading feedback
- transaction loading was de-duplicated: entity changes load financial-account choices separately while the canonical transaction loader owns filtered/paginated transaction refreshes
- transaction empty/loading state now uses accessible skeleton rows rather than a transient text-only blank table


## 2026-09-25 entity-switch/list-state polish

- added reusable table loading/empty rows for core entity-scoped lists
- Chart of Accounts, Contacts, Cash/Bank, Exchange Rates, and Users & Access now clear stale rows when entity context changes, announce errors/successes accessibly, and expose explicit loading/empty states
- exchange-rate default date now follows the newly selected entity timezone immediately
- password visibility toggles in user administration expose pressed state to assistive technology


## 2026-09-25 read-state polish

- account ledger and audit list now clear stale rows on entity/filter changes and use reusable accessible table loading/empty states
- transaction detail clears prior-entity detail before reload and exposes a non-stale loading state
- Security session listing, System diagnostics, and Transaction Locking now distinguish loading from empty/unavailable state and announce operational results/errors
- mutation semantics and accounting authorization were not changed


## 2026-09-25 entry-workflow reference-data hardening

- income/expense, transfer, manual-journal and inter-entity entry screens now reset entity-specific selections and dates when the active entity changes
- posting actions remain disabled until the selected entity's accounts/contacts/financial accounts are loaded
- reference selectors show explicit loading/choose states instead of appearing silently empty
- compact transaction-entry dialog now has labelled dialog semantics, accessible errors, and the same reference-data readiness guard
- this prevents accidental reuse of stale account IDs across entity switches without changing server accounting rules


## 2026-09-25 entity-context/report polish

- entity context now exposes loading state and automatically falls back only to a valid accessible entity after reloads/role changes
- entity selector disables itself while entity access is loading and clearly distinguishes loading from no-access state
- all accounting report panels clear prior-entity values before reload, show accessible loading/empty rows, and disable CSV export until the selected entity's report data is ready


## 2026-09-25 attachment/draft editor hardening

- transaction attachment lists now clear prior-transaction thumbnails before loading a new transaction and expose explicit loading/error state
- attachment preview receives focus when opened and has labelled zoom/navigation controls
- draft transaction editor now reloads accounts/contacts/financial accounts with cancellation protection, disables save until references are ready, and has labelled dialog semantics


## 2026-09-25 production config validation

- production startup now rejects non-HTTPS/non-origin CORS values, insecure configured R2 endpoints, and configured metrics bearer tokens shorter than 32 characters
- config tests cover invalid origins, HTTPS origins with ports, R2 scheme enforcement, and metrics token strength


## 2026-09-25 overnight deployment attempt

Finance was not started on the VPS. Existing MyanKafe and Royal Masterpiece services were left running and were still healthy afterward.

The shared droplet is `root@159.223.48.7`, deploy path reserved as `/opt/myankafe-finance`. Public entry is Cloudflare Tunnel, not Caddy or Nginx. Ports `8180` and `3100` were free. The managed PostgreSQL cluster is reachable, but no Finance database or application role exists, and the existing application roles cannot create one. `finance.myankafe.com` does not resolve, and neither tunnel ingress includes it. Database, R2, and initial OWNER values were not supplied, so migrations, bootstrap, and acceptance tests were not run.

Full evidence, the acceptance matrix, and the credential/rotation notes are in [docs/DEPLOYMENT_ACCEPTANCE_REPORT.md](DEPLOYMENT_ACCEPTANCE_REPORT.md).

PR #1 remains Draft. Do not merge it.

Recommended next step: create the Finance database and least-privilege role with the cluster `doadmin` account, supply R2 credentials and an OWNER bootstrap password, add a path-split Cloudflare Tunnel route for `finance.myankafe.com`, then run `scripts/deploy-production.sh` with `pool_max_conns=2`. Do not revoke `doadmin`; it is still required for this shared cluster.

## 2026-09-25 production deploy on the shared VPS

The section above describes the first pass, before database and R2 values were available. Finance was started afterward.

- API is healthy on `127.0.0.1:8180` and web answers on `127.0.0.1:3100`. Both binds are loopback only. MyanKafe and Royal Masterpiece stayed up.
- Database `myankafe-finance` is owned by `myankafe-finance-admin`. Goose is at version 13. `DATABASE_URL` uses `pool_max_conns=2`. `GOOSE_DATABASE_URL` omits pool parameters because Goose forwards them to PostgreSQL.
- The migrate image is built from `deploy/goose.Dockerfile` (Goose 3.24.3 release binary). The anonymous GHCR pull is denied.
- Migration `00013` widens `idempotency_records.scope` to `varchar(300)`. Two-ULID mutation URLs were returning HTTP 500.
- Financial-account codes are normalized on create: uppercase, whitespace collapsed to underscores.
- The initial OWNER username is `owner`. The temporary password is only in `/opt/myankafe-finance/.env.production` as `BOOTSTRAP_PASSWORD`. Remove it after the first human password change.
- Public routing is no longer a same-origin `/api/*` split. `finance.myankafe.com` goes to `127.0.0.1:3100`. `finance-api.myankafe.com` goes to `127.0.0.1:8180`. Both names are on MyanKafe tunnel `ae61f9d6-aace-42d5-8cd1-8397d9de1387`.
- `CORS_ORIGIN` remains `https://finance.myankafe.com`. `NEXT_PUBLIC_API_URL` is `https://finance-api.myankafe.com/api/v1`. Cookies stay Secure and SameSite=Strict. That works because both hostnames are the same site.
- The Finance R2 bucket accepts the dedicated token. The system probe and browser image/PDF checks passed. Royal Masterpiece R2 keys were not changed.
- `doadmin` and `myankafe-finance-admin` passwords were rotated. The new `doadmin` password is only in `/root/.secrets/doadmin-password` on the VPS. The application password is only in `/opt/myankafe-finance/.env.production`.
- No permanent owner password was supplied. `BOOTSTRAP_PASSWORD` is still in the server env file. The owner should sign in, change it, and then remove that line.
- `/health` and `/ready` on `finance-api.myankafe.com` are still reachable. `/metrics` returns 401 without the bearer token. The supplied R2 token cannot edit the remotely managed tunnel, so path exclusions were not added.
- Cross-currency transfer was not posted. The stored USD/MMK rate exists, and every financial account is MMK.
- The R2 system probe fails closed with AccessDenied on bucket `myankafe-finance`. The keys that work for the Royal Masterpiece buckets do not work for this bucket.
- A logical backup and a restore into a disposable database succeeded. The disposable database was dropped. Provider PITR was not confirmed in the console.
- Recommendation is **READY FOR FINAL HUMAN REVIEW**. PR #1 is Ready for Review and must not be merged until a person reviews it.

Evidence is in [docs/DEPLOYMENT_ACCEPTANCE_REPORT.md](DEPLOYMENT_ACCEPTANCE_REPORT.md).
