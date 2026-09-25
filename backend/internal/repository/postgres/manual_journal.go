package postgres

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/accounting"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type ManualLine struct{ AccountPublicID, Debit, Credit, Description string }
type ManualJournalInput struct {
	Date, Description string
	Lines             []ManualLine
}

func (s *Store) PostManualJournal(ctx context.Context, user User, e Entity, in ManualJournalInput) (map[string]any, error) {
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		return nil, fmt.Errorf("invalid journal date")
	}
	if len(in.Lines) < 2 {
		return nil, fmt.Errorf("at least two lines are required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.EnsureOpenDateTx(ctx, tx, e.ID, in.Date); err != nil {
		return nil, err
	}

	type resolved struct{ id, debit, credit, desc string }
	lines := make([]resolved, 0, len(in.Lines))
	totalD, totalC := new(big.Rat), new(big.Rat)
	for _, l := range in.Lines {
		d, err := accounting.ParseAmount(l.Debit)
		if err != nil {
			return nil, err
		}
		c, err := accounting.ParseAmount(l.Credit)
		if err != nil {
			return nil, err
		}
		if (d.Sign() > 0) == (c.Sign() > 0) {
			return nil, fmt.Errorf("each line must contain exactly one of debit or credit")
		}
		totalD.Add(totalD, d)
		totalC.Add(totalC, c)
		var aid string
		var postable bool
		if err := tx.QueryRow(ctx, `SELECT id::text,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, l.AccountPublicID).Scan(&aid, &postable); err != nil {
			return nil, err
		}
		if !postable {
			return nil, fmt.Errorf("journal account is not postable")
		}
		lines = append(lines, resolved{aid, l.Debit, l.Credit, l.Description})
	}
	if totalD.Cmp(totalC) != 0 {
		return nil, fmt.Errorf("manual journal is not balanced")
	}

	tid, _ := ids.UUIDv7()
	tpub, _ := ids.ULID()
	jid, _ := ids.UUIDv7()
	jpub, _ := ids.ULID()
	if _, err := tx.Exec(ctx, `INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,currency_code,total_amount,created_by) VALUES($1,$2,$3,'MANUAL_JOURNAL','DRAFT',$4,$5,$6,0,$7)`, tid, tpub, e.ID, in.Date, in.Description, e.FunctionalCurrency, user.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO journal_entries(id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8)`, jid, jpub, e.ID, tid, in.Date, in.Description, e.FunctionalCurrency, user.ID); err != nil {
		return nil, err
	}
	for i, l := range lines {
		lid, _ := ids.UUIDv7()
		if _, err := tx.Exec(ctx, `INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$7,1,$8,$9)`, lid, jid, e.ID, i+1, l.id, l.desc, e.FunctionalCurrency, l.debit, l.credit); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED',posted_by=$2,posted_at=$3 WHERE id=$1`, jid, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED',posted_by=$2,posted_at=$3,updated_at=$3 WHERE id=$1`, tid, user.ID, now); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, user, e, "MANUAL_JOURNAL_POST", "JOURNAL_ENTRY", jpub, map[string]any{"transaction_id": tpub, "debits": totalD.FloatString(6)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": jpub, "transaction_id": tpub, "status": "POSTED", "debits": totalD.FloatString(6), "credits": totalC.FloatString(6)}, nil
}
