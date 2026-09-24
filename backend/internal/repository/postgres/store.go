package postgres

import (
  "context"
  "fmt"

  "github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string) (*Store, error) {
  p, err := pgxpool.New(ctx, dsn)
  if err != nil { return nil, err }
  if err := p.Ping(ctx); err != nil { p.Close(); return nil, fmt.Errorf("database ping: %w", err) }
  return &Store{Pool:p}, nil
}

func (s *Store) Close(){ s.Pool.Close() }
