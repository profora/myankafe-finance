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
	RowID              string  `json:"row_id"`
	PublicID           string  `json:"id"`
	MovementLabel      string  `json:"movement_label"`
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	Date               string  `json:"date"`
	Description        string  `json:"description"`
	ContactName        *string `json:"contact_name"`
	FinancialAccountID *string `json:"financial_account_id"`
	FinancialAccount   *string `json:"financial_account_name"`
	AccountCurrency    *string `json:"account_currency"`
	LedgerAccountID    *string `json:"ledger_account_id"`
	SignedMovement     *string `json:"signed_movement"`
	Balance            *string `json:"balance"`
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
WITH posted_fa AS (
  SELECT t.id transaction_id,
         t.public_id::text transaction_public_id,
         t.transaction_type,
         t.status,
         t.transaction_date,
         t.description,
         t.created_at tx_created_at,
         c.display_name contact_name,
         fa.id fa_id,
         fa.public_id::text fa_public_id,
         fa.name fa_name,
         fa.currency_code fa_currency,
         coa.public_id::text ledger_account_id,
         SUM(jl.transaction_debit_amount-jl.transaction_credit_amount) movement,
         MAX(je.created_at) journal_created_at,
         MAX(jl.line_no) last_line_no,
         (SELECT count(*) FROM transaction_attachments ta WHERE ta.transaction_id=t.id AND ta.deleted_at IS NULL) attachment_count
  FROM journal_lines jl
  JOIN journal_entries je ON je.id=jl.journal_entry_id
  JOIN transactions t ON t.id=je.transaction_id
  JOIN financial_accounts fa ON fa.id=jl.financial_account_id
  JOIN accounts coa ON coa.id=fa.account_id
  LEFT JOIN contacts c ON c.id=t.contact_id
  WHERE je.entity_id=$1
    AND t.entity_id=$1
    AND je.status IN ('POSTED','REVERSED')
    AND jl.financial_account_id IS NOT NULL
  GROUP BY t.id, fa.id, coa.public_id, c.display_name
),
balanced AS (
  SELECT *,
         SUM(movement) OVER (
           PARTITION BY fa_id
           ORDER BY transaction_date,
             CASE transaction_type WHEN 'OPENING_BALANCE' THEN 0 WHEN 'OPENING_BALANCE_ADJUSTMENT' THEN 1 ELSE 2 END,
             journal_created_at, last_line_no, transaction_public_id
           ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
         ) running_balance
  FROM posted_fa
),
movements AS (
  SELECT transaction_public_id||':'||fa_public_id row_id,
         transaction_public_id,
         transaction_type,
         status,
         transaction_date,
         description,
         tx_created_at,
         contact_name,
         fa_public_id,
         fa_name,
         fa_currency,
         ledger_account_id,
         movement,
         running_balance,
         attachment_count,
         CASE
           WHEN transaction_type='ACCOUNT_TRANSFER' AND movement<0 THEN 'Transfer out'
           WHEN transaction_type='ACCOUNT_TRANSFER' AND movement>0 THEN 'Transfer in'
           WHEN transaction_type='ACCOUNT_TRANSFER' THEN 'Transfer'
           WHEN transaction_type='INCOME' THEN 'Income'
           WHEN transaction_type='EXPENSE' THEN 'Expense'
           WHEN transaction_type='INTER_ENTITY' THEN 'Pay for another entity'
           WHEN transaction_type='MANUAL_JOURNAL' THEN 'Manual journal'
           WHEN transaction_type='ADJUSTMENT' THEN 'Adjustment'
           WHEN transaction_type='REVERSAL' THEN 'Reversal'
           WHEN transaction_type='OPENING_BALANCE' THEN 'Opening balance'
           WHEN transaction_type='OPENING_BALANCE_ADJUSTMENT' THEN 'Opening balance adjustment'
           ELSE transaction_type
         END movement_label
  FROM balanced
),
headers AS (
  SELECT t.public_id::text||':none' row_id,
         t.public_id::text transaction_public_id,
         t.transaction_type,
         t.status,
         t.transaction_date,
         t.description,
         t.created_at tx_created_at,
         c.display_name contact_name,
         NULL::text fa_public_id,
         NULL::text fa_name,
         NULL::text fa_currency,
         NULL::text ledger_account_id,
         NULL::numeric movement,
         NULL::numeric running_balance,
         (SELECT count(*) FROM transaction_attachments ta WHERE ta.transaction_id=t.id AND ta.deleted_at IS NULL) attachment_count,
         CASE t.transaction_type
           WHEN 'INCOME' THEN 'Income'
           WHEN 'EXPENSE' THEN 'Expense'
           WHEN 'ACCOUNT_TRANSFER' THEN 'Transfer'
           WHEN 'INTER_ENTITY' THEN 'Pay for another entity'
           WHEN 'MANUAL_JOURNAL' THEN 'Manual journal'
           WHEN 'ADJUSTMENT' THEN 'Adjustment'
           WHEN 'REVERSAL' THEN 'Reversal'
           WHEN 'OPENING_BALANCE' THEN 'Opening balance'
           WHEN 'OPENING_BALANCE_ADJUSTMENT' THEN 'Opening balance adjustment'
           ELSE t.transaction_type
         END movement_label
  FROM transactions t
  LEFT JOIN contacts c ON c.id=t.contact_id
  WHERE t.entity_id=$1
    AND NOT EXISTS (
      SELECT 1
      FROM journal_lines jl
      JOIN journal_entries je ON je.id=jl.journal_entry_id
      WHERE je.transaction_id=t.id
        AND je.status IN ('POSTED','REVERSED')
        AND jl.financial_account_id IS NOT NULL
    )
),
display_rows AS (
  SELECT * FROM movements
  UNION ALL
  SELECT * FROM headers
),
filtered AS (
  SELECT *
  FROM display_rows d
  WHERE ($2='' OR d.status=$2)
    AND ($3='' OR d.transaction_type=$3)
    AND ($4='' OR d.transaction_date>=NULLIF($4,'')::date)
    AND ($5='' OR d.transaction_date<=NULLIF($5,'')::date)
    AND ($6='' OR d.fa_public_id=$6)
    AND (
      $7='' OR
      d.description ILIKE '%'||$7||'%' OR
      d.transaction_public_id ILIKE '%'||$7||'%' OR
      COALESCE(d.contact_name,'') ILIKE '%'||$7||'%' OR
      COALESCE(d.fa_name,'') ILIKE '%'||$7||'%' OR
      EXISTS (
        SELECT 1
        FROM transaction_attachments ta
        JOIN transactions t ON t.id=ta.transaction_id
        WHERE t.entity_id=$1
          AND t.public_id::text=d.transaction_public_id
          AND ta.deleted_at IS NULL
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
tx_keys AS (
  SELECT DISTINCT transaction_public_id FROM filtered
),
totals AS (
  SELECT COALESCE(SUM(COALESCE(e.income_effect,0)),0) income_total,
         COALESCE(SUM(COALESCE(e.expense_effect,0)),0) expense_total,
         COALESCE(SUM(COALESCE(e.functional_effect,0)),0) net_total
  FROM tx_keys k
  JOIN transactions t ON t.public_id::text=k.transaction_public_id AND t.entity_id=$1
  LEFT JOIN effects e ON e.transaction_id=t.id
),
enriched AS (
  SELECT f.*, COUNT(*) OVER() total_count
  FROM filtered f
)
SELECT e.row_id,e.transaction_public_id,e.movement_label,e.transaction_type,e.status,e.transaction_date::text,e.description,
       e.contact_name,e.fa_public_id,e.fa_name,e.fa_currency,e.ledger_account_id,
       e.movement::text,e.running_balance::text,e.attachment_count,
       e.total_count,totals.income_total::text,totals.expense_total::text,totals.net_total::text,
       ent.functional_currency_code
FROM enriched e
CROSS JOIN totals
JOIN entities ent ON ent.id=$1
ORDER BY e.transaction_date DESC,e.tx_created_at DESC,e.transaction_public_id DESC,
         CASE WHEN e.movement<0 THEN 0 WHEN e.movement IS NULL THEN 1 ELSE 2 END,
         e.fa_name
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
			&item.RowID, &item.PublicID, &item.MovementLabel, &item.Type, &item.Status, &item.Date, &item.Description,
			&item.ContactName, &item.FinancialAccountID, &item.FinancialAccount, &item.AccountCurrency, &item.LedgerAccountID,
			&item.SignedMovement, &item.Balance, &item.AttachmentCount,
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
