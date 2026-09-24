package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func TestTransactionListRunningTotalsRespectReversal(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "LIST_TOTALS")
	_, incomeAccount := seedTypedAccountCommitted(t, ctx, s, entity, user, "INCOME", "TEST_INCOME")

	incomeDraft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "INCOME",
		Date:                     "2026-09-20",
		Description:              "Searchable sale " + mustULID(t),
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: incomeAccount, Amount: "5000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, incomeDraft.PublicID); err != nil {
		t.Fatal(err)
	}

	expenseDraft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-21",
		Description:              "Reversed expense " + mustULID(t),
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "2000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, expenseDraft.PublicID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReverseTransaction(ctx, user, entity, expenseDraft.PublicID, "2026-09-22", "list totals test"); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{
		From:  "2026-09-20",
		To:    "2026-09-22",
		Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 3 {
		t.Fatalf("count=%d want 3", result.Count)
	}
	if result.IncomeTotal != "5000.000000" {
		t.Fatalf("income total=%s want 5000", result.IncomeTotal)
	}
	if result.ExpenseTotal != "0.000000" && result.ExpenseTotal != "0" {
		t.Fatalf("expense total=%s want 0 after reversal", result.ExpenseTotal)
	}
	if result.NetTotal != "5000.000000" {
		t.Fatalf("net total=%s want 5000", result.NetTotal)
	}
	if len(result.Items) != 3 || result.Items[0].RunningNet != "5000.000000" {
		t.Fatalf("unexpected latest running total: %+v", result.Items)
	}

	search, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{
		Search: "Searchable sale",
		Limit:  100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if search.Count != 1 || len(search.Items) != 1 || search.Items[0].PublicID != incomeDraft.PublicID {
		t.Fatalf("unexpected search result: %+v", search)
	}
}

func TestTransactionAttachmentRegistrationPreservesOrder(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "ATTACHMENTS")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-24",
		Description:              "Attachment metadata test",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := make([]NewAttachment, 0, 3)
	for i, name := range []string{"receipt.jpg", "invoice.pdf", "note.txt"} {
		publicID, err := ids.ULID()
		if err != nil {
			t.Fatal(err)
		}
		input = append(input, NewAttachment{
			PublicID:         publicID,
			StorageKey:       fmt.Sprintf("transactions/%s/%s/%s/%s", entity.PublicID, draft.PublicID, publicID, name),
			OriginalFilename: name,
			MimeType:         []string{"image/jpeg", "application/pdf", "text/plain"}[i],
			SizeBytes:        int64(100 + i),
			SHA256Hex:        fmt.Sprintf("%064x", i+1),
		})
	}

	created, err := s.RegisterTransactionAttachments(ctx, user, entity, draft.PublicID, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 3 {
		t.Fatalf("created=%d want 3", len(created))
	}
	for i, item := range created {
		if item.DisplayOrder != i {
			t.Fatalf("created order[%d]=%d", i, item.DisplayOrder)
		}
	}

	listed, err := s.ListTransactionAttachments(ctx, entity.ID, draft.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 3 {
		t.Fatalf("listed=%d want 3", len(listed))
	}
	for i, item := range listed {
		if item.PublicID != input[i].PublicID || item.DisplayOrder != i {
			t.Fatalf("attachment[%d]=%+v want id=%s order=%d", i, item, input[i].PublicID, i)
		}
	}

	obj, err := s.TransactionAttachmentObject(ctx, entity.ID, draft.PublicID, input[1].PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if obj.StorageKey != input[1].StorageKey || obj.OriginalFilename != "invoice.pdf" {
		t.Fatalf("unexpected attachment object: %+v", obj)
	}
}

func TestAttachmentPublicIDsRemainULIDs(t *testing.T) {
	id, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 26 {
		t.Fatalf("ULID length=%d", len(id))
	}
	_ = context.Background()
}
