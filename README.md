# MyanKafe Finance

Private multi-entity, multi-currency double-entry finance platform for MyanKafe, Royal Masterpiece, Personal, and future entities.

## Stack

- Go 1.23 API (chi + pgx)
- PostgreSQL 17
- Goose migrations
- Next.js 15 + React 19 + TypeScript
- UUIDv7 internal relational IDs
- canonical ULID public IDs
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
- append-only audit history
- mutation idempotency using `Idempotency-Key`
- entity dashboard and all-entity management dashboard
- P&L, Balance Sheet, Trial Balance, General Ledger, Account Ledger API, cash movement, and inter-entity balances
- entity/user/role administration
- database-level entity-boundary and posted-immutability guards

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

```bash
cp .env.example .env
docker compose up -d db migrate
docker compose up -d api
```

Bootstrap the first owner and the three default entities:

```bash
docker compose run --rm \
  --entrypoint bootstrap \
  -e BOOTSTRAP_USERNAME=owner \
  -e BOOTSTRAP_DISPLAY_NAME="Owner" \
  -e BOOTSTRAP_ENTITIES=true \
  api
```

The command prints an `owner_public_id`. Put it in `.env` as:

```bash
NEXT_PUBLIC_DEV_USER_ULID=<owner-ulid>
```

Then start the web app:

```bash
docker compose up -d web
```

Open `http://localhost:3000`.

Development authentication uses:

```text
Authorization: Bearer dev:<USER_ULID>
```

`AUTH_MODE=dev` is rejected when `APP_ENV=production`. Production authentication must be wired before release.

## Validation

The draft feature PR runs GitHub Actions for:

- frontend TypeScript typecheck
- Next.js production build
- Go module resolution
- Go unit and PostgreSQL integration tests
- fresh Goose migration against PostgreSQL 17
- production API and non-root web Docker image builds

## Documentation

- [Architecture and invariants](docs/ARCHITECTURE.md)
- [API V1](docs/API_V1.md)
- [Cursor handoff](docs/CURSOR_HANDOFF.md)

## Before production merge

Cursor should commit generated dependency lock metadata (`backend/go.sum`, `frontend/package-lock.json`), run and commit `gofmt`, expand the remaining service-level integration tests listed in the handoff, replace development auth, and switch CI to strict non-mutating format/tidy checks.
