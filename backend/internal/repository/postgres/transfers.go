package postgres

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/accounting"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type TransferInput struct {
	Date                         string
	FromFinancialAccountPublicID string
	ToFinancialAccountPublicID   string
	FromAmount                   string
	ToAmount                     string
	FeeAmount                    string
	FeeExpenseAccountPublicID    string
	Description                  string
}

func (s *Store) rateToFunctional(ctx context.Context, tx pgx.Tx, e Entity, currency, date string) (*big.Rat, any, error) {
	if currency == e.FunctionalCurrency {
		return new(big.Rat).SetInt64(1), nil, nil
	}
	var rate, id string
	if err := tx.QueryRow(ctx, `SELECT rate::text,id::text FROM exchange_rates WHERE entity_id=$1 AND from_currency_code=$2 AND to_currency_code=$3 AND rate_date<=$4 ORDER BY rate_date DESC,created_at DESC LIMIT 1`, e.ID, currency, e.FunctionalCurrency, date).Scan(&rate, &id); err != nil {
		return nil, nil, err
	}
	r, _ := new(big.Rat).SetString(rate)
	return r, id, nil
}

func (s *Store) PostTransfer(ctx context.Context, user User, e Entity, in TransferInput) (map[string]any, error) {
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		return nil, fmt.Errorf("invalid transfer date")
	}
	fromAmount, err := accounting.ParseAmount(in.FromAmount)
	if err != nil || fromAmount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid from amount")
	}
	toAmount, err := accounting.ParseAmount(in.ToAmount)
	if err != nil || toAmount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid to amount")
	}
	if in.FromFinancialAccountPublicID == in.ToFinancialAccountPublicID {
		return nil, fmt.Errorf("source and destination must differ")
	}
	feeAmount, err := transferFeeAmount(in.FeeAmount)
	if err != nil {
		return nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.EnsureOpenDateTx(ctx, tx, e.ID, in.Date); err != nil {
		return nil, err
	}

	var fromID, fromCOA, fromCurrency, toID, toCOA, toCurrency string
	if err := tx.QueryRow(ctx, `SELECT id::text,account_id::text,currency_code FROM financial_accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, in.FromFinancialAccountPublicID).Scan(&fromID, &fromCOA, &fromCurrency); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT id::text,account_id::text,currency_code FROM financial_accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, in.ToFinancialAccountPublicID).Scan(&toID, &toCOA, &toCurrency); err != nil {
		return nil, err
	}
	fromCurrency = strings.TrimSpace(fromCurrency)
	toCurrency = strings.TrimSpace(toCurrency)

	fromRate, fromRateID, err := s.rateToFunctional(ctx, tx, e, fromCurrency, in.Date)
	if err != nil {
		return nil, fmt.Errorf("source exchange rate: %w", err)
	}
	toRate, toRateID, err := s.rateToFunctional(ctx, tx, e, toCurrency, in.Date)
	if err != nil {
		return nil, fmt.Errorf("destination exchange rate: %w", err)
	}
	fromFunctional := new(big.Rat).Mul(fromAmount, fromRate)
	toFunctional := new(big.Rat).Mul(toAmount, toRate)
	var feeAccountID string
	feeFunctional := new(big.Rat)
	if fromCurrency == toCurrency {
		if toAmount.Cmp(fromAmount) > 0 {
			return nil, fmt.Errorf("destination amount cannot exceed the source amount on a same-currency transfer")
		}
		if feeAmount == nil {
			if fromAmount.Cmp(toAmount) != 0 {
				return nil, fmt.Errorf("same-currency transfers require equal amounts unless a transfer fee is entered")
			}
		} else {
			if strings.TrimSpace(in.FeeExpenseAccountPublicID) == "" {
				return nil, fmt.Errorf("fee expense account is required")
			}
			sum := new(big.Rat).Add(toAmount, feeAmount)
			if sum.Cmp(fromAmount) != 0 {
				return nil, fmt.Errorf("same-currency transfer requires the source amount to equal the destination amount plus the fee")
			}
			var feeType string
			var feePostable, feeActive bool
			if err := tx.QueryRow(ctx, `
SELECT id::text, account_type, is_postable, active
FROM accounts
WHERE entity_id=$1 AND public_id=$2`, e.ID, in.FeeExpenseAccountPublicID).Scan(&feeAccountID, &feeType, &feePostable, &feeActive); err != nil {
				return nil, fmt.Errorf("fee expense account must be an active postable EXPENSE account in this entity")
			}
			if feeType != "EXPENSE" || !feePostable || !feeActive {
				return nil, fmt.Errorf("fee expense account must be an active postable EXPENSE account in this entity")
			}
			feeFunctional = new(big.Rat).Mul(feeAmount, fromRate)
		}
	} else if feeAmount != nil {
		return nil, fmt.Errorf("transfer fees are only supported when both accounts use the same currency")
	}
	debitFunctional := new(big.Rat).Add(toFunctional, feeFunctional)
	if fromFunctional.FloatString(6) != debitFunctional.FloatString(6) {
		return nil, fmt.Errorf("transfer functional values do not balance; add a rate/amount that produces equal functional value")
	}

	tid, _ := ids.UUIDv7()
	tpub, _ := ids.ULID()
	jid, _ := ids.UUIDv7()
	jpub, _ := ids.ULID()
	if _, err := tx.Exec(ctx, `INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,primary_financial_account_id,currency_code,total_amount,created_by) VALUES($1,$2,$3,'ACCOUNT_TRANSFER','DRAFT',$4,$5,$6,$7,$8,$9)`, tid, tpub, e.ID, in.Date, in.Description, fromID, fromCurrency, in.FromAmount, user.ID); err != nil {
		return nil, err
	}
	var feeAmountValue, feeCurrencyValue, feeAccountValue any
	if feeAmount != nil {
		feeAmountValue = feeAmount.FloatString(6)
		feeCurrencyValue = fromCurrency
		feeAccountValue = feeAccountID
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_transfer_details(transaction_id,from_financial_account_id,to_financial_account_id,from_amount,from_currency_code,to_amount,to_currency_code,fee_amount,fee_currency_code,fee_account_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, tid, fromID, toID, in.FromAmount, fromCurrency, in.ToAmount, toCurrency, feeAmountValue, feeCurrencyValue, feeAccountValue); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO journal_entries(id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8)`, jid, jpub, e.ID, tid, in.Date, in.Description, e.FunctionalCurrency, user.ID); err != nil {
		return nil, err
	}

	destLine, _ := ids.UUIDv7()
	srcLine, _ := ids.UUIDv7()
	if _, err := tx.Exec(ctx, `INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,financial_account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id) VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,0,$9,$10,$11,0,$12)`, destLine, jid, e.ID, toCOA, toID, in.Description, toCurrency, in.ToAmount, e.FunctionalCurrency, toRate.FloatString(12), toFunctional.FloatString(6), toRateID); err != nil {
		return nil, err
	}
	sourceLineNo := 2
	if feeAmount != nil {
		feeLine, _ := ids.UUIDv7()
		if _, err := tx.Exec(ctx, `INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id) VALUES($1,$2,$3,2,$4,$5,$6,$7,0,$8,$9,$10,0,$11)`, feeLine, jid, e.ID, feeAccountID, "Transfer fee", fromCurrency, feeAmount.FloatString(6), e.FunctionalCurrency, fromRate.FloatString(12), feeFunctional.FloatString(6), fromRateID); err != nil {
			return nil, err
		}
		sourceLineNo = 3
	}
	if _, err := tx.Exec(ctx, `INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,financial_account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0,$9,$10,$11,0,$12,$13)`, srcLine, jid, e.ID, sourceLineNo, fromCOA, fromID, in.Description, fromCurrency, in.FromAmount, e.FunctionalCurrency, fromRate.FloatString(12), fromFunctional.FloatString(6), fromRateID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED',posted_by=$2,posted_at=$3 WHERE id=$1`, jid, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED',posted_by=$2,posted_at=$3,updated_at=$3 WHERE id=$1`, tid, user.ID, now); err != nil {
		return nil, err
	}
	auditAfter := map[string]any{"journal_id": jpub, "from_account": in.FromFinancialAccountPublicID, "to_account": in.ToFinancialAccountPublicID}
	if feeAmount != nil {
		auditAfter["fee_amount"] = feeAmount.FloatString(6)
		auditAfter["fee_currency"] = fromCurrency
		auditAfter["fee_expense_account"] = in.FeeExpenseAccountPublicID
	}
	if err := insertAuditTx(ctx, tx, user, e, "ACCOUNT_TRANSFER_POST", "TRANSACTION", tpub, auditAfter); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": tpub, "journal_id": jpub, "status": "POSTED", "from_currency": fromCurrency, "to_currency": toCurrency}, nil
}

func transferFeeAmount(raw string) (*big.Rat, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	amount, err := accounting.ParseAmount(raw)
	if err != nil || amount.Sign() < 0 {
		return nil, fmt.Errorf("invalid transfer fee")
	}
	if amount.Sign() == 0 {
		return nil, nil
	}
	return amount, nil
}
