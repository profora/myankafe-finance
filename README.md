# MyanKafe Finance

Multi-entity, multi-currency double-entry finance platform for MyanKafe, Royal Masterpiece, Personal, and future entities.

## Stack
- Go 1.23 API (chi + pgx)
- PostgreSQL 17
- Goose migrations
- Next.js 15 + TypeScript
- UUIDv7 internal relational IDs
- ULID public IDs

## V1 invariants
- Every posted journal balances in entity functional currency.
- Posted accounting is immutable; corrections use reversals/adjustments.
- Entity lock dates are enforced in the Go posting boundary.
- Only OWNER can unlock a period.
- Inter-entity posting is atomic.
- Meaningful API actions create append-only audit events.
- Public routes use ULIDs; internal UUIDv7 values stay internal.

## Local development
```bash
cp .env.example .env
docker compose up -d db migrate
docker compose up -d api web
```

Development authentication is intentionally a temporary boundary and is rejected in production mode. Cursor/deployment handoff must wire production identity before release.
