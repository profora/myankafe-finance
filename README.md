# MyanKafe Finance

Private multi-entity, multi-currency double-entry finance platform for MyanKafe, Royal Masterpiece, Personal, and future entities.

## Stack

- Go 1.23 API (chi + pgx)
- PostgreSQL 17
- Goose migrations
- Next.js 15 + React 19 + TypeScript
- UUIDv7 internal relational IDs
- canonical ULID public IDs
- Argon2id username/password authentication with opaque database-backed sessions
- Cloudflare R2 private attachment storage
- Docker Compose
- GitHub Actions CI

## Implemented V1 foundation

- independent entity books and per-entity roles
- hierarchical Chart of Accounts
- definable cash, bank, mobile-wallet, credit-card, and other financial accounts
- contacts/payees/payers
- income/expense entry with splits
- deterministic double-entry posting
- manual journals
- account transfers
- multi-currency FX snapshots
- atomic inter-entity due-to/due-from posting
- accounting period locking with OWNER-only unlock
- reversal-based correction of posted accounting
- append-only business/request/auth audit history
- mutation idempotency using `Idempotency-Key`
- username/password login, logout, password change, owner password reset, active-session listing/revocation and sign-out-other-devices
- entity dashboard and all-entity management dashboard
- searchable/filterable transaction list with functional-currency summaries and filtered running net
- multi-action New Transaction menu for income, expense, transfer, manual journal, and inter-entity entry
- transaction/journal detail inspection and account-ledger drill-down
- editable income/expense drafts with audited cancellation; locked-period drafts cannot be created, re-dated, or cancelled
- multiple private R2 attachments per transaction
- authenticated streaming attachment preview; fullscreen image viewer with zoom, pan and keyboard navigation; PDF/text/CSV inline preview
- P&L, Balance Sheet, Trial Balance, General Ledger, Account Ledger, cash movement, and inter-entity balances
- entity-timezone-aware date defaults
- entity/user/role administration
- database-level entity-boundary, lifecycle, lock, FX, and posted-immutability guards

## Accounting invariants

- Every posted journal balances in entity functional currency.
- Posted accounting is immutable; corrections use reversals.
- Entity lock dates are enforced for posting and reversal.
- Only OWNER can unlock a period.
- Inter-entity posting is all-or-nothing in one PostgreSQL transaction.
- Used FX snapshots are immutable.
- Business audit events and accounting lock events are append-only.
- Public API routes use ULIDs; internal UUIDv7 values stay internal.
- PostgreSQL independently rejects cross-entity ledger wiring.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full rules.

## Local development

Copy the environment template and set a real development owner password:

```bash
cp .env.example .env
# Edit BOOTSTRAP_PASSWORD in .env; minimum 12 characters.
```

Start PostgreSQL and run migrations:

```bash
docker compose up -d db migrate
```

Bootstrap the first owner and the three default entities:

```bash
docker compose run --rm \
  --entrypoint bootstrap \
  -e BOOTSTRAP_USERNAME=owner \
  -e BOOTSTRAP_DISPLAY_NAME="Owner" \
  -e BOOTSTRAP_PASSWORD="$BOOTSTRAP_PASSWORD" \
  -e BOOTSTRAP_ENTITIES=true \
  api
```

Then start the application:

```bash
docker compose up -d api web
```

Open `http://localhost:3000/login` and sign in with the bootstrap username/password.

### R2 attachments

Configure a private Cloudflare R2 bucket in `.env`:

```text
R2_ENDPOINT=https://dbc116a454dda65b1ab21ad7744e9473.r2.cloudflarestorage.com/myankafe-finance
R2_BUCKET=
R2_REGION=auto
R2_ACCESS_KEY_ID=<access-key>
R2_SECRET_ACCESS_KEY=<secret>
ATTACHMENT_MAX_MB=20
```

R2 credentials remain backend-only. The browser uploads and previews attachments through authenticated finance API endpoints; it never receives R2 credentials.

If R2 is not configured, accounting still works, but attachment upload/content endpoints return service unavailable. Attachment metadata supports reorder and audited soft-removal; object deletion is best-effort after the database audit state is committed.

## Authentication

Normal mode is `AUTH_MODE=password`.

- passwords are Argon2id hashes
- raw session tokens are opaque random values
- only SHA-256 session-token hashes are stored
- web sessions use HttpOnly cookies
- Bearer session tokens are supported for future native clients
- changing a password revokes old sessions
- OWNER password reset revokes all sessions for the target user
- production refuses `AUTH_MODE=dev`
- production requires secure cookies

The legacy development ULID bearer path exists only when `AUTH_MODE=dev` is explicitly selected; the web UI no longer uses it.

## Branding

The web app uses the Chieftain Chin Coffee logo supplied for MyanKafe Finance at `frontend/public/brand/chieftain-logo.webp` on login, sidebar, and app metadata/icon surfaces.

## Operations

- `GET /health` is a lightweight liveness endpoint.
- `GET /ready` verifies PostgreSQL reachability and reports whether attachment storage is configured.
- Docker Compose waits for API readiness before starting the web container.

## Validation

The draft feature PR validates:

- fail-on-diff Go formatting and module tidiness
- committed dependency lockfiles with `npm ci`
- frontend TypeScript typecheck
- Next.js production build
- Go module resolution
- fresh Goose migration against PostgreSQL 17
- Go unit and PostgreSQL integration tests
- production API Docker image build
- production non-root web Docker image build

Tests cover core ledger invariants, authorization, idempotency, reversal, inter-entity atomicity, transaction running totals, attachment metadata ordering, password hashing/session tokens, and R2 signing helpers.

## Documentation

- [Architecture and invariants](docs/ARCHITECTURE.md)
- [API V1](docs/API_V1.md)
- [Cursor handoff](docs/CURSOR_HANDOFF.md)
- [Production deployment runbook](docs/DEPLOYMENT.md)

## Before production merge

Cursor should still:
- validate against the real production R2 bucket and credentials
- configure production secrets, TLS/reverse proxy, PostgreSQL backups/PITR, and observability
- perform final responsive/accessibility/browser polish and deployment validation
