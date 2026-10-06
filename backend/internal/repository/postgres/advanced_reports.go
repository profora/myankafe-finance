package postgres

import (
	"context"
	"time"
)

func (s *Store) BalanceSheet(ctx context.Context, entityID string, through time.Time) (map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT a.public_id::text,a.code,a.name,a.account_type,
CASE
 WHEN a.account_type='ASSET' THEN COALESCE(SUM(jl.debit_amount-jl.credit_amount),0)
 ELSE COALESCE(SUM(jl.credit_amount-jl.debit_amount),0)
END::text balance
FROM accounts a
JOIN journal_lines jl ON jl.account_id=a.id
JOIN journal_entries je ON je.id=jl.journal_entry_id
WHERE a.entity_id=$1
  AND a.account_type IN ('ASSET','LIABILITY','EQUITY')
  AND je.status IN ('POSTED','REVERSED')
  AND je.journal_date <= $2
GROUP BY a.id,a.public_id,a.code,a.name,a.account_type
HAVING COALESCE(SUM(jl.debit_amount-jl.credit_amount),0)<>0
ORDER BY a.account_type,a.code`, entityID, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, code, name, typ, balance string
		if err := rows.Scan(&id, &code, &name, &typ, &balance); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "code": code, "name": name, "type": typ, "balance": balance})
	}

	var currentEarnings string
	err = s.Pool.QueryRow(ctx, `
SELECT COALESCE(SUM(
 CASE
  WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount
  WHEN a.account_type='EXPENSE' THEN jl.credit_amount-jl.debit_amount
  ELSE 0
 END
),0)::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN accounts a ON a.id=jl.account_id
WHERE je.entity_id=$1 AND je.status IN ('POSTED','REVERSED') AND je.journal_date <= $2`, entityID, through).Scan(&currentEarnings)
	if err != nil {
		return nil, err
	}

	return map[string]any{"items": items, "current_earnings": currentEarnings, "through": through.Format("2006-01-02")}, nil
}

func (s *Store) GeneralLedger(ctx context.Context, entityID string, from, to time.Time, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := s.Pool.Query(ctx, `
SELECT je.public_id::text,je.journal_date::text,je.description,
       a.public_id::text,a.code,a.name,
       jl.line_no,COALESCE(jl.description,''),jl.debit_amount::text,jl.credit_amount::text,
       je.functional_currency_code,t.public_id::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN accounts a ON a.id=jl.account_id
LEFT JOIN transactions t ON t.id=je.transaction_id
WHERE je.entity_id=$1 AND je.status IN ('POSTED','REVERSED') AND je.journal_date BETWEEN $2 AND $3
ORDER BY je.journal_date DESC,je.created_at DESC,jl.line_no
LIMIT $4`, entityID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var journal, date, jdesc, account, code, name, ldesc, debit, credit, currency string
		var line int
		var tx *string
		if err := rows.Scan(&journal, &date, &jdesc, &account, &code, &name, &line, &ldesc, &debit, &credit, &currency, &tx); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"journal_id": journal, "date": date, "journal_description": jdesc, "account_id": account, "account_code": code, "account_name": name, "line_no": line, "description": ldesc, "debit": debit, "credit": credit, "currency": currency, "transaction_id": tx})
	}
	return out, rows.Err()
}

func (s *Store) AccountLedger(ctx context.Context, entityID, accountPublicID string, from, to time.Time, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := s.Pool.Query(ctx, `
WITH account_lines AS (
  SELECT je.journal_date,je.created_at,je.public_id journal_public_id,t.public_id transaction_public_id,
         je.description journal_description,COALESCE(jl.description,'') line_description,
         jl.line_no,jl.debit_amount,jl.credit_amount,je.functional_currency_code,
         CASE COALESCE(t.transaction_type,'')
           WHEN 'OPENING_BALANCE' THEN 0
           WHEN 'OPENING_BALANCE_ADJUSTMENT' THEN 1
           ELSE 2
         END opening_rank,
         SUM(jl.debit_amount-jl.credit_amount) OVER (
           ORDER BY je.journal_date,
             CASE COALESCE(t.transaction_type,'')
               WHEN 'OPENING_BALANCE' THEN 0
               WHEN 'OPENING_BALANCE_ADJUSTMENT' THEN 1
               ELSE 2
             END,
             je.created_at,jl.line_no,je.public_id
           ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
         ) running_balance
  FROM journal_lines jl
  JOIN journal_entries je ON je.id=jl.journal_entry_id
  JOIN accounts a ON a.id=jl.account_id
  LEFT JOIN transactions t ON t.id=je.transaction_id
  WHERE je.entity_id=$1 AND a.public_id=$2 AND je.status IN ('POSTED','REVERSED')
    AND je.journal_date <= $4
)
SELECT journal_date::text,journal_public_id::text,transaction_public_id::text,
       journal_description,line_description,
       debit_amount::text,credit_amount::text,running_balance::text,functional_currency_code
FROM account_lines
WHERE journal_date >= $3
ORDER BY journal_date,opening_rank,created_at,line_no
LIMIT $5`, entityID, accountPublicID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var date, journal, jdesc, ldesc, debit, credit, balance, currency string
		var tx *string
		if err := rows.Scan(&date, &journal, &tx, &jdesc, &ldesc, &debit, &credit, &balance, &currency); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"date": date, "journal_id": journal, "transaction_id": tx, "journal_description": jdesc, "description": ldesc, "debit": debit, "credit": credit, "running_balance": balance, "currency": currency})
	}
	return out, rows.Err()
}

func (s *Store) CashMovement(ctx context.Context, entityID string, from, to time.Time) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT je.journal_date::text,fa.public_id::text,fa.name,fa.currency_code,
       COALESCE(SUM(jl.transaction_debit_amount-jl.transaction_credit_amount),0)::text movement
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN financial_accounts fa ON fa.id=jl.financial_account_id
JOIN transactions t ON t.id=je.transaction_id
WHERE je.entity_id=$1 AND je.status IN ('POSTED','REVERSED') AND je.journal_date BETWEEN $2 AND $3
  AND t.transaction_type NOT IN ('OPENING_BALANCE','OPENING_BALANCE_ADJUSTMENT')
GROUP BY je.journal_date,fa.id,fa.public_id,fa.name,fa.currency_code
ORDER BY je.journal_date,fa.name`, entityID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var date, id, name, currency, movement string
		if err := rows.Scan(&date, &id, &name, &currency, &movement); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"date": date, "financial_account_id": id, "name": name, "currency": currency, "movement": movement})
	}
	return out, rows.Err()
}

func (s *Store) InterEntityBalances(ctx context.Context, entityID string, through time.Time) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT ce.public_id::text,ce.name,
       dfa.public_id::text,dfa.code,dfa.name,
       dta.public_id::text,dta.code,dta.name,
       COALESCE((SELECT SUM(jl.debit_amount-jl.credit_amount)
                 FROM journal_lines jl JOIN journal_entries je ON je.id=jl.journal_entry_id
                 WHERE jl.account_id=dfa.id AND je.status IN ('POSTED','REVERSED') AND je.journal_date <= $2),0)::text due_from,
       COALESCE((SELECT SUM(jl.credit_amount-jl.debit_amount)
                 FROM journal_lines jl JOIN journal_entries je ON je.id=jl.journal_entry_id
                 WHERE jl.account_id=dta.id AND je.status IN ('POSTED','REVERSED') AND je.journal_date <= $2),0)::text due_to
FROM inter_entity_account_mappings m
JOIN entities ce ON ce.id=m.counterparty_entity_id
JOIN accounts dfa ON dfa.id=m.due_from_account_id
JOIN accounts dta ON dta.id=m.due_to_account_id
WHERE m.entity_id=$1
ORDER BY ce.name`, entityID, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var cp, cpName, df, dfCode, dfName, dt, dtCode, dtName, dueFrom, dueTo string
		if err := rows.Scan(&cp, &cpName, &df, &dfCode, &dfName, &dt, &dtCode, &dtName, &dueFrom, &dueTo); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"counterparty_entity_id": cp, "counterparty_name": cpName, "due_from_account": map[string]any{"id": df, "code": dfCode, "name": dfName}, "due_to_account": map[string]any{"id": dt, "code": dtCode, "name": dtName}, "due_from": dueFrom, "due_to": dueTo})
	}
	return out, rows.Err()
}
