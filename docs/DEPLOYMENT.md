# Production Deployment Runbook

This runbook deploys MyanKafe Finance without weakening any accounting invariant. Production uses an external/managed PostgreSQL database, loopback-only application ports, and a reverse proxy/TLS terminator such as Caddy.

## 1. Preconditions

- Linux host with Docker Engine + Docker Compose v2.
- Caddy (or equivalent reverse proxy) on the host.
- A dedicated PostgreSQL database and least-privilege application role.
- PostgreSQL provider backups/PITR enabled before live accounting data is entered.
- Private Cloudflare R2 bucket and scoped object credentials if attachments are enabled.
- DNS for the chosen Finance hostname pointed at the host.
- A long random `METRICS_BEARER_TOKEN` if metrics will be scraped.

Do not reuse another application's database, database role, cookie name, R2 credentials, or storage prefix.

## 2. Production environment

Copy the template on the server:

```bash
cp .env.production.example .env.production
chmod 600 .env.production
$EDITOR .env.production
```

Required production values include:

- `PUBLIC_ORIGIN` / `CORS_ORIGIN` — exact HTTPS Finance origin.
- `NEXT_PUBLIC_API_URL` — normally the same origin plus `/api/v1`.
- `DATABASE_URL` — production PostgreSQL with TLS required by the provider.
- `AUTH_COOKIE_SECURE=true`.
- R2 values when attachments are enabled.
- `METRICS_BEARER_TOKEN` when monitoring is enabled.

The API refuses production startup with development auth or insecure cookies.

## 3. Reverse proxy and upload size

Use `deploy/Caddyfile.example` as the host site template. The production Compose file binds API and web only to `127.0.0.1`; they should not be exposed directly to the Internet.

The reverse proxy body limit must be **larger than** `ATTACHMENT_MAX_MB` because multipart requests add overhead. With the default 20 MB attachment limit, the example uses 25 MB.

Only `/api/*` is proxied to the API. `/metrics`, `/ready`, and `/health` remain loopback-only by default.

## 4. Build, migrate, and start

From the checked-out release commit:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml config

docker compose --env-file .env.production -f docker-compose.prod.yml \
  up -d --build
```

The `migrate` service runs Goose first. The API starts only after migrations complete and the web service starts only after API readiness is healthy.

Verify locally on the host:

```bash
curl -fsS http://127.0.0.1:8180/health
curl -fsS http://127.0.0.1:8180/ready
curl -fsSI http://127.0.0.1:3100/login
```

Then verify the public HTTPS login and API through the reverse proxy.

## 5. First OWNER bootstrap

Only when no production credential exists yet:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml \
  run --rm --entrypoint bootstrap api
```

After the owner signs in successfully, remove `BOOTSTRAP_PASSWORD` from the production environment file. Do not create an unauthenticated bootstrap HTTP endpoint.

## 6. R2 acceptance

Before relying on attachments:

1. Sign in as an OWNER.
2. Open **Settings → System**.
3. Run the R2 write/read/delete acceptance probe.
4. Upload an image and a PDF to a test draft transaction.
5. Preview both through the authenticated Finance UI.
6. Remove the test attachments and confirm the metadata is soft-removed and object cleanup succeeds.
7. Confirm the reverse proxy accepts the configured maximum upload size.

R2 credentials remain backend-only.

## 7. Backups and PITR

Provider-managed PostgreSQL backups/PITR are the primary recovery mechanism. A logical dump is an additional portable backup, not a replacement for PITR.

The repository includes `scripts/postgres-backup.sh`. Run it from a secured host that has PostgreSQL client tools:

```bash
set -a
. ./.env.production
set +a
BACKUP_DIR=/srv/backups/myankafe-finance RETENTION_DAYS=14 \
  ./scripts/postgres-backup.sh
```

Store backups encrypted and off-host. Periodically restore a backup into a separate database and run `pg_restore --list` plus application smoke checks. A backup that has never been restore-tested is not considered verified.

## 8. Metrics and logs

- API logs are structured JSON and should be shipped from container stdout/stderr to the selected log platform.
- Scrape `http://127.0.0.1:8180/metrics` from the host/private monitoring network with:
  `Authorization: Bearer <METRICS_BEARER_TOKEN>`.
- Alert on sustained 5xx rate, readiness failure, abnormal latency, database capacity, and backup/PITR failures.
- Do not expose `/metrics` through the public reverse proxy.

## 9. Deployment sequence for later releases

1. Confirm CI is green for the exact commit.
2. Take/verify a recent database backup and provider PITR status.
3. Pull/checkout the exact release SHA.
4. Review pending migrations.
5. Run Compose build/start; migrations execute before API replacement.
6. Verify `/ready`, login, transaction list, one read-only report, and R2 diagnostics.
7. Record the deployed Git SHA.

Do not automatically run Goose `down` in production.

## 10. Rollback

Application rollback is allowed only when the previous application version is compatible with the already-applied schema. Prefer forward fixes for additive migrations.

If a release requires destructive schema rollback, stop writes and use a reviewed recovery plan or restore from verified PostgreSQL backup/PITR. Never mutate posted journals to make a software rollback easier.

## 11. Production acceptance checklist

- [ ] Exact production origin/CORS configured.
- [ ] Secure cookie enforced.
- [ ] Database uses TLS and dedicated credentials.
- [ ] Provider backups/PITR enabled and restore procedure tested.
- [ ] R2 write/read/delete probe passes.
- [ ] Image/PDF attachment preview passes at expected sizes.
- [ ] Reverse-proxy body limit exceeds attachment limit.
- [ ] API/web only bind to loopback.
- [ ] TLS certificate active.
- [ ] Metrics scrape authenticated and not public.
- [ ] Structured logs are collected.
- [ ] OWNER bootstrap secret removed after first use.
- [ ] CI green for deployed SHA.
- [ ] Deployed SHA recorded.
