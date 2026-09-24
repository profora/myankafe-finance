# MyanKafe Finance

Repository: `https://github.com/profora/myankafe-finance`

Multi-entity, multi-currency double-entry finance platform for MyanKafe, Royal Masterpiece, Personal, and future entities.

## Stack

- Go 1.23 API (chi + pgx)
- PostgreSQL 17
- Goose migrations
- sqlc query definitions (incremental adoption)
- Next.js 15 + TypeScript
- UUIDv7 internal relational IDs
- ULID public IDs

## Accounting invariants

- Posted journals must balance in entity functional currency.
- Posted journal lines are immutable; corrections use reversal/adjustment entries.
- Entity transaction locks are enforced by the Go posting boundary and serialized with PostgreSQL row locks.
- Only OWNER can unlock a locked entity period.
- Inter-entity entries post both sides in one database transaction.
- Every authenticated API action is audited, with financial mutation audit events committed atomically with the mutation.
- Internal UUIDs are not normal API identifiers; public routes use ULIDs.

## Local start

1. Copy `backend/.env.example` if running the API outside Docker.
2. Start PostgreSQL and migrations:

   ```bash
   docker compose up -d db migrate
   ```

3. Start the API:

   ```bash
   docker compose up -d api
   ```

4. Bootstrap the first owner and the three default entities:

   ```bash
   docker compose run --rm \
     -e BOOTSTRAP_USERNAME=owner \
     -e BOOTSTRAP_DISPLAY_NAME="Owner" \
     -e BOOTSTRAP_ENTITIES=true \
     api /app/bootstrap
   ```

   Copy the emitted `owner_public_id` into `NEXT_PUBLIC_DEV_USER_ULID` for local development.

5. Start the web app:

   ```bash
   NEXT_PUBLIC_DEV_USER_ULID=<owner-ulid> docker compose up -d web
   ```

Open `http://localhost:3000`.

`AUTH_MODE=dev` is rejected when `APP_ENV=production`. Production authentication must be replaced/wired to the MyanKafe identity boundary before deployment.

## Repository layout

```text
backend/
  cmd/api/                 HTTP API
  cmd/bootstrap/           first-owner bootstrap
  db/migrations/           append-only Goose migrations
  db/queries/              sqlc query definitions
  internal/accounting/     pure accounting rules
  internal/auth/           current auth boundary
  internal/httpapi/        REST handlers/middleware
  internal/repository/     PostgreSQL implementation
frontend/
  app/                     Next.js App Router pages
  components/
docs/                      architecture/database/API source of truth
scripts/                    validation helpers
```

## Before production

Cursor/deployment handoff should run the full PostgreSQL integration test suite, `go mod tidy`, `go test ./...`, `npm install`, `npm run typecheck`, `npm run build`, and a fresh-database Goose migration test. The current workspace cannot download external modules or start PostgreSQL, so those environment-dependent checks are intentionally left for the connected repository environment.
