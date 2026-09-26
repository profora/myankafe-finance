package postgres

import (
	"context"
	"fmt"
)

type TransactionListFilter struct {
	Search             string
	Status             string
	Type               string
	From               string
	To                 string
	FinancialAccountID string
	Limit              int
	Offset             int
}

type TransactionListItem struct {
	PublicID           string  `json:"id"`
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	Date               string  `json:"date"`
	Description        string  `json:"description"`
	Currency           string  `json:"currency"`
	Total              string  `json:"total"`
	ContactName        *string `json:"contact_name"`
	FinancialAccountID *string `json:"financial_account_id"`
	FinancialAccount   *string `json:"financial_account_name"`
	AttachmentCount    int     `json:"attachment_count"`
}

type TransactionListResult struct {
	Items              []TransactionListItem `json:"items"`
	Count              int                   `json:"count"`
	IncomeTotal        string                `json:"income_total"`
	ExpenseTotal       string                `json:"expense_total"`
	NetTotal           string                `json:"net_total"`
	FunctionalCurrency string                `json:"functional_currency"`
	HasMore            bool                  `json:"has_more"`
}

func (s *Store) ListTransactionsFiltered(ctx context.Context, entityID string, f TransactionListFilter) (TransactionListResult, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	rows, err := s.Pool.Query(ctx, `
WITH filtered AS (
  SELECT t.id,t.public_id,t.transaction_type,t.status,t.transaction_date,t.description,
         t.currency_code,t.total_amount,t.created_at,
         c.display_name contact_name,
         fa.public_id financial_account_public_id,fa.name financial_account_name,
         (SELECT count(*) FROM transaction_attachments ta WHERE ta.transaction_id=t.id AND ta.deleted_at IS NULL) attachment_count
  FROM transactions t
  LEFT JOIN contacts c ON c.id=t.contact_id
  LEFT JOIN financial_accounts fa ON fa.id=t.primary_financial_account_id
  WHERE t.entity_id=$1
    AND ($2='' OR t.status=$2)
    AND ($3='' OR t.transaction_type=$3)
    AND ($4='' OR t.transaction_date>=NULLIF($4,'')::date)
    AND ($5='' OR t.transaction_date<=NULLIF($5,'')::date)
    AND (
      $6='' OR
      fa.public_id::text=$6 OR
      EXISTS (
        SELECT 1
        FROM journal_entries jef
        JOIN journal_lines jlf ON jlf.journal_entry_id=jef.id
        JOIN financial_accounts faf ON faf.id=jlf.financial_account_id
        WHERE jef.transaction_id=t.id AND faf.public_id::text=$6
      )
    )
    AND (
      $7='' OR
      t.description ILIKE '%'||$7||'%' OR
      t.public_id::text ILIKE '%'||$7||'%' OR
      COALESCE(c.display_name,'') ILIKE '%'||$7||'%' OR
      COALESCE(fa.name,'') ILIKE '%'||$7||'%' OR
      COALESCE(t.external_reference,'') ILIKE '%'||$7||'%' OR
      EXISTS (
        SELECT 1 FROM transaction_attachments ta
        WHERE ta.transaction_id=t.id AND ta.deleted_at IS NULL
          AND ta.original_filename ILIKE '%'||$7||'%'
      )
    )
),
effects AS (
  SELECT je.transaction_id,
         COALESCE(SUM(CASE WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount ELSE 0 END),0) income_effect,
         COALESCE(SUM(CASE WHEN a.account_type='EXPENSE' THEN jl.debit_amount-jl.credit_amount ELSE 0 END),0) expense_effect,
         COALESCE(SUM(
           CASE
             WHEN a.account_type='INCOME' THEN jl.credit_amount-jl.debit_amount
             WHEN a.account_type='EXPENSE' THEN jl.credit_amount-jl.debit_amount
             ELSE 0
           END
         ),0) functional_effect
  FROM journal_entries je
  JOIN journal_lines jl ON jl.journal_entry_id=je.id
  JOIN accounts a ON a.id=jl.account_id
  WHERE je.entity_id=$1 AND je.status IN ('POSTED','REVERSED')
  GROUP BY je.transaction_id
),
enriched AS (
  SELECT f.*,
         COUNT(*) OVER() total_count,
         COALESCE(SUM(COALESCE(e.income_effect,0)) OVER(),0) income_total,
         COALESCE(SUM(COALESCE(e.expense_effect,0)) OVER(),0) expense_total,
         COALESCE(SUM(COALESCE(e.functional_effect,0)) OVER(),0) net_total
  FROM filtered f
  LEFT JOIN effects e ON e.transaction_id=f.id
)
SELECT e.public_id::text,e.transaction_type,e.status,e.transaction_date::text,e.description,
       e.currency_code,e.total_amount::text,e.contact_name,
       e.financial_account_public_id::text,e.financial_account_name,
       e.attachment_count,
       e.total_count,e.income_total::text,e.expense_total::text,e.net_total::text,
       ent.functional_currency_code
FROM enriched e
JOIN entities ent ON ent.id=$1
ORDER BY e.transaction_date DESC,e.created_at DESC,e.public_id DESC
LIMIT $8 OFFSET $9`,
		entityID, f.Status, f.Type, f.From, f.To, f.FinancialAccountID, f.Search, f.Limit, f.Offset)
	if err != nil {
		return TransactionListResult{}, err
	}
	defer rows.Close()

	out := TransactionListResult{Items: []TransactionListItem{}, IncomeTotal: "0", ExpenseTotal: "0", NetTotal: "0"}
	for rows.Next() {
		var item TransactionListItem
		var totalCount int
		var income, expense, net, functionalCurrency string
		if err := rows.Scan(
			&item.PublicID, &item.Type, &item.Status, &item.Date, &item.Description,
			&item.Currency, &item.Total, &item.ContactName, &item.FinancialAccountID, &item.FinancialAccount,
			&item.AttachmentCount,
			&totalCount, &income, &expense, &net, &functionalCurrency,
		); err != nil {
			return TransactionListResult{}, err
		}
		out.Items = append(out.Items, item)
		out.Count = totalCount
		out.IncomeTotal = income
		out.ExpenseTotal = expense
		out.NetTotal = net
		out.FunctionalCurrency = functionalCurrency
	}
	if err := rows.Err(); err != nil {
		return TransactionListResult{}, err
	}
	if out.FunctionalCurrency == "" {
		if err := s.Pool.QueryRow(ctx, `SELECT functional_currency_code FROM entities WHERE id=$1`, entityID).Scan(&out.FunctionalCurrency); err != nil {
			return TransactionListResult{}, fmt.Errorf("load entity currency: %w", err)
		}
	}
	out.HasMore = f.Offset+len(out.Items) < out.Count
	return out, nil
}
