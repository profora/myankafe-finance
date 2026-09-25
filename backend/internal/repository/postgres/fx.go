package postgres

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func (s *Store) ListExchangeRates(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT er.public_id::text,er.rate_date::text,er.from_currency_code,er.to_currency_code,
       er.rate::text,er.source,er.source_reference,er.created_at,
       EXISTS(
         SELECT 1
         FROM journal_lines jl
         JOIN journal_entries je ON je.id=jl.journal_entry_id
         WHERE jl.exchange_rate_id=er.id AND je.status IN ('POSTED','REVERSED')
       ) used
FROM exchange_rates er
WHERE er.entity_id=$1
ORDER BY er.rate_date DESC,er.created_at DESC
LIMIT 200`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, date, from, to, rate, source string
		var ref *string
		var created time.Time
		var used bool
		if err := rows.Scan(&id, &date, &from, &to, &rate, &source, &ref, &created, &used); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "rate_date": date, "from_currency": from, "to_currency": to, "rate": rate,
			"source": source, "source_reference": ref, "created_at": created, "used": used,
		})
	}
	return out, rows.Err()
}

func validateExchangeRateInput(date, from, to, rate, source string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("invalid rate date")
	}
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if len(from) != 3 || len(to) != 3 || from == to {
		return fmt.Errorf("invalid currency pair")
	}
	r, ok := new(big.Rat).SetString(strings.TrimSpace(rate))
	if !ok || r.Sign() <= 0 {
		return fmt.Errorf("rate must be greater than zero")
	}
	switch source {
	case "MANUAL", "INTEGRATION", "SYSTEM":
	default:
		return fmt.Errorf("invalid exchange rate source")
	}
	return nil
}

func (s *Store) CreateExchangeRate(ctx context.Context, user User, e Entity, date, from, to, rate, source, reference string) (map[string]any, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if source == "" {
		source = "MANUAL"
	}
	source = strings.ToUpper(strings.TrimSpace(source))
	if err := validateExchangeRateInput(date, from, to, rate, source); err != nil {
		return nil, err
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	_, err := s.Pool.Exec(ctx, `INSERT INTO exchange_rates(id,public_id,entity_id,rate_date,from_currency_code,to_currency_code,rate,source,source_reference,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10)`, id, pub, e.ID, date, from, to, rate, source, strings.TrimSpace(reference), user.ID)
	if err != nil {
		return nil, err
	}
	_ = s.Audit(ctx, user, &e, "EXCHANGE_RATE_CREATE", "EXCHANGE_RATE", &pub, "SUCCESS", map[string]any{"rate_date": date, "from": from, "to": to, "rate": rate})
	return map[string]any{"id": pub, "rate_date": date, "from_currency": from, "to_currency": to, "rate": rate, "source": source, "source_reference": reference, "used": false}, nil
}

type UpdateExchangeRateInput struct {
	RateDate        string
	FromCurrency    string
	ToCurrency      string
	Rate            string
	SourceReference string
}

func (s *Store) UpdateExchangeRate(ctx context.Context, user User, e Entity, publicID string, in UpdateExchangeRateInput) (map[string]any, error) {
	in.FromCurrency = strings.ToUpper(strings.TrimSpace(in.FromCurrency))
	in.ToCurrency = strings.ToUpper(strings.TrimSpace(in.ToCurrency))
	if err := validateExchangeRateInput(in.RateDate, in.FromCurrency, in.ToCurrency, in.Rate, "MANUAL"); err != nil {
		return nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var source string
	if err := tx.QueryRow(ctx, `
SELECT source
FROM exchange_rates
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`, e.ID, publicID).Scan(&source); err != nil {
		return nil, err
	}
	if source != "MANUAL" {
		return nil, fmt.Errorf("only MANUAL exchange rates can be edited")
	}

	if _, err := tx.Exec(ctx, `
UPDATE exchange_rates
SET rate_date=$3,
    from_currency_code=$4,
    to_currency_code=$5,
    rate=$6,
    source_reference=NULLIF($7,'')
WHERE entity_id=$1 AND public_id=$2`,
		e.ID, publicID, in.RateDate, in.FromCurrency, in.ToCurrency, in.Rate, strings.TrimSpace(in.SourceReference)); err != nil {
		return nil, err
	}

	if err := insertAuditTx(ctx, tx, user, e, "EXCHANGE_RATE_UPDATE", "EXCHANGE_RATE", publicID, map[string]any{
		"rate_date": in.RateDate, "from": in.FromCurrency, "to": in.ToCurrency, "rate": in.Rate,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": publicID, "rate_date": in.RateDate, "from_currency": in.FromCurrency,
		"to_currency": in.ToCurrency, "rate": in.Rate, "source": "MANUAL",
		"source_reference": in.SourceReference, "used": false,
	}, nil
}

func (s *Store) DeleteExchangeRate(ctx context.Context, user User, e Entity, publicID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var source string
	if err := tx.QueryRow(ctx, `
SELECT source
FROM exchange_rates
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`, e.ID, publicID).Scan(&source); err != nil {
		return err
	}
	if source != "MANUAL" {
		return fmt.Errorf("only MANUAL exchange rates can be deleted")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM exchange_rates WHERE entity_id=$1 AND public_id=$2`, e.ID, publicID); err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, user, e, "EXCHANGE_RATE_DELETE", "EXCHANGE_RATE", publicID, map[string]any{"deleted": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
