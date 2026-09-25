package postgres

import "testing"

func TestCreateDraftRejectsLockedDate(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "CREATE_LOCK")

	if err := s.Lock(ctx, user, entity, "2026-09-30", "closed month"); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-24",
		Description:              "must be rejected",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	})
	if err == nil {
		t.Fatal("expected draft creation inside locked period to fail")
	}
}

func TestUpdateDraftTransactionReplacesAccountingInput(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "DRAFT_EDIT")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-02",
		Description:              "original draft",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000", Description: "old"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateDraftTransaction(ctx, user, entity, draft.PublicID, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-03",
		Description:              "updated draft",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{
			{AccountPublicID: expenseAccount, Amount: "1200", Description: "first"},
			{AccountPublicID: expenseAccount, Amount: "800", Description: "second"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "DRAFT" || updated.Date != "2026-10-03" || updated.Total != "2000.000000" {
		t.Fatalf("unexpected updated draft: %+v", updated)
	}

	var description, total, date string
	var splitCount int
	if err := s.Pool.QueryRow(ctx, `
SELECT description,total_amount::text,transaction_date::text,
       (SELECT count(*) FROM transaction_splits WHERE transaction_id=t.id)
FROM transactions t
WHERE entity_id=$1 AND public_id=$2`, entity.ID, draft.PublicID).
		Scan(&description, &total, &date, &splitCount); err != nil {
		t.Fatal(err)
	}
	if description != "updated draft" || total != "2000.000000" || date != "2026-10-03" || splitCount != 2 {
		t.Fatalf("stored draft description=%q total=%s date=%s splits=%d", description, total, date, splitCount)
	}
}

func TestUpdatePostedTransactionIsRejected(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "POSTED_EDIT")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-02",
		Description:              "posted source",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
		t.Fatal(err)
	}

	_, err = s.UpdateDraftTransaction(ctx, user, entity, draft.PublicID, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-03",
		Description:              "illegal edit",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "2000"}},
	})
	if err == nil {
		t.Fatal("expected posted transaction edit to fail")
	}
}

func TestCancelDraftPreservesRecordAndAuditState(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "DRAFT_CANCEL")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-02",
		Description:              "cancel me",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	cancelled, err := s.CancelDraftTransaction(ctx, user, entity, draft.PublicID, "entered twice")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "VOIDED" {
		t.Fatalf("status=%s want VOIDED", cancelled.Status)
	}

	var status, reason string
	var auditCount int
	if err := s.Pool.QueryRow(ctx, `
SELECT status,COALESCE(void_reason,''),
       (SELECT count(*) FROM audit_events
        WHERE entity_id=t.entity_id
          AND resource_public_id=t.public_id
          AND action='TRANSACTION_DRAFT_CANCEL'
          AND outcome='SUCCESS')
FROM transactions t
WHERE entity_id=$1 AND public_id=$2`, entity.ID, draft.PublicID).
		Scan(&status, &reason, &auditCount); err != nil {
		t.Fatal(err)
	}
	if status != "VOIDED" || reason != "entered twice" || auditCount != 1 {
		t.Fatalf("status=%s reason=%q audit=%d", status, reason, auditCount)
	}
}

func TestCancelDraftRejectsNewlyLockedDate(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "DRAFT_CANCEL_LOCK")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-02",
		Description:              "draft before close",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, entity, "2026-10-02", "close through draft date"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CancelDraftTransaction(ctx, user, entity, draft.PublicID, "try cancel"); err == nil {
		t.Fatal("expected draft cancellation in newly locked period to fail")
	}

	var status string
	if err := s.Pool.QueryRow(ctx, `SELECT status FROM transactions WHERE entity_id=$1 AND public_id=$2`, entity.ID, draft.PublicID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "DRAFT" {
		t.Fatalf("status=%s want DRAFT", status)
	}
}
