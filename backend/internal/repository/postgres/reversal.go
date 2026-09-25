package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func (s *Store) ReverseTransaction(ctx context.Context, user User, e Entity, originalPublicID, reversalDate, reason string) (map[string]any, error) {
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	if _, err := time.Parse("2006-01-02", reversalDate); err != nil {
		return nil, fmt.Errorf("invalid reversal date")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var originalID, originalType, originalStatus, originalDate, description, currency, total, journalID string
	if err := tx.QueryRow(ctx, `
SELECT t.id::text,t.transaction_type,t.status,t.transaction_date::text,t.description,t.currency_code,t.total_amount::text,je.id::text
FROM transactions t
JOIN journal_entries je ON je.transaction_id=t.id
WHERE t.entity_id=$1 AND t.public_id=$2
FOR UPDATE`, e.ID, originalPublicID).Scan(&originalID, &originalType, &originalStatus, &originalDate, &description, &currency, &total, &journalID); err != nil {
		return nil, err
	}
	if originalStatus != "POSTED" {
		return nil, fmt.Errorf("only POSTED transactions can be reversed")
	}
	if err := s.EnsureOpenDateTx(ctx, tx, e.ID, reversalDate); err != nil {
		return nil, err
	}

	reversalID, _ := ids.UUIDv7()
	reversalPub, _ := ids.ULID()
	reversalJournalID, _ := ids.UUIDv7()
	reversalJournalPub, _ := ids.ULID()
	reversalDescription := "Reversal: " + description + " — " + reason

	if _, err := tx.Exec(ctx, `
INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,currency_code,total_amount,source_type,original_transaction_id,created_by)
VALUES($1,$2,$3,'REVERSAL','DRAFT',$4,$5,$6,$7,'USER',$8,$9)`,
		reversalID, reversalPub, e.ID, reversalDate, reversalDescription, currency, total, originalID, user.ID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO journal_entries(id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by,reversal_of_journal_id)
SELECT $1,$2,$3,$4,$5,$6,'DRAFT',functional_currency_code,$7,id
FROM journal_entries WHERE id=$8`,
		reversalJournalID, reversalJournalPub, e.ID, reversalID, reversalDate, reversalDescription, user.ID, journalID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT line_no,account_id::text,contact_id::text,financial_account_id::text,
       COALESCE(description,''),transaction_currency_code,
       transaction_debit_amount::text,transaction_credit_amount::text,
       functional_currency_code,fx_rate_to_functional::text,
       debit_amount::text,credit_amount::text,exchange_rate_id::text
FROM journal_lines WHERE journal_entry_id=$1 ORDER BY line_no`, journalID)
	if err != nil {
		return nil, err
	}
	type line struct {
		n                                 int
		account                           string
		contact, fa                       *string
		desc, txc, td, tc, fc, rate, d, c string
		rateID                            *string
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.n, &l.account, &l.contact, &l.fa, &l.desc, &l.txc, &l.td, &l.tc, &l.fc, &l.rate, &l.d, &l.c, &l.rateID); err != nil {
			rows.Close()
			return nil, err
		}
		lines = append(lines, l)
	}
	rows.Close()

	for _, l := range lines {
		id, _ := ids.UUIDv7()
		if _, err := tx.Exec(ctx, `
INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,contact_id,financial_account_id,description,
transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			id, reversalJournalID, e.ID, l.n, l.account, l.contact, l.fa, l.desc, l.txc, l.tc, l.td, l.fc, l.rate, l.c, l.d, l.rateID); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED',posted_by=$2,posted_at=$3 WHERE id=$1`, reversalJournalID, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='REVERSED',reversed_by_journal_id=$2 WHERE id=$1`, journalID, reversalJournalID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED',posted_by=$2,posted_at=$3,updated_at=$3 WHERE id=$1`, reversalID, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='VOIDED',voided_by=$2,voided_at=$3,void_reason=$4,reversal_transaction_id=$5,updated_at=$3 WHERE id=$1`, originalID, user.ID, now, reason, reversalID); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, user, e, "TRANSACTION_REVERSE", "TRANSACTION", originalPublicID, map[string]any{"reversal_transaction_id": reversalPub, "reversal_date": reversalDate, "reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return map[string]any{"original_transaction_id": originalPublicID, "reversal_transaction_id": reversalPub, "reversal_journal_id": reversalJournalPub, "status": "POSTED", "reversed_at": now}, nil
}
