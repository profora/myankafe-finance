package postgres

import (
	"context"
	"os"
	"testing"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func integrationTx(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return ctx, tx
}

func mustUUID(t *testing.T) string {
	t.Helper()
	v, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mustULID(t *testing.T) string {
	t.Helper()
	v, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func seedEntity(t *testing.T, ctx context.Context, tx pgx.Tx, suffix string) (userID, entityID string) {
	t.Helper()
	userID = mustUUID(t)
	userPublic := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO users(id,public_id,username,display_name)
VALUES($1,$2,$3,$4)`,
		userID, userPublic, strings.ToLower("test-"+suffix+"-"+userPublic), "Test User"); err != nil {
		t.Fatal(err)
	}

	entityID = mustUUID(t)
	entityPublic := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO entities(
  id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,$4,'BUSINESS','MMK','Asia/Yangon',$5)`,
		entityID, entityPublic, "TEST_"+suffix+"_"+entityPublic, "Test Entity "+suffix, userID); err != nil {
		t.Fatal(err)
	}
	return userID, entityID
}

func seedAccount(t *testing.T, ctx context.Context, tx pgx.Tx, entityID, userID, accountType, suffix string) string {
	t.Helper()
	id := mustUUID(t)
	publicID := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,$6,true,$7)`,
		id, publicID, entityID, "A_"+suffix+"_"+publicID, "Account "+suffix, accountType, userID); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedTransaction(t *testing.T, ctx context.Context, tx pgx.Tx, entityID, userID string) string {
	t.Helper()
	id := mustUUID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions(
  id,public_id,entity_id,transaction_type,status,transaction_date,
  description,currency_code,total_amount,created_by
) VALUES($1,$2,$3,'MANUAL_JOURNAL','DRAFT','2026-09-24','test','MMK',100,$4)`,
		id, mustULID(t), entityID, userID); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedJournal(t *testing.T, ctx context.Context, tx pgx.Tx, entityID, transactionID, userID string) string {
	t.Helper()
	id := mustUUID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO journal_entries(
  id,public_id,entity_id,transaction_id,journal_date,description,status,
  functional_currency_code,created_by
) VALUES($1,$2,$3,$4,'2026-09-24','test','DRAFT','MMK',$5)`,
		id, mustULID(t), entityID, transactionID, userID); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedJournalLine(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	journalID, entityID, accountID string,
	lineNo int,
	debit, credit string,
	exchangeRateID any,
) string {
	t.Helper()
	id := mustUUID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO journal_lines(
  id,journal_entry_id,entity_id,line_no,account_id,
  transaction_currency_code,transaction_debit_amount,transaction_credit_amount,
  functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id
) VALUES($1,$2,$3,$4,$5,'MMK',$6,$7,'MMK',1,$6,$7,$8)`,
		id, journalID, entityID, lineNo, accountID, debit, credit, exchangeRateID); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDatabaseRejectsUnbalancedPostedJournal(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "UNBALANCED")
	debitAccount := seedAccount(t, ctx, tx, entityID, userID, "EXPENSE", "DEBIT")
	creditAccount := seedAccount(t, ctx, tx, entityID, userID, "ASSET", "CREDIT")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)
	journalID := seedJournal(t, ctx, tx, entityID, transactionID, userID)

	seedJournalLine(t, ctx, tx, journalID, entityID, debitAccount, 1, "100", "0", nil)
	seedJournalLine(t, ctx, tx, journalID, entityID, creditAccount, 2, "0", "99", nil)

	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED' WHERE id=$1`, journalID); err == nil {
		t.Fatal("expected unbalanced journal posting to fail")
	}
}

func TestDatabaseRejectsCrossEntityTransactionSplit(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityOne := seedEntity(t, ctx, tx, "SPLIT_ONE")
	_, entityTwo := seedEntity(t, ctx, tx, "SPLIT_TWO")
	foreignAccount := seedAccount(t, ctx, tx, entityTwo, userID, "EXPENSE", "FOREIGN")
	transactionID := seedTransaction(t, ctx, tx, entityOne, userID)

	_, err := tx.Exec(ctx, `
INSERT INTO transaction_splits(id,transaction_id,line_no,account_id,amount)
VALUES($1,$2,1,$3,100)`, mustUUID(t), transactionID, foreignAccount)
	if err == nil {
		t.Fatal("expected cross-entity transaction split to fail")
	}
}

func TestDatabaseRejectsPostingInsideLockedPeriod(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "LOCKED")
	debitAccount := seedAccount(t, ctx, tx, entityID, userID, "EXPENSE", "DEBIT")
	creditAccount := seedAccount(t, ctx, tx, entityID, userID, "ASSET", "CREDIT")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)
	journalID := seedJournal(t, ctx, tx, entityID, transactionID, userID)
	seedJournalLine(t, ctx, tx, journalID, entityID, debitAccount, 1, "100", "0", nil)
	seedJournalLine(t, ctx, tx, journalID, entityID, creditAccount, 2, "0", "100", nil)

	if _, err := tx.Exec(ctx, `
INSERT INTO entity_accounting_controls(entity_id,transactions_locked_through_date,lock_reason)
VALUES($1,'2026-09-30','test lock')`, entityID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED' WHERE id=$1`, journalID); err == nil {
		t.Fatal("expected locked-period journal posting to fail")
	}
}

func TestDatabaseRejectsPostedJournalLineMutation(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "IMMUTABLE")
	debitAccount := seedAccount(t, ctx, tx, entityID, userID, "EXPENSE", "DEBIT")
	creditAccount := seedAccount(t, ctx, tx, entityID, userID, "ASSET", "CREDIT")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)
	journalID := seedJournal(t, ctx, tx, entityID, transactionID, userID)
	lineID := seedJournalLine(t, ctx, tx, journalID, entityID, debitAccount, 1, "100", "0", nil)
	seedJournalLine(t, ctx, tx, journalID, entityID, creditAccount, 2, "0", "100", nil)

	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED' WHERE id=$1`, journalID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE journal_lines SET debit_amount=101 WHERE id=$1`, lineID); err == nil {
		t.Fatal("expected posted journal line mutation to fail")
	}
}

func TestDatabaseRejectsUsedExchangeRateMutation(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "FX")
	debitAccount := seedAccount(t, ctx, tx, entityID, userID, "EXPENSE", "DEBIT")
	creditAccount := seedAccount(t, ctx, tx, entityID, userID, "ASSET", "CREDIT")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)
	journalID := seedJournal(t, ctx, tx, entityID, transactionID, userID)

	rateID := mustUUID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO exchange_rates(
  id,public_id,entity_id,rate_date,from_currency_code,to_currency_code,rate,source,created_by
) VALUES($1,$2,$3,'2026-09-24','USD','MMK',4000,'MANUAL',$4)`,
		rateID, mustULID(t), entityID, userID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `UPDATE transactions SET currency_code='USD',total_amount=1 WHERE id=$1`, transactionID); err != nil {
		t.Fatal(err)
	}
	for lineNo, line := range []struct {
		accountID string
		debitUSD  string
		creditUSD string
		debitMMK  string
		creditMMK string
	}{
		{debitAccount, "1", "0", "4000", "0"},
		{creditAccount, "0", "1", "0", "4000"},
	} {
		if _, err := tx.Exec(ctx, `
INSERT INTO journal_lines(
  id,journal_entry_id,entity_id,line_no,account_id,
  transaction_currency_code,transaction_debit_amount,transaction_credit_amount,
  functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id
) VALUES($1,$2,$3,$4,$5,'USD',$6,$7,'MMK',4000,$8,$9,$10)`,
			mustUUID(t), journalID, entityID, lineNo+1, line.accountID,
			line.debitUSD, line.creditUSD, line.debitMMK, line.creditMMK, rateID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED' WHERE id=$1`, journalID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `UPDATE exchange_rates SET rate=4100 WHERE id=$1`, rateID); err == nil {
		t.Fatal("expected used exchange rate mutation to fail")
	}
}

func TestDatabaseRequiresPostedJournalBeforeTransactionPosting(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "TX_JOURNAL")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)

	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED' WHERE id=$1`, transactionID); err == nil {
		t.Fatal("expected transaction posting without posted journal to fail")
	}
}
