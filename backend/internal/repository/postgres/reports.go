package postgres

import (
	"context"
	"time"
)

type Dashboard struct {
	CashBalances []map[string]any `json:"cash_balances"`
	Income       string           `json:"income"`
	Expenses     string           `json:"expenses"`
	NetProfit    string           `json:"net_profit"`
}

func (s *Store) Dashboard(ctx context.Context, entityID string, from, to time.Time) (Dashboard, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT fa.public_id::text,fa.name,fa.currency_code,
       COALESCE(SUM(jl.transaction_debit_amount-jl.transaction_credit_amount),0)::text
FROM financial_accounts fa
LEFT JOIN (
  SELECT jl.*
  FROM journal_lines jl
  JOIN journal_entries je ON je.id=jl.journal_entry_id
  WHERE je.status IN ('POSTED','REVERSED')
) jl ON jl.financial_account_id=fa.id
WHERE fa.entity_id=$1 AND fa.active=true
GROUP BY fa.id,fa.public_id,fa.name,fa.currency_code
ORDER BY fa.name`, entityID)
	if err != nil {
		return Dashboard{}, err
	}
	defer rows.Close()
	balances := []map[string]any{}
	for rows.Next() {
		var id, name, currency, balance string
		if err := rows.Scan(&id, &name, &currency, &balance); err != nil {
			return Dashboard{}, err
		}
		balances = append(balances, map[string]any{"id": id, "name": name, "currency": currency, "balance": balance})
	}
	var income, expense, profit string
	err = s.Pool.QueryRow(ctx, `
SELECT
 COALESCE(SUM(CASE WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount ELSE 0 END),0)::text,
 COALESCE(SUM(CASE WHEN a.account_type='EXPENSE' THEN jl.debit_amount-jl.credit_amount ELSE 0 END),0)::text,
 COALESCE(SUM(CASE WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount WHEN a.account_type='EXPENSE' THEN jl.credit_amount-jl.debit_amount ELSE 0 END),0)::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id AND je.status IN ('POSTED','REVERSED')
JOIN accounts a ON a.id=jl.account_id
WHERE je.entity_id=$1 AND je.journal_date BETWEEN $2 AND $3`, entityID, from, to).Scan(&income, &expense, &profit)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{CashBalances: balances, Income: income, Expenses: expense, NetProfit: profit}, nil
}

func (s *Store) AccessibleFinancialAccounts(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT e.public_id::text,e.name,
       fa.public_id::text,fa.name,fa.kind,fa.currency_code,fa.active,
       a.public_id::text,
       COALESCE(SUM(CASE WHEN je.id IS NOT NULL AND je.status IN ('POSTED','REVERSED') THEN jl.transaction_debit_amount-jl.transaction_credit_amount ELSE 0 END),0)::text
FROM financial_accounts fa
JOIN entities e ON e.id=fa.entity_id
JOIN accounts a ON a.id=fa.account_id
LEFT JOIN journal_lines jl ON jl.financial_account_id=fa.id
LEFT JOIN journal_entries je ON je.id=jl.journal_entry_id AND je.status IN ('POSTED','REVERSED')
WHERE e.active=true
  AND EXISTS (
    SELECT 1 FROM user_entity_roles uer
    WHERE uer.entity_id=e.id AND uer.user_id=$1 AND uer.revoked_at IS NULL
  )
GROUP BY e.public_id,e.name,fa.id,fa.public_id,fa.name,fa.kind,fa.currency_code,fa.active,a.public_id
ORDER BY e.name,fa.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var entityID, entityName, id, name, kind, currency, ledgerID, balance string
		var active bool
		if err := rows.Scan(&entityID, &entityName, &id, &name, &kind, &currency, &active, &ledgerID, &balance); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"entity_id": entityID, "entity_name": entityName,
			"id": id, "name": name, "kind": kind, "currency": currency,
			"active": active, "ledger_account_id": ledgerID, "balance": balance,
		})
	}
	return out, rows.Err()
}

func (s *Store) ProfitLoss(ctx context.Context, entityID string, from, to time.Time) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT a.public_id::text,a.code,a.name,a.account_type,
 COALESCE(SUM(CASE WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount ELSE jl.debit_amount-jl.credit_amount END),0)::text amount
FROM accounts a
JOIN journal_lines jl ON jl.account_id=a.id
JOIN journal_entries je ON je.id=jl.journal_entry_id AND je.status IN ('POSTED','REVERSED')
WHERE a.entity_id=$1 AND a.account_type IN ('INCOME','EXPENSE') AND je.journal_date BETWEEN $2 AND $3
GROUP BY a.id,a.public_id,a.code,a.name,a.account_type
ORDER BY a.account_type,a.code`, entityID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, code, name, typ, amount string
		if err := rows.Scan(&id, &code, &name, &typ, &amount); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "code": code, "name": name, "type": typ, "amount": amount})
	}
	return out, rows.Err()
}

func (s *Store) TrialBalance(ctx context.Context, entityID string, through time.Time) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT a.public_id::text,a.code,a.name,a.account_type,
 CASE WHEN COALESCE(SUM(jl.debit_amount-jl.credit_amount),0) > 0
      THEN COALESCE(SUM(jl.debit_amount-jl.credit_amount),0) ELSE 0 END::text debits,
 CASE WHEN COALESCE(SUM(jl.debit_amount-jl.credit_amount),0) < 0
      THEN -COALESCE(SUM(jl.debit_amount-jl.credit_amount),0) ELSE 0 END::text credits
FROM accounts a
LEFT JOIN (
  SELECT jl.*
  FROM journal_lines jl
  JOIN journal_entries je ON je.id=jl.journal_entry_id
  WHERE je.status IN ('POSTED','REVERSED') AND je.journal_date <= $2
) jl ON jl.account_id=a.id
WHERE a.entity_id=$1
GROUP BY a.id,a.public_id,a.code,a.name,a.account_type
HAVING COALESCE(SUM(jl.debit_amount-jl.credit_amount),0)<>0
ORDER BY a.code`, entityID, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, code, name, typ, debits, credits string
		if err := rows.Scan(&id, &code, &name, &typ, &debits, &credits); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "code": code, "name": name, "type": typ, "debits": debits, "credits": credits})
	}
	return out, rows.Err()
}
