package postgres

import (
	"context"
	"time"
)

func (s *Store) TransactionDetail(ctx context.Context, entityID, publicID string) (map[string]any, error) {
	var (
		internalID, id, typ, status, date, description, currency, total string
		contactID, contactName, financialID, financialName              *string
		originalID, reversalID, voidReason                              *string
		postedAt, voidedAt                                              *time.Time
	)

	err := s.Pool.QueryRow(ctx, `
SELECT t.id::text,t.public_id::text,t.transaction_type,t.status,t.transaction_date::text,
       t.description,t.currency_code,t.total_amount::text,
       c.public_id::text,c.display_name,
       fa.public_id::text,fa.name,
       ot.public_id::text,rt.public_id::text,
       t.posted_at,t.voided_at,t.void_reason
FROM transactions t
LEFT JOIN contacts c ON c.id=t.contact_id
LEFT JOIN financial_accounts fa ON fa.id=t.primary_financial_account_id
LEFT JOIN transactions ot ON ot.id=t.original_transaction_id
LEFT JOIN transactions rt ON rt.id=t.reversal_transaction_id
WHERE t.entity_id=$1 AND t.public_id=$2`, entityID, publicID).
		Scan(
			&internalID, &id, &typ, &status, &date, &description, &currency, &total,
			&contactID, &contactName, &financialID, &financialName,
			&originalID, &reversalID, &postedAt, &voidedAt, &voidReason,
		)
	if err != nil {
		return nil, err
	}

	splits := []map[string]any{}
	rows, err := s.Pool.Query(ctx, `
SELECT ts.line_no,a.public_id::text,a.code,a.name,ts.amount::text,COALESCE(ts.description,'')
FROM transaction_splits ts
JOIN accounts a ON a.id=ts.account_id
WHERE ts.transaction_id=$1
ORDER BY ts.line_no`, internalID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var lineNo int
		var accountID, code, name, amount, lineDescription string
		if err := rows.Scan(&lineNo, &accountID, &code, &name, &amount, &lineDescription); err != nil {
			rows.Close()
			return nil, err
		}
		splits = append(splits, map[string]any{
			"line_no": lineNo, "account_id": accountID, "account_code": code, "account_name": name,
			"amount": amount, "description": lineDescription,
		})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	journals := []map[string]any{}
	journalRows, err := s.Pool.Query(ctx, `
SELECT id::text,public_id::text,journal_date::text,description,status,functional_currency_code,
       posted_at,reversed_by_journal_id::text,reversal_of_journal_id::text
FROM journal_entries
WHERE entity_id=$1 AND transaction_id=$2
ORDER BY created_at`, entityID, internalID)
	if err != nil {
		return nil, err
	}
	defer journalRows.Close()

	for journalRows.Next() {
		var journalInternal, journalPublic, journalDate, journalDescription, journalStatus, functionalCurrency string
		var journalPostedAt *time.Time
		var reversedByInternal, reversalOfInternal *string
		if err := journalRows.Scan(
			&journalInternal, &journalPublic, &journalDate, &journalDescription, &journalStatus, &functionalCurrency,
			&journalPostedAt, &reversedByInternal, &reversalOfInternal,
		); err != nil {
			return nil, err
		}

		lines := []map[string]any{}
		lineRows, err := s.Pool.Query(ctx, `
SELECT jl.line_no,a.public_id::text,a.code,a.name,
       fa.public_id::text,fa.name,
       COALESCE(jl.description,''),
       jl.transaction_currency_code,
       jl.transaction_debit_amount::text,jl.transaction_credit_amount::text,
       jl.functional_currency_code,jl.fx_rate_to_functional::text,
       jl.debit_amount::text,jl.credit_amount::text,
       er.public_id::text
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
LEFT JOIN financial_accounts fa ON fa.id=jl.financial_account_id
LEFT JOIN exchange_rates er ON er.id=jl.exchange_rate_id
WHERE jl.journal_entry_id=$1
ORDER BY jl.line_no`, journalInternal)
		if err != nil {
			return nil, err
		}
		for lineRows.Next() {
			var lineNo int
			var accountID, code, name, lineDescription, txCurrency, txDebit, txCredit, funcCurrency, fxRate, debit, credit string
			var faID, faName, exchangeRateID *string
			if err := lineRows.Scan(
				&lineNo, &accountID, &code, &name, &faID, &faName, &lineDescription,
				&txCurrency, &txDebit, &txCredit, &funcCurrency, &fxRate, &debit, &credit, &exchangeRateID,
			); err != nil {
				lineRows.Close()
				return nil, err
			}
			lines = append(lines, map[string]any{
				"line_no": lineNo,
				"account": map[string]any{"id": accountID, "code": code, "name": name},
				"financial_account": map[string]any{"id": faID, "name": faName},
				"description": lineDescription,
				"transaction_currency": txCurrency,
				"transaction_debit": txDebit,
				"transaction_credit": txCredit,
				"functional_currency": funcCurrency,
				"fx_rate": fxRate,
				"debit": debit,
				"credit": credit,
				"exchange_rate_id": exchangeRateID,
			})
		}
		lineRows.Close()
		if err := lineRows.Err(); err != nil {
			return nil, err
		}

		journals = append(journals, map[string]any{
			"id": journalPublic,
			"date": journalDate,
			"description": journalDescription,
			"status": journalStatus,
			"functional_currency": functionalCurrency,
			"posted_at": journalPostedAt,
			"lines": lines,
		})
	}

	return map[string]any{
		"id": id,
		"type": typ,
		"status": status,
		"date": date,
		"description": description,
		"currency": currency,
		"total": total,
		"contact": map[string]any{"id": contactID, "name": contactName},
		"financial_account": map[string]any{"id": financialID, "name": financialName},
		"original_transaction_id": originalID,
		"reversal_transaction_id": reversalID,
		"posted_at": postedAt,
		"voided_at": voidedAt,
		"void_reason": voidReason,
		"splits": splits,
		"journals": journals,
	}, nil
}
