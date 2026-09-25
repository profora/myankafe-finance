package postgres

import "testing"

func TestDatabaseRejectsTransactionInsertInsideLockedPeriod(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "DIRECT_LOCK_INSERT")

	if _, err := tx.Exec(ctx, `
INSERT INTO entity_accounting_controls(entity_id,transactions_locked_through_date,lock_reason)
VALUES($1,'2026-09-30','direct SQL guard test')`, entityID); err != nil {
		t.Fatal(err)
	}

	_, err := tx.Exec(ctx, `
INSERT INTO transactions(
  id,public_id,entity_id,transaction_type,status,transaction_date,
  description,currency_code,total_amount,created_by
) VALUES($1,$2,$3,'EXPENSE','DRAFT','2026-09-24','blocked insert','MMK',100,$4)`,
		mustUUID(t), mustULID(t), entityID, userID)
	if err == nil {
		t.Fatal("expected direct transaction insert inside locked period to fail")
	}
}

func TestDatabaseRejectsDraftCancellationInsideLockedPeriod(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "DIRECT_LOCK_CANCEL")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)

	if _, err := tx.Exec(ctx, `
INSERT INTO entity_accounting_controls(entity_id,transactions_locked_through_date,lock_reason)
VALUES($1,'2026-09-30','direct SQL guard test')`, entityID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE transactions
SET status='VOIDED',voided_by=$2,voided_at=now(),void_reason='blocked cancel'
WHERE id=$1`, transactionID, userID); err == nil {
		t.Fatal("expected direct draft cancellation inside locked period to fail")
	}
}

func TestDatabaseAllowsReversalStatusOnOldPostedTransaction(t *testing.T) {
	ctx, tx := integrationTx(t)
	userID, entityID := seedEntity(t, ctx, tx, "DIRECT_REVERSAL_ALLOWED")
	debitAccount := seedAccount(t, ctx, tx, entityID, userID, "EXPENSE", "DEBIT")
	creditAccount := seedAccount(t, ctx, tx, entityID, userID, "ASSET", "CREDIT")
	transactionID := seedTransaction(t, ctx, tx, entityID, userID)
	journalID := seedJournal(t, ctx, tx, entityID, transactionID, userID)
	seedJournalLine(t, ctx, tx, journalID, entityID, debitAccount, 1, "100", "0", nil)
	seedJournalLine(t, ctx, tx, journalID, entityID, creditAccount, 2, "0", "100", nil)

	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED' WHERE id=$1`, journalID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED' WHERE id=$1`, transactionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO entity_accounting_controls(entity_id,transactions_locked_through_date,lock_reason)
VALUES($1,'2026-09-30','period closed after original posting')`, entityID); err != nil {
		t.Fatal(err)
	}

	// Reversal logic intentionally marks the old posted transaction VOIDED even
	// when its original date is locked. The correcting journal itself must be
	// dated in an open period.
	if _, err := tx.Exec(ctx, `
UPDATE transactions
SET status='VOIDED',voided_by=$2,voided_at=now(),void_reason='reversed in open period'
WHERE id=$1`, transactionID, userID); err != nil {
		t.Fatalf("posted-to-voided reversal state should remain allowed: %v", err)
	}
}
