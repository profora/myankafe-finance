# MyanKafe Finance overnight deployment and acceptance report

Date: 2026-09-25  
Recommendation: **READY FOR FINAL HUMAN REVIEW**

PR #1 was moved from Draft to Ready for Review after this continuation. It was not merged. No force-push was performed.

Finance is running on the shared VPS, bound to loopback only. Public traffic enters through the existing MyanKafe Cloudflare Tunnel on two hostnames. Existing MyanKafe and Royal Masterpiece services stayed healthy.

## Environment

```text
Hostname: rm-floral-platform (root@159.223.48.7)
Deployment path: /opt/myankafe-finance
Git branch: feat/v1-accounting-foundation
Git SHA: tip of feat/v1-accounting-foundation that contains migration 00013, the Goose image change, and this report
Docker version: 29.7.2
Compose version: v5.5.0
Reverse proxy: Cloudflare Tunnel (cloudflared 2026.8.2). Nothing listens on ports 80 or 443.
API loopback port: 127.0.0.1:8180 (healthy)
Web loopback port: 127.0.0.1:3100 (login returns 200)
```

Intended public URLs:

```text
https://finance.myankafe.com       -> http://127.0.0.1:3100
https://finance-api.myankafe.com   -> http://127.0.0.1:8180
```

This replaces the earlier same-origin plan that would have sent `/api/*` on `finance.myankafe.com` to the API. The browser calls `https://finance-api.myankafe.com/api/v1` directly. `CORS_ORIGIN` is `https://finance.myankafe.com`, not the API hostname. Cookies remain `HttpOnly`, `Secure`, and `SameSite=Strict`. Both hostnames share the registrable domain `myankafe.com`, so credentialed browser requests include that cookie. CORS is the exact frontend origin with credentials. It is not `*`.

### How the VPS is actually wired

| Existing service | Loopback | Public hostname | Tunnel |
| --- | --- | --- | --- |
| Royal Masterpiece API | `127.0.0.1:8080` | `api.royalmasterpiecefloral.com` | RM tunnel `751289aa-fa12-4b83-bbbd-70d5d72f2ef1` |
| Royal Masterpiece admin | `127.0.0.1:3000` | `admin.royalmasterpiecefloral.com` | same |
| Royal Masterpiece customer site | `127.0.0.1:3001` | `royalmasterpiecefloral.com` | same |
| MyanKafe API | `127.0.0.1:8090` | `api.myankafe.com` | MyanKafe tunnel `ae61f9d6-aace-42d5-8cd1-8397d9de1387` |
| MyanKafe admin | `127.0.0.1:3010` | `admin.myankafe.com` | same |
| MyanKafe legacy staff web | `127.0.0.1:3011` | `staff.myankafe.com` | same |
| MyanKafe customer web | `127.0.0.1:3012` | `web.myankafe.com` | same |
| Finance API | `127.0.0.1:8180` | `finance-api.myankafe.com` | MyanKafe tunnel |
| Finance web | `127.0.0.1:3100` | `finance.myankafe.com` | MyanKafe tunnel |

Both tunnels are remotely managed. No local proxy config was changed, and Caddy or Nginx was not installed. The MyanKafe tunnel already had these hostname routes when this continuation started:

```text
finance.myankafe.com       -> http://127.0.0.1:3100
finance-api.myankafe.com   -> http://127.0.0.1:8180
```

Existing MyanKafe and Royal Masterpiece hostnames were left as they were. `/health` and `/ready` on the API hostname still answer publicly. `/metrics` returns 401 without the bearer token. The supplied Finance token is not a Cloudflare API token that can edit the remotely managed tunnel, so path exclusions were not added. Loopback health checks still work.

Production env file is `/opt/myankafe-finance/.env.production`, mode `600`. It is not in Git. `APP_ENV=production`, `AUTH_MODE=password`, `AUTH_COOKIE_SECURE=true`, `CORS_ORIGIN=https://finance.myankafe.com`, and `NEXT_PUBLIC_API_URL=https://finance-api.myankafe.com/api/v1`. The web image was rebuilt so that API base is baked into the browser bundle. `ATTACHMENT_MAX_MB=20`. A metrics bearer token of 64 hex characters remains only in that file.

The first OWNER username is `owner`. No permanent password was supplied, so the temporary password is still `BOOTSTRAP_PASSWORD` in the server env file. Compose does not pass `BOOTSTRAP_*` into the running API. After the owner signs in and changes the password, remove `BOOTSTRAP_PASSWORD` from that file.

## Database

```text
Managed DB provider if identifiable: DigitalOcean Managed PostgreSQL (same cluster already used by MyanKafe and Royal Masterpiece)
DB host: private-rm-myankafe-db-do-user-43112147-0.h.db.ondigitalocean.com
DB port: 25060
Private address: 10.104.0.3 (reachable from this VPS)
DB name: myankafe-finance
Application DB username: myankafe-finance-admin
SSL mode: sslmode=require (no provider CA file was present for verify-full)
Migration result: Goose versions 00001 through 00013 applied; goose_db_version max is 13
PITR status: not verified in the DigitalOcean console (login was required). Sibling runbooks treat this cluster's provider backups/PITR as the primary recovery mechanism.
Automated backup status: not verified in the provider console this session
Logical backup test: PASS. Custom-format dump /var/backups/myankafe-finance/myankafe-finance-20260925T085939Z.dump, mode restricted by the script umask, pg_restore --list succeeded.
Restore drill: PASS. Restored into disposable database myankafe_finance_restore_drill, which then showed goose version 13, 3 entities, and 13 transactions. That database was dropped. Production was not overwritten.
```

`doadmin` is not a superuser. It can create databases and roles. It remains the cluster administration role and was not revoked. The running API uses `myankafe-finance-admin`, not `doadmin`. `doadmin` is not in the Finance env file.

Both passwords that had been pasted into chat were rotated. The old `doadmin` password no longer authenticates. The new `doadmin` password is only in `/root/.secrets/doadmin-password` (mode 600). The new application password is only in `DATABASE_URL` and `GOOSE_DATABASE_URL`. After the API was recreated, `/ready`, login, entity list, and profit-and-loss all succeeded.

`DATABASE_URL` includes `pool_max_conns=2`. Goose rejects that parameter, so `GOOSE_DATABASE_URL` is the same DSN without pool settings. `max_connections` on the cluster is 25.

Host after the web rebuild and a build-cache prune: disk about 43 GiB used / 4.9 GiB free (90%).

## R2

```text
Endpoint hostname: dbc116a454dda65b1ab21ad7744e9473.r2.cloudflarestorage.com
Bucket: myankafe-finance (R2_BUCKET; the endpoint has no bucket path, so object keys are not double-prefixed)
Configured: yes, backend-only, in the server env file, using the dedicated Finance key
System probe: PASS (Settings → System, write/read/verify/delete, 772.9 ms in the browser)
Image upload: PASS
PDF upload: PASS
Preview: PASS
Delete: PASS
```

The bucket was not made public. Royal Masterpiece R2 keys were not changed. Browser responses for the probe, upload, and image content did not contain the R2 secret. Authenticated image content returned `image/png` with the PNG signature.

## Acceptance matrix

| Area | Result |
| --- | --- |
| Authentication | PASS |
| Sessions | PASS |
| Entity switching | PASS |
| Dashboard | PASS |
| Chart of Accounts | PASS |
| Financial Accounts | PASS |
| Contacts | PASS |
| Income | PASS |
| Expense | PASS |
| Split transactions | PASS |
| Draft editing | PASS |
| Draft cancellation | PASS |
| Attachments | PASS |
| Transfers | PASS |
| Cross-currency transfer | BLOCKED |
| Manual journal | PASS |
| Exchange rates | PASS |
| Inter-entity | PASS |
| Accounting locking | PASS |
| OWNER unlock | PASS |
| Posted reversal | PASS |
| Transaction search/filter | PASS |
| P&L | PASS |
| Balance Sheet | PASS |
| Trial Balance | PASS |
| General Ledger | PASS |
| Account Ledger | PASS |
| Cash Movement | PASS |
| Inter-Entity Balances | PASS |
| CSV exports | PASS |
| Audit log | PASS |
| User management | PASS |
| Role enforcement | PASS |
| Desktop responsive QA | PASS |
| Tablet responsive QA | PASS |
| Mobile responsive QA | PASS |
| Keyboard accessibility | PASS |
| R2 acceptance | PASS |
| Backup | PASS |
| Restore drill | PASS |
| Metrics protection | PASS |
| Public HTTPS | PASS |
| Existing VPS services unaffected | PASS |

Public browser checks on `https://finance.myankafe.com` used the secure cookie. Login reached the dashboard. `/auth/me` on `https://finance-api.myankafe.com` returned 200 with credentials. The session cookie is not visible to JavaScript. A public login response set `HttpOnly`, `Secure`, and `SameSite=Strict`, with `Access-Control-Allow-Origin: https://finance.myankafe.com` and `Access-Control-Allow-Credentials: true`. Refresh stayed signed in. Logout returned the browser to `/login`, and a later visit to `/reports` was sent back to `/login`. API calls in the built bundle use `https://finance-api.myankafe.com/api/v1`.

Dashboard entity switch changed MyanKafe's current-month income of 1,600 MMK to Royal Masterpiece's 0 income and 80 MMK expenses. Cash balances changed from the MyanKafe TEST accounts to `TEST RM Cash`. The combined section stays labeled as all entities.

Transaction search for Packaging, type EXPENSE, status POSTED, and 2026-09-01 through 2026-09-30 returned one MyanKafe row. Switching to Royal Masterpiece cleared that row and listed `TEST RM Cash` instead of the MyanKafe financial accounts. MyanKafe CSV export contained Packaging. Royal Masterpiece CSV export did not. Report date range 2026-01-01 through 2026-01-31 showed the empty-state copy instead of the September TEST rows.

Attachment checks on posted `TEST Packaging Expense`: image and PDF upload, thumbnail, fullscreen image zoom to 125%, ArrowRight to the PDF preview, Escape and Close, reopen, reorder, and removal. The posted transaction remained POSTED after the attachments were removed.

Page overflow was 0 at 1440×900, 768×1024, 390×844, 375, and 430. Wide tables scroll inside the table card. Skip-to-content, visible focus styles, labelled dialogs, and attachment keyboard controls are present. The Chieftain logo was the brand asset at that time; the current brand is the MyanKafe logo described in the later section.

A USD/MMK rate of 4500 is stored. Every financial account is MMK, so a cross-currency transfer was not posted.

## Bugs discovered

1. The anonymous GHCR Goose image pull is denied. Production migrate now builds `deploy/goose.Dockerfile` from the Goose 3.24.3 release binary, checksummed for amd64 and arm64.
2. Goose forwards `DATABASE_URL` to PostgreSQL, which rejects `pool_max_conns`. Migrate uses `GOOSE_DATABASE_URL` without pool parameters. The API keeps `pool_max_conns=2`.
3. `idempotency_records.scope` was `varchar(120)`. Entity URLs with two ULIDs overflowed and returned HTTP 500 on draft cancel, reversal, and inter-entity mapping. Migration `00013` widens the column to `varchar(300)`. Those operations then succeeded. Already-applied migrations were not edited.
4. The API stored financial-account codes with spaces. `NormalizeFinancialAccountCode` now uppercases and collapses whitespace to underscores. A live create of `test  spaced wallet` stored `TEST_SPACED_WALLET`. An earlier TEST cash-box code still contains a space because it was created before the rebuild. It was left in place because posted TEST activity may reference it.

## Remaining blockers

### BLOCKER

None for the ready criterion. Public HTTPS, browser login, CORS, the Finance R2 probe, image and PDF attachment checks, dashboard, transactions, responsive QA, keyboard QA, and the existing VPS applications passed.

### IMPORTANT FOLLOW-UP

- `/health` and `/ready` on `https://finance-api.myankafe.com` return 200. `/metrics` returns 401 without the bearer token. Hiding health and readiness needs a tunnel path rule. The supplied Finance token cannot edit the remotely managed tunnel, and the tunnel was not converted to a local config.
- Confirm DigitalOcean automated backups and PITR for cluster `rm-myankafe-db` in the control panel. The logical dump is a secondary copy.
- Sign in as `owner`, change the temporary password, and remove `BOOTSTRAP_PASSWORD` from `/opt/myankafe-finance/.env.production`. No permanent password was supplied in this run.
- Store the rotated `doadmin` password from `/root/.secrets/doadmin-password` in the owner's password manager. Do not revoke the role.
- Keep `pool_max_conns=2`. The shared cluster allows 25 connections.
- Disk is about 90% used with 4.9 GiB free after build-cache cleanup. Do not delete the Finance images or the pre-existing PostgreSQL images to make room.

### OPTIONAL IMPROVEMENT

- Add a USD financial account and post a TEST transfer against the stored 4500 rate.
- This droplet has no Prometheus or Grafana for Finance. `/metrics` stays bearer-protected.

## Credentials to rotate

Names only. Values are not recorded here.

- DigitalOcean role `doadmin`: rotated. The new password is only in `/root/.secrets/doadmin-password` on the VPS. Do not revoke or delete the role.
- Database role `myankafe-finance-admin`: rotated. `DATABASE_URL` and `GOOSE_DATABASE_URL` were updated and the API was recreated.
- Application user `owner`: temporary password remains `BOOTSTRAP_PASSWORD` in the server env file. Change it on first human sign-in, then delete that line.
- `METRICS_BEARER_TOKEN`: generated on the server and not printed. Leave it unless it is exposed.
- Royal Masterpiece R2 keys: not rotated. Finance uses its own key.

## TEST data left in place

Posted accounting rows were not deleted. Remaining records include TEST chart accounts, financial accounts (including one pre-fix code that still contains a space, and `TEST_SPACED_WALLET`), contacts TEST Customer and TEST Supplier, posted and voided TEST transactions, a TEST inter-entity expense, and a USD/MMK rate of 4500 dated 2026-09-25. Users `test-admin`, `test-accountant`, `test-bookkeeper`, `test-viewer`, and `test-session-rotate` remain. The TEST image and PDF attachments added to `TEST Packaging Expense` were removed. The transaction stayed POSTED. The accounting period lock used for the earlier test was unlocked again.

## Security checks

- No secrets were committed. `.env.production` is untracked and mode 600.
- Finance binds `127.0.0.1` only. Ports 80 and 443 are not listening on the VPS.
- The database is the existing private managed host. No database port was published.
- Production startup validation was not bypassed: password auth, secure cookies, exact HTTPS CORS origin, HTTPS R2 endpoint, metrics token length.
- Debug auth was not enabled.
- R2 credentials are only in the server env.
- `/metrics` requires the bearer token. Public unauthenticated `/metrics` returned 401.

## Existing services after this session

```text
https://finance.myankafe.com/login          200
https://finance-api.myankafe.com/api/v1     reachable
https://web.myankafe.com                    200
https://api.myankafe.com/healthz            200
https://admin.myankafe.com                  307
https://royalmasterpiecefloral.com          200
http://127.0.0.1:8180/ready                 200
http://127.0.0.1:8080/healthz               200
http://127.0.0.1:8090/healthz               200
```

No debug container was left running.

## Final recommendation

**READY FOR FINAL HUMAN REVIEW**

Public HTTPS, split-hostname browser login, CORS with credentials, the Finance R2 probe, image and PDF attachment checks, dashboard and transaction browser checks, responsive QA, and the existing VPS applications passed. The 2026-09-26 configuration round below is included. Do not merge PR #1 until a person reviews it.

The owner still needs to change the temporary `owner` password and keep the rotated `doadmin` password. `/health` and `/ready` on the API hostname remain publicly reachable.

## 2026-09-26 configuration, branding, and navigation

Deployed commit `c1241d7202dbadb26b51b7cde02ba40dba7e5f51`. CI is green:

- push run 36220060219
- pull-request run 36220062232

Production Goose is at version 14. Migration `00014_currency_contact_types_transfer_fees.sql` applied at 2026-09-26 05:20:15 UTC. Finance API `/ready` and the web login page returned 200. MyanKafe and Royal Masterpiece `/healthz` stayed 200. Disk remained about 90% with 4.9 GiB free.

The Finance logo is the current MyanKafe admin brand, copied from `myankafe-platform` `admin/public/brand/logo-head.svg` and `logo-text.svg`. Those are the files the admin shell and login page render. `frontend/public/brand/chieftain-logo.webp` is no longer referenced by the UI and was left in the repository.

Browser checks on `https://finance.myankafe.com`:

- Login and sidebar show the MyanKafe mark with the words MyanKafe Finance.
- Desktop navigation is grouped: Overview, Transactions, Accounting Setup, Reports & Control, Settings.
- At 768px the drawer starts off-canvas and Open navigation shows the same groups, including Currencies and Contact Types.
- Page overflow was 0 at 1440, 768, and 390. The transaction table scrolls inside its card.
- Audit Action is a dropdown filled from `GET /audit-actions`, with All actions and exact action values.
- Transactions use page size 25/50/100 and show the range. MyanKafe currently has one page. The list has no Running net or Functional effect column. The summary says Posted income, Posted expenses, and Net.
- Currency create and unused delete were exercised. Deleting referenced MMK returned 400. Contact type create and unused delete were exercised. The five migrated types are present.
- A same-currency transfer with a fee was posted in the browser: transaction `01M3E3D64G0XQ69MM9W9ZN00C9`. The journal debits TEST Cash Box 299 MMK and TEST Packaging 1 MMK, and credits TEST Bank Account 300 MMK.

The earlier temporary UI password no longer signs in. It was reset on the server. The new value is only in `/root/.secrets/finance-ui-test-password`. It is not recorded here.

## 2026-09-26 movements, fiscal year, and inter-entity separation

Deployed application commit `2b36b7dd91d6f86cd11caeb079caef6e6edf2bbe`. CI is green:

- push run 36224772744
- pull-request run 36224776283

Goose reported no migrations to run. Current version is 14. Finance API `/ready` and the web login page returned 200 on loopback and on the public hostnames. MyanKafe `https://api.myankafe.com/healthz` and Royal Masterpiece `https://royalmasterpiecefloral.com/` returned 200. Disk remained about 90% with 4.9 GiB free after `docker builder prune`.

The API image includes the IANA timezone database. Saving entity timezone `Asia/Yangon` succeeded after this deploy. MyanKafe and Royal Masterpiece now start their fiscal year on April 1 and display `Apr 1 → Mar 31`. Personal remains `Jan 1 → Dec 31`.

Browser checks on `https://finance.myankafe.com` as the owner UI user:

- Sidebar groups are Overview, Transactions, Accounting Setup, Reports & Control, and Settings. Pay for Another Entity is under Transactions. Inter-Entity Setup is under Accounting Setup. The UUIDv7 / double-entry sidebar note is gone.
- Transactions show Date, Movement, Description, Account, Status, Amount, and Balance. The fee transfer `01M3E3D64G0XQ69MM9W9ZN00C9` is two rows: Transfer out TEST Bank Account −300 MMK balance 250 MMK, and Transfer in TEST Cash Box +299 MMK balance 1,404 MMK. There is no fee movement row.
- Filtering type Account Transfer and financial account TEST Bank Account returned only that account's rows, `1–3 of 3`, including Transfer out for the fee transfer and not the cash side.
- Page overflow was 0 at 1440, 768, and 390. At 768 and 390 the drawer starts off-canvas (`translateX(-294px)`).
- Dashboard fiscal year for MyanKafe reads Apr 1 → Mar 31. Selected-entity income, expenses, and net profit remain. Combined all-entities P&L remains. Financial Accounts lists MyanKafe and Royal Masterpiece accounts in MMK, not a mixed-currency total. TEST Bank Account fell from 250 MMK to 150 MMK after the payment below. Personal has no financial accounts, so it has no balance row.
- Inter-Entity Setup lists the MyanKafe ↔ Royal Masterpiece pair and its four due-from / due-to accounts. Pay for Another Entity does not show mapping controls. Personal shows that inter-entity accounting has not been set up, with Open Inter-Entity Setup for this owner.
- A same-currency payment was posted: MyanKafe pays 100 MMK from TEST Bank Account for Royal Masterpiece TEST RM Expense, description "Browser QA pay for Royal Masterpiece". Paying transaction `01M3E7H0S1ZS9JSRJ666PEKHQC` debits TEST Due From 100 and credits TEST Bank 100. Counterparty transaction `01M3E7H0S1RREDN3CAWTPST3BN` debits TEST RM Expense 100 and credits TEST RM Due To 100. The counterparty link switched entity context. Posted TEST rows were not deleted.

ACCOUNTANT, BOOKKEEPER, and VIEWER setup denial, payer-only posting denial, atomic pair save, and running-balance cases are covered by the Go integration tests in the green CI run. Those roles were not signed in through the browser.

Different-currency Pay for Another Entity was not posted. Every entity and financial account in this environment is MMK. The form uses one amount when functional currencies match.

PR #1 stays unmerged.
