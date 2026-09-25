# MyanKafe Finance overnight deployment and acceptance report

Date: 2026-09-25  
Recommendation: **NOT READY FOR FINAL HUMAN REVIEW**

PR #1 remains **Draft** and was not merged. No force-push was performed.

Finance is running on the shared VPS, bound to loopback only. Existing MyanKafe and Royal Masterpiece services stayed healthy. The public hostname and the Finance R2 bucket are not accepted yet, so this is not a production cutover.

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

Intended public URL remains `https://finance.myankafe.com`, with the API on the same origin at `/api/v1/...`. `finance.myankafe.com` does not resolve (`DNS_NONE`).

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
| Finance API | `127.0.0.1:8180` | not published | not in either ingress |
| Finance web | `127.0.0.1:3100` | not published | not in either ingress |

Both tunnels are remotely managed. No local proxy config was changed. Adding Finance requires a new public hostname on the MyanKafe tunnel, with path rules in this order:

```text
finance.myankafe.com   /api/*   -> http://127.0.0.1:8180
finance.myankafe.com   (other)  -> http://127.0.0.1:3100
```

`/health`, `/ready`, and `/metrics` must stay off that public hostname. The browser calls `NEXT_PUBLIC_API_URL` directly; Next.js does not proxy `/api`.

Host after the image build and a build-cache prune: 1 vCPU, 1.9 GiB RAM, about 1.3 GiB available, swap in use, disk about 43 GiB used / 4.9 GiB free (90%). Live sibling apps are systemd/host processes. Finance containers are the only Compose services running.

Production env file is `/opt/myankafe-finance/.env.production`, mode `600`. It is not in Git. `APP_ENV=production`, `AUTH_MODE=password`, `AUTH_COOKIE_SECURE=true`, `CORS_ORIGIN=https://finance.myankafe.com`, and `NEXT_PUBLIC_API_URL=https://finance.myankafe.com/api/v1`. `ATTACHMENT_MAX_MB=20`. A metrics bearer token of 64 hex characters was generated on the server and stored only in that file.

The first OWNER was bootstrapped with the existing command. Username is `owner`. The temporary password is `BOOTSTRAP_PASSWORD` in the server env file. Compose does not pass `BOOTSTRAP_*` into the running API. After the owner signs in and changes the password, remove `BOOTSTRAP_PASSWORD` from that file.

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

`doadmin` is not a superuser. It can create databases and roles. It was used to set the Finance database owner to `myankafe-finance-admin` and to create and drop the disposable restore database. The running API uses `myankafe-finance-admin`, not `doadmin`. `doadmin` is not in the Finance env file.

`DATABASE_URL` includes `pool_max_conns=2`. Goose rejects that parameter, so `GOOSE_DATABASE_URL` is the same DSN without pool settings. `max_connections` on the cluster is 25.

## R2

```text
Endpoint hostname: dbc116a454dda65b1ab21ad7744e9473.r2.cloudflarestorage.com
Bucket: myankafe-finance (included in the endpoint path; R2_BUCKET is empty so object keys are not double-prefixed)
Configured: yes, backend-only, in the server env file
System probe: FAIL (HTTP 503; R2 PUT returned 403 AccessDenied)
Image upload: BLOCKED
PDF upload: BLOCKED
Preview: BLOCKED
Delete: BLOCKED
```

The Royal Masterpiece conversation-media and product-media key pairs can read their own buckets and both receive AccessDenied on `myankafe-finance`. The Finance probe fails closed. The bucket was not made public. A token scoped to this bucket is still required. Browser attachment checks were not run.

## Acceptance matrix

| Area | Result |
| --- | --- |
| Authentication | PASS |
| Sessions | PASS |
| Entity switching | PASS |
| Dashboard | BLOCKED |
| Chart of Accounts | PASS |
| Financial Accounts | PASS |
| Contacts | PASS |
| Income | PASS |
| Expense | PASS |
| Split transactions | PASS |
| Draft editing | PASS |
| Draft cancellation | PASS |
| Attachments | BLOCKED |
| Transfers | PASS |
| Cross-currency transfer | BLOCKED |
| Manual journal | PASS |
| Exchange rates | PASS |
| Inter-entity | PASS |
| Accounting locking | PASS |
| OWNER unlock | PASS |
| Posted reversal | PASS |
| Transaction search/filter | BLOCKED |
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
| Desktop responsive QA | BLOCKED |
| Tablet responsive QA | BLOCKED |
| Mobile responsive QA | BLOCKED |
| Keyboard accessibility | BLOCKED |
| R2 acceptance | FAIL |
| Backup | PASS |
| Restore drill | PASS |
| Metrics protection | PASS |
| Public HTTPS | BLOCKED |
| Existing VPS services unaffected | PASS |

Checked on the loopback API with bearer sessions, because a `Secure` cookie is not sent to `http://127.0.0.1`. Set-Cookie on login is HttpOnly and Secure. Login uses the same error for an unknown user and a wrong password. Logout is 204 and the next `/auth/me` is 401. `/metrics` is 401 without the bearer token and 200 with it. The token was not printed.

Three bootstrap entities exist: `MYANKAFE`, `ROYAL_MASTERPIECE`, and `PERSONAL`. Using another entity's account on a post is rejected. A same-account transfer is rejected. An unbalanced journal is rejected. A balanced journal posts. A simple expense journal is balanced. Draft save, edit, and post work, and a later edit of the posted transaction is rejected. Draft cancel sets `VOIDED` and does not hard-delete. Reversal leaves the original amounts, marks the original transaction `VOIDED` and its journal `REVERSED`, and posts a separate reversal transaction. A lock through 2026-01-31 blocks a 2026-01-15 entry. An accountant cannot unlock. The owner can. A viewer cannot write. A bookkeeper can create a draft. User list payloads do not include password hashes. A transaction CSV for the active entity returned 200 and contained TEST rows. Account ledger for a TEST account returned 200.

Two sessions for user `test-session-rotate` were created. Revoking the other session returned `revoked: 1`; that session then received 401 and the current session stayed 200. Changing that user's password invalidated the old password and the previous session. The new password logged in. The owner password was not changed.

A USD to MMK rate of 4500 was stored with source `MANUAL`. A cross-currency transfer was not posted. Dashboard, search, and filter screens were not exercised in a browser. Report endpoints returned 200; a full numeric tie-out of every report was not written down.

## Bugs discovered

1. The anonymous GHCR Goose image pull is denied. Production migrate now builds `deploy/goose.Dockerfile` from the Goose 3.24.3 release binary, checksummed for amd64 and arm64.
2. Goose forwards `DATABASE_URL` to PostgreSQL, which rejects `pool_max_conns`. Migrate uses `GOOSE_DATABASE_URL` without pool parameters. The API keeps `pool_max_conns=2`.
3. `idempotency_records.scope` was `varchar(120)`. Entity URLs with two ULIDs overflowed and returned HTTP 500 on draft cancel, reversal, and inter-entity mapping. Migration `00013` widens the column to `varchar(300)`. Those operations then succeeded. Already-applied migrations were not edited.
4. The API stored financial-account codes with spaces. `NormalizeFinancialAccountCode` now uppercases and collapses whitespace to underscores. A live create of `test  spaced wallet` stored `TEST_SPACED_WALLET`. An earlier TEST cash-box code still contains a space because it was created before the rebuild. It was left in place because posted TEST activity may reference it.

## Remaining blockers

### BLOCKER

- `finance.myankafe.com` does not resolve, and MyanKafe tunnel `ae61f9d6-aace-42d5-8cd1-8397d9de1387` has no ingress for it. Public HTTPS, secure-cookie browser login, and same-origin `/api` routing were not verified. No Cloudflare API token was available, and the remotely managed tunnel was not converted to a local config.
- The Finance R2 bucket denies the keys that work for the Royal Masterpiece buckets. The system probe stays 503. Image and PDF attachment acceptance was not run.

### IMPORTANT FOLLOW-UP

- Add the path-split tunnel route above. Do not publish `/health`, `/ready`, or `/metrics`. Confirm the Cloudflare request-body limit is above 25 MB.
- Issue an R2 token that can write, read, and delete objects in `myankafe-finance`, update only the server env file, recreate the API, and rerun the system probe plus a real image and PDF attachment.
- Confirm DigitalOcean automated backups and PITR for cluster `rm-myankafe-db` in the control panel. The logical dump is a secondary copy.
- Sign in as `owner`, change the temporary password, and remove `BOOTSTRAP_PASSWORD` from `/opt/myankafe-finance/.env.production`.
- Rotate the `doadmin` password and the `myankafe-finance-admin` password, because both were pasted into chat. After rotating the application password, update the server env and recreate the API. Do not revoke the `doadmin` role.
- Keep `pool_max_conns=2`. The shared cluster allows 25 connections.
- Disk was at 95% after the image build. Unused build cache was pruned and free space returned to about 4.9 GiB. Do not delete the Finance images or the pre-existing PostgreSQL images to make room.

### OPTIONAL IMPROVEMENT

- Document the Cloudflare Tunnel path split beside `deploy/Caddyfile.example`.
- Finish the cross-currency transfer against the stored 4500 rate, and do a numeric tie-out of the TEST profit-and-loss figures.
- Browser QA at 1440x900, 768x1024, 390x844, and nearby 375 and 430 widths, plus keyboard access, after the hostname is live.
- This droplet has no Prometheus or Grafana for Finance. `/metrics` stays on loopback.

## Credentials to rotate

Names only. Values are not recorded here.

- DigitalOcean role `doadmin`: rotate the password. Do not revoke or delete the role. It remains the only role that can create databases on this cluster. It is not the Finance runtime login.
- Database role `myankafe-finance-admin`: rotate the password, then update `/opt/myankafe-finance/.env.production` (`DATABASE_URL` and `GOOSE_DATABASE_URL`) and recreate the API.
- Application user `owner`: change the temporary password on first sign-in, then delete `BOOTSTRAP_PASSWORD` from the server env file. The password is only in that file.
- `METRICS_BEARER_TOKEN`: generated on the server and not printed. Leave it unless it is exposed.
- Royal Masterpiece R2 keys: do not rotate them because of this deploy. They do not grant access to bucket `myankafe-finance`. Create a separate token for that bucket.

## TEST data left in place

Posted accounting rows were not deleted. Remaining records include TEST chart accounts, financial accounts (including one pre-fix code that still contains a space, and `TEST_SPACED_WALLET`), contacts TEST Customer and TEST Supplier, posted and voided TEST transactions, a TEST inter-entity expense, a USD/MMK rate of 4500 dated 2026-09-25, and users `test-admin`, `test-accountant`, `test-bookkeeper`, `test-viewer`, and `test-session-rotate`. The accounting period lock used for the test was unlocked again. Passwords for the TEST users were generated during the run and were not stored.

## Security checks

- No secrets were committed. `.env.production` is untracked.
- Finance binds `127.0.0.1` only. Ports 80 and 443 are not listening.
- The database is the existing private managed host. No database port was published.
- Production startup validation was not bypassed: password auth, secure cookies, exact HTTPS CORS origin, HTTPS R2 endpoint, metrics token length.
- Debug auth was not enabled.
- R2 credentials are only in the server env. They did not appear in the probe error body beyond the provider AccessDenied XML.
- `/metrics` requires the bearer token.

## Existing services after this session

```text
http://127.0.0.1:8180/ready             200
http://127.0.0.1:8180/health            200
http://127.0.0.1:3100/login             200
http://127.0.0.1:8080/healthz           200
http://127.0.0.1:8090/healthz           200
```

No debug container was left running. The Finance API container was healthy after the code-normalization recreate.

## Final recommendation

**NOT READY FOR FINAL HUMAN REVIEW**

The API, web, migrations, owner bootstrap, accounting acceptance on loopback, logical backup, and disposable restore drill are in place, and the existing VPS applications were left running. Public DNS and HTTPS are not configured, and the Finance R2 bucket still denies the available keys, so attachment and browser acceptance are incomplete.

Do not merge PR #1.
