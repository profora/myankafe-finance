package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type IdempotencyRecord struct {
	RequestHash    string
	ResponseStatus *int
	ResponseBody   *string
	ExpiresAt      *time.Time
}

func (s *Store) ClaimIdempotency(ctx context.Context, scope, key, requestHash string) (bool, IdempotencyRecord, error) {
	if scope == "" || key == "" || requestHash == "" {
		return false, IdempotencyRecord{}, fmt.Errorf("idempotency scope, key and request hash are required")
	}

	for attempt := 0; attempt < 2; attempt++ {
		id, err := ids.UUIDv7()
		if err != nil {
			return false, IdempotencyRecord{}, err
		}

		tag, err := s.Pool.Exec(ctx, `
INSERT INTO idempotency_records(id,scope,idempotency_key,request_hash,created_at,expires_at)
VALUES($1,$2,$3,$4,now(),now()+interval '24 hours')
ON CONFLICT(scope,idempotency_key) DO NOTHING`, id, scope, key, requestHash)
		if err != nil {
			return false, IdempotencyRecord{}, err
		}
		if tag.RowsAffected() == 1 {
			return true, IdempotencyRecord{RequestHash: requestHash}, nil
		}

		var rec IdempotencyRecord
		err = s.Pool.QueryRow(ctx, `
SELECT request_hash,response_status,response_body::text,expires_at
FROM idempotency_records
WHERE scope=$1 AND idempotency_key=$2`, scope, key).
			Scan(&rec.RequestHash, &rec.ResponseStatus, &rec.ResponseBody, &rec.ExpiresAt)
		if err != nil {
			return false, IdempotencyRecord{}, err
		}

		if rec.ExpiresAt != nil && rec.ExpiresAt.Before(time.Now().UTC()) {
			if _, err := s.Pool.Exec(ctx, `
DELETE FROM idempotency_records
WHERE scope=$1 AND idempotency_key=$2 AND expires_at < now()`, scope, key); err != nil {
				return false, IdempotencyRecord{}, err
			}
			continue
		}

		return false, rec, nil
	}

	return false, IdempotencyRecord{}, fmt.Errorf("could not claim expired idempotency key")
}

func (s *Store) CompleteIdempotency(ctx context.Context, scope, key string, status int, responseBody string) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE idempotency_records
SET response_status=$3,
    response_body=CASE WHEN $4='' THEN NULL ELSE $4::jsonb END,
    expires_at=now()+interval '24 hours'
WHERE scope=$1 AND idempotency_key=$2`, scope, key, status, responseBody)
	return err
}

func (s *Store) ReleaseIdempotency(ctx context.Context, scope, key string) error {
	_, err := s.Pool.Exec(ctx, `
DELETE FROM idempotency_records
WHERE scope=$1 AND idempotency_key=$2`, scope, key)
	return err
}
