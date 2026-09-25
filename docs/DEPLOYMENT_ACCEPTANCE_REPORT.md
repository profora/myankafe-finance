# MyanKafe Finance overnight deployment and acceptance report

Date: 2026-09-25  
Recommendation: **NOT READY FOR FINAL HUMAN REVIEW**

PR #1 remains **Draft** and was not merged. No force-push was performed.

Finance containers were not started. Existing MyanKafe and Royal Masterpiece services were inspected before and after this session and remained healthy. No reverse-proxy configuration was changed.

## Environment

```text
Hostname: rm-floral-platform (root@159.223.48.7)
Deployment path: /opt/myankafe-finance (source tree only; no production env file and no running containers)
Git branch: feat/v1-accounting-foundation
Git SHA: application inspected at 5323af47ae9049e22fff2dc552e5996c2f3dd898; this report is a later commit on the same branch
Docker version: 29.7.2
Compose version: v5.5.0
Reverse proxy: Cloudflare Tunnel (cloudflared 2026.8.2). Nothing listens on ports 80 or 443. Caddy and Nginx are not the public proxy.
API loopback port: 8180 planned and confirmed free; not bound
Web loopback port: 3100 planned and confirmed free; not bound
```

Intended public URL remains `https://finance.myankafe.com`, with the API on the same origin at `/api/v1/...`.

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

Both tunnels are remotely managed (`cloudflared tunnel run --token-file`). Ingress is not a local Caddyfile. Adding Finance requires a new public hostname on the MyanKafe tunnel, with path rules in this order:

```text
finance.myankafe.com   /api/*   -> http://127.0.0.1:8180
finance.myankafe.com   (other)  -> http://127.0.0.1:3100
```

`/health`, `/ready`, and `/metrics` must stay off that public hostname. The browser calls `NEXT_PUBLIC_API_URL` directly; Next.js does not proxy `/api`.

Host resources at inspection: 1 vCPU, 1.9 GiB RAM, about 1.1 GiB available, 405 MiB swap in use, disk 43 GiB used / 5.4 GiB free (89%). Docker had no running containers. The live apps are systemd/host processes. Building the Finance images on this droplet can pressure memory; do it only while watching the existing services.

## Database

```text
Managed DB provider if identifiable: DigitalOcean Managed PostgreSQL (same cluster already used by MyanKafe and Royal Masterpiece)
DB host: private-rm-myankafe-db-do-user-43112147-0.h.db.ondigitalocean.com
DB port: 25060
Private address: 10.104.0.3 (resolves and accepts connections from this VPS)
DB name: not created
Application DB username: not created
SSL mode: sibling applications use sslmode=require; no provider CA file was present to switch Finance to verify-full
Migration result: not run
PITR status: not verified in the DigitalOcean console (login was required). Sibling runbooks treat this cluster's provider backups/PITR as the primary recovery mechanism.
Automated backup status: not verified in the provider console this session
Logical backup test: not run; there is no Finance database to dump. Other applications' databases were not dumped.
Restore drill: not performed
```

Databases visible on the cluster: `_dodb`, `defaultdb`, `myankafe-db`, `rm-floral-db`. There is no Finance database.

Role facts, read with the existing MyanKafe application login:

| Role | Superuser | Create DB | Create role | Login |
| --- | --- | --- | --- | --- |
| `doadmin` | no | yes | yes | yes |
| `myankafe-admin` | no | no | no | yes |
| `rm-floral-admin` | no | no | no | yes |

`myankafe-admin` cannot create the Finance database or role. The `doadmin` password is not stored on the VPS, and it was not supplied for this session. No temporary administrator credential was used.

`max_connections` is 25. A point-in-time count showed about 23 sessions, including Royal Masterpiece (9), MyanKafe (2), and provider/system sessions. Finance must set `pool_max_conns=2` on its `DATABASE_URL`. The API uses pgx defaults, which would open up to 4 connections on this 1-CPU host.

Required setup, once `doadmin` is available, before the application is started:

1. Create a dedicated database, for example `myankafe_finance`. Do not migrate into `myankafe-db` or `rm-floral-db`.
2. Create a dedicated login role. Do not run Finance as `doadmin`, `myankafe-admin`, or `rm-floral-admin`.
3. Make that role the database owner so Goose can create tables, functions, and triggers.
4. Put only that role in `/opt/myankafe-finance/.env.production` with mode `600`, `sslmode=require`, and `pool_max_conns=2`.
5. Run `scripts/deploy-production.sh`. It migrates, starts the API, waits for `/ready`, and only then replaces web.

A disposable restore drill needs a second empty database created by `doadmin`. Restore the custom-format dump there, run `pg_restore --list` and a schema/row check, then drop only that disposable database.

## R2

```text
Endpoint hostname: not supplied. The repository example uses the same Cloudflare account as the existing tunnels (dbc116a454dda65b1ab21ad7744e9473.r2.cloudflarestorage.com).
Bucket: not supplied
Configured: FAIL
System probe: BLOCKED
Image upload: BLOCKED
PDF upload: BLOCKED
Preview: BLOCKED
Delete: BLOCKED
```

No R2 access key or secret was supplied, and the existing product-media keys were not copied. Credentials stay backend-only when they are added. The bucket must remain private.

## Acceptance matrix

| Area | Result |
| --- | --- |
| Authentication | BLOCKED |
| Sessions | BLOCKED |
| Entity switching | BLOCKED |
| Dashboard | BLOCKED |
| Chart of Accounts | BLOCKED |
| Financial Accounts | BLOCKED |
| Contacts | BLOCKED |
| Income | BLOCKED |
| Expense | BLOCKED |
| Split transactions | BLOCKED |
| Draft editing | BLOCKED |
| Draft cancellation | BLOCKED |
| Attachments | BLOCKED |
| Transfers | BLOCKED |
| Cross-currency transfer | BLOCKED |
| Manual journal | BLOCKED |
| Exchange rates | BLOCKED |
| Inter-entity | BLOCKED |
| Accounting locking | BLOCKED |
| OWNER unlock | BLOCKED |
| Posted reversal | BLOCKED |
| Transaction search/filter | BLOCKED |
| P&L | BLOCKED |
| Balance Sheet | BLOCKED |
| Trial Balance | BLOCKED |
| General Ledger | BLOCKED |
| Account Ledger | BLOCKED |
| Cash Movement | BLOCKED |
| Inter-Entity Balances | BLOCKED |
| CSV exports | BLOCKED |
| Audit log | BLOCKED |
| User management | BLOCKED |
| Role enforcement | BLOCKED |
| Desktop responsive QA | BLOCKED |
| Tablet responsive QA | BLOCKED |
| Mobile responsive QA | BLOCKED |
| Keyboard accessibility | BLOCKED |
| R2 acceptance | BLOCKED |
| Backup | BLOCKED |
| Restore drill | BLOCKED |
| Metrics protection | BLOCKED |
| Public HTTPS | BLOCKED |
| Existing VPS services unaffected | PASS |

GitHub CI for `5323af47ae9049e22fff2dc552e5996c2f3dd898` was green (backend, frontend, operations-config, containers) before this report. That is repository CI, not a deployed-environment acceptance pass.

Application workflows were not exercised because the API cannot start without `DATABASE_URL`, and no Finance database exists. No TEST accounting records were created. Nothing posted was deleted.

## Bugs discovered

No application defect was reproduced. The API was not started, so accounting, UI, attachment, and authorization behavior were not retested on this host.

Checked and not defective: Alpine 3.20 provides `/usr/bin/wget`, which the production health check and `scripts/deploy-production.sh` call. The temporary Alpine image used for that check was removed.

## Remaining blockers

### BLOCKER

- Managed PostgreSQL application database and least-privilege role were not created. `doadmin` is required and its password was not available.
- Owner-supplied values were empty: `DB_*`, `R2_*`, `OWNER_USERNAME`, `OWNER_DISPLAY_NAME`, and `OWNER_TEMP_PASSWORD`.
- `finance.myankafe.com` does not resolve. Neither Cloudflare Tunnel ingress contains that hostname. DigitalOcean and Cloudflare dashboards both required an interactive login, so DNS and the tunnel route were not created.
- Public HTTPS, secure-cookie login, and same-origin `/api` routing therefore could not be verified.

### IMPORTANT FOLLOW-UP

- Cap Finance at `pool_max_conns=2` before first start. The shared cluster allows 25 connections and was already near that ceiling.
- When the tunnel route is added, put `/api/*` ahead of the catch-all web route. Do not publish `/metrics`.
- Confirm the Cloudflare request-body limit is above 25 MB so a 20 MB attachment plus multipart overhead is accepted. The Caddy example is not what this droplet uses.
- Confirm DigitalOcean automated backups and PITR on cluster `rm-myankafe-db` in the control panel.
- Build images while watching memory. The droplet has 1.9 GiB RAM and 5.4 GiB free disk.
- Generate `METRICS_BEARER_TOKEN` with `openssl rand -hex 32` into the server env file only. Do not commit it.
- Bootstrap the first OWNER with `docker compose ... run --rm --entrypoint bootstrap api`, then remove `BOOTSTRAP_PASSWORD` from `.env.production`.
- After a real dump exists, restore it into a disposable database. Do not restore over `myankafe-db` or `rm-floral-db`.

### OPTIONAL IMPROVEMENT

- Document the Cloudflare Tunnel path split next to `deploy/Caddyfile.example`, so the next deploy does not assume Caddy owns ports 80/443.
- Ship API stdout JSON logs and scrape loopback `/metrics` only after a monitoring destination exists. This droplet has no Prometheus or Grafana for Finance.

## Credentials to rotate

No new credentials were created or written.

- Do **not** revoke the existing DigitalOcean `doadmin` role. It was not a one-time Finance credential, and it is the only role on this cluster that can create the Finance database and application role. No temporary administrator password was supplied, so there is nothing from this session to revoke.
- Do not reuse or rotate `myankafe-admin` or `rm-floral-admin` as part of Finance.
- R2 keys and the initial OWNER password were not installed.
- No metrics bearer token was generated, because there is no production env file yet.

## Security checks that were possible

- No secrets were committed.
- `.env.production` is not in Git.
- Finance did not publish a database port or an application port.
- Existing application ports stay on loopback, and the public path is Cloudflare Tunnel.
- Production config code still requires `AUTH_MODE=password`, `AUTH_COOKIE_SECURE=true`, an exact HTTPS `CORS_ORIGIN`, HTTPS when R2 is set, and a metrics token of at least 32 characters when one is set. Those checks were not bypassed.
- Debug auth was not enabled.

## Existing services after this session

```text
http://127.0.0.1:8080/healthz          200
http://127.0.0.1:8080/readyz           200
http://127.0.0.1:8090/healthz          200
http://127.0.0.1:8090/readyz           200
https://api.myankafe.com/healthz       200
https://admin.myankafe.com             307
https://web.myankafe.com               200
https://royalmasterpiecefloral.com     200
```

No Finance container, extra cloudflared process, or debug service was left running.

## Final recommendation

**NOT READY FOR FINAL HUMAN REVIEW**

The feature branch and CI were already in place, and the VPS can host Finance on free loopback ports without replacing the current proxy. Deployment and acceptance did not start because the Finance database cannot be created without `doadmin`, R2 and OWNER bootstrap values were not supplied, and `finance.myankafe.com` is not in DNS or the MyanKafe tunnel. Starting Finance on another application's database, or opening ports 80/443 beside the tunnels, would have been the wrong fix.

Do not merge PR #1.
