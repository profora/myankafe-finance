package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type SalesChannel struct {
	PublicID string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

type SalesChannelInput struct {
	Code   string
	Name   string
	Active bool
}

func NormalizeSalesChannelCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if b.Len() > 0 && !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" || len(out) > 40 {
		return "", fmt.Errorf("sales channel code is required")
	}
	return out, nil
}

func (s *Store) ListSalesChannels(ctx context.Context, entityID string) ([]SalesChannel, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT public_id::text,code,name,active
FROM sales_channels
WHERE entity_id=$1
ORDER BY active DESC,name,code`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SalesChannel{}
	for rows.Next() {
		var item SalesChannel
		if err := rows.Scan(&item.PublicID, &item.Code, &item.Name, &item.Active); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateSalesChannel(ctx context.Context, user User, e Entity, in SalesChannelInput) (SalesChannel, error) {
	code, err := NormalizeSalesChannelCode(in.Code)
	if err != nil {
		return SalesChannel{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return SalesChannel{}, fmt.Errorf("sales channel name is required")
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO sales_channels(id,public_id,entity_id,code,name,active,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7)`, id, pub, e.ID, code, name, in.Active, user.ID); err != nil {
		return SalesChannel{}, err
	}
	if err := s.Audit(ctx, user, &e, "SALES_CHANNEL_CREATE", "SALES_CHANNEL", &pub, "SUCCESS", map[string]any{
		"code": code, "name": name, "active": in.Active,
	}); err != nil {
		return SalesChannel{}, err
	}
	return SalesChannel{PublicID: pub, Code: code, Name: name, Active: in.Active}, nil
}

func (s *Store) UpdateSalesChannel(ctx context.Context, user User, e Entity, code, name string, active bool) (SalesChannel, error) {
	normalized, err := NormalizeSalesChannelCode(code)
	if err != nil {
		return SalesChannel{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return SalesChannel{}, fmt.Errorf("sales channel name is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return SalesChannel{}, err
	}
	defer tx.Rollback(ctx)

	var id, pub string
	if err := tx.QueryRow(ctx, `
SELECT id::text,public_id::text
FROM sales_channels
WHERE entity_id=$1 AND code=$2
FOR UPDATE`, e.ID, normalized).Scan(&id, &pub); err != nil {
		if err == pgx.ErrNoRows {
			return SalesChannel{}, fmt.Errorf("unknown sales channel")
		}
		return SalesChannel{}, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE sales_channels
SET name=$3,active=$4,updated_at=now()
WHERE id=$1 AND entity_id=$2`, id, e.ID, name, active); err != nil {
		return SalesChannel{}, err
	}
	if err := insertAuditTx(ctx, tx, user, e, "SALES_CHANNEL_UPDATE", "SALES_CHANNEL", pub, map[string]any{
		"code": normalized, "name": name, "active": active,
	}); err != nil {
		return SalesChannel{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SalesChannel{}, err
	}
	return SalesChannel{PublicID: pub, Code: normalized, Name: name, Active: active}, nil
}

func resolveSalesChannelTx(ctx context.Context, tx pgx.Tx, entityID, publicID string) (any, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, nil
	}
	var id string
	var active bool
	if err := tx.QueryRow(ctx, `
SELECT id::text,active
FROM sales_channels
WHERE entity_id=$1 AND public_id=$2`, entityID, publicID).Scan(&id, &active); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("unknown sales channel")
		}
		return nil, err
	}
	if !active {
		return nil, fmt.Errorf("sales channel is inactive")
	}
	return id, nil
}

func (s *Store) SalesAnalysis(ctx context.Context, entityID string, from, to time.Time) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT a.public_id::text,a.code,a.name,
       COALESCE(sc.code,'UNCLASSIFIED') channel_code,
       COALESCE(sc.name,'Unclassified') channel_name,
       COALESCE(c.customer_segment,'') customer_segment,
       CASE
         WHEN c.customer_segment='RETAILER' THEN 'RETAILER'
         WHEN c.customer_segment='DISTRIBUTOR' THEN 'DISTRIBUTOR'
         ELSE COALESCE(sc.code,'UNCLASSIFIED')
       END route_code,
       CASE
         WHEN c.customer_segment='RETAILER' THEN 'Retailer'
         WHEN c.customer_segment='DISTRIBUTOR' THEN 'Distributor'
         ELSE COALESCE(sc.name,'Unclassified')
       END route_name,
       COALESCE(SUM(jl.credit_amount-jl.debit_amount),0)::text amount
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN accounts a ON a.id=jl.account_id
JOIN transactions t ON t.id=je.transaction_id
LEFT JOIN transactions ot ON ot.id=t.original_transaction_id
LEFT JOIN contacts c ON c.id=COALESCE(t.contact_id,ot.contact_id)
LEFT JOIN sales_channels sc ON sc.id=COALESCE(t.sales_channel_id,ot.sales_channel_id)
WHERE je.entity_id=$1
  AND je.status IN ('POSTED','REVERSED')
  AND je.journal_date BETWEEN $2 AND $3
  AND a.account_type='INCOME'
GROUP BY a.id,a.public_id,a.code,a.name,sc.code,sc.name,c.customer_segment
HAVING COALESCE(SUM(jl.credit_amount-jl.debit_amount),0)<>0
ORDER BY a.code,route_name,channel_name,customer_segment`, entityID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var accountID, code, name, channelCode, channelName, segment, routeCode, routeName, amount string
		if err := rows.Scan(&accountID, &code, &name, &channelCode, &channelName, &segment, &routeCode, &routeName, &amount); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"account_id": accountID, "account_code": code, "account_name": name,
			"sales_channel_code": channelCode, "sales_channel_name": channelName,
			"customer_segment": segment, "route_code": routeCode, "route_name": routeName,
			"amount": amount,
		})
	}
	return out, rows.Err()
}
