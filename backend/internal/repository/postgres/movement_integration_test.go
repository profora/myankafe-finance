package postgres

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"
)

func amountNear(t *testing.T, got string, want float64) {
	t.Helper()
	v, err := strconv.ParseFloat(got, 64)
	if err != nil || math.Abs(v-want) > 0.001 {
		t.Fatalf("amount %q want %v", got, want)
	}
}

func movementBy(items []TransactionListItem, transactionID, label string) TransactionListItem {
	for _, item := range items {
		if item.PublicID == transactionID && item.MovementLabel == label {
			return item
		}
	}
	return TransactionListItem{}
}

func TestFinancialAccountMovementBalances(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expensePublic, cashPublic := seedServiceEntity(t, ctx, s, "MOVE")
	_, other, _, _ := seedServiceEntity(t, ctx, s, "MOVEB")
	_, incomePublic := seedTypedAccountCommitted(t, ctx, s, entity, user, "INCOME", "SALE")
	secondCash := insertCashAccount(t, ctx, s, entity, user, "MMK")
	usdCash := insertCashAccount(t, ctx, s, entity, user, "USD")

	opening, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "INCOME", Date: "2026-01-01", Description: "Opening cash",
		FinancialAccountPublicID: cashPublic, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: incomePublic, Amount: "1000000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, opening.PublicID); err != nil {
		t.Fatal(err)
	}
	income, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "INCOME", Date: "2026-09-20", Description: "Later income",
		FinancialAccountPublicID: cashPublic, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: incomePublic, Amount: "100000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, income.PublicID); err != nil {
		t.Fatal(err)
	}
	expense, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-21", Description: "Later expense",
		FinancialAccountPublicID: cashPublic, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expensePublic, Amount: "50000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, expense.PublicID); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	incomeRow := movementBy(all.Items, income.PublicID, "Income")
	expenseRow := movementBy(all.Items, expense.PublicID, "Expense")
	if incomeRow.PublicID == "" || expenseRow.PublicID == "" {
		t.Fatalf("missing movement rows: %+v", all.Items)
	}
	amountNear(t, *incomeRow.SignedMovement, 100000)
	amountNear(t, *incomeRow.Balance, 1100000)
	amountNear(t, *expenseRow.SignedMovement, -50000)
	amountNear(t, *expenseRow.Balance, 1050000)
	if incomeRow.FinancialAccountID == nil || *incomeRow.FinancialAccountID != cashPublic {
		t.Fatalf("income account=%v", incomeRow.FinancialAccountID)
	}

	filtered, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{From: "2026-09-01", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	visibleIncome := movementBy(filtered.Items, income.PublicID, "Income")
	if visibleIncome.Balance == nil {
		t.Fatal("filtered income lost its balance")
	}
	amountNear(t, *visibleIncome.Balance, 1100000)

	var ledgerAccount string
	if err := s.Pool.QueryRow(ctx, `SELECT a.public_id::text FROM financial_accounts fa JOIN accounts a ON a.id=fa.account_id WHERE fa.public_id=$1`, cashPublic).Scan(&ledgerAccount); err != nil {
		t.Fatal(err)
	}
	ledger, err := s.AccountLedger(ctx, entity.ID, ledgerAccount, dateOf("2026-01-01"), dateOf("2026-09-30"), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) == 0 {
		t.Fatal("account ledger empty")
	}
	amountNear(t, ledger[len(ledger)-1]["running_balance"].(string), 1050000)

	transfer, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-22", FromFinancialAccountPublicID: cashPublic, ToFinancialAccountPublicID: secondCash,
		FromAmount: "100000", ToAmount: "100000", Description: "Two sided transfer",
	})
	if err != nil {
		t.Fatal(err)
	}
	fee, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-23", FromFinancialAccountPublicID: cashPublic, ToFinancialAccountPublicID: secondCash,
		FromAmount: "100000", ToAmount: "99000", FeeAmount: "1000", FeeExpenseAccountPublicID: expensePublic,
		Description: "Fee transfer",
	})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	outRow := movementBy(listed.Items, transfer["id"].(string), "Transfer out")
	inRow := movementBy(listed.Items, transfer["id"].(string), "Transfer in")
	if outRow.PublicID == "" || inRow.PublicID == "" || outRow.PublicID != inRow.PublicID {
		t.Fatalf("transfer rows out=%+v in=%+v", outRow, inRow)
	}
	amountNear(t, *outRow.SignedMovement, -100000)
	amountNear(t, *inRow.SignedMovement, 100000)
	if *outRow.FinancialAccountID == *inRow.FinancialAccountID {
		t.Fatal("transfer rows share one financial account")
	}
	feeRows := 0
	for _, item := range listed.Items {
		if item.PublicID == fee["id"].(string) {
			feeRows++
		}
	}
	if feeRows != 2 {
		t.Fatalf("fee transfer rows=%d", feeRows)
	}
	feeOut := movementBy(listed.Items, fee["id"].(string), "Transfer out")
	feeIn := movementBy(listed.Items, fee["id"].(string), "Transfer in")
	amountNear(t, *feeOut.SignedMovement, -100000)
	amountNear(t, *feeIn.SignedMovement, 99000)

	onlySource, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{FinancialAccountID: cashPublic, Type: "ACCOUNT_TRANSFER", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range onlySource.Items {
		if item.FinancialAccountID == nil || *item.FinancialAccountID != cashPublic || item.MovementLabel != "Transfer out" {
			t.Fatalf("account filter returned %+v", item)
		}
	}

	if _, err := s.ReverseTransaction(ctx, user, entity, expense.PublicID, "2026-09-24", "undo expense"); err != nil {
		t.Fatal(err)
	}
	reversed, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{FinancialAccountID: cashPublic, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var reversalBalance string
	for _, item := range reversed.Items {
		if item.Type == "REVERSAL" && item.Balance != nil {
			reversalBalance = *item.Balance
		}
	}
	amountNear(t, reversalBalance, 900000)

	page, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	byRow := map[string]TransactionListItem{}
	seen := map[string]bool{}
	for _, item := range full.Items {
		if seen[item.RowID] {
			t.Fatalf("duplicate row %s", item.RowID)
		}
		seen[item.RowID] = true
		byRow[item.RowID] = item
	}
	for _, item := range page.Items {
		match, ok := byRow[item.RowID]
		if !ok || item.Balance == nil || match.Balance == nil || *item.Balance != *match.Balance {
			t.Fatalf("page balance drifted for %+v", item)
		}
	}

	var cashCOA, secondCOA, equityPublic string
	if err := s.Pool.QueryRow(ctx, `SELECT a.public_id::text FROM financial_accounts fa JOIN accounts a ON a.id=fa.account_id WHERE fa.public_id=$1`, cashPublic).Scan(&cashCOA); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT a.public_id::text FROM financial_accounts fa JOIN accounts a ON a.id=fa.account_id WHERE fa.public_id=$1`, secondCash).Scan(&secondCOA); err != nil {
		t.Fatal(err)
	}
	_, equityPublic = seedTypedAccountCommitted(t, ctx, s, entity, user, "EQUITY", "OPENING")
	journal, err := s.PostManualJournal(ctx, user, entity, ManualJournalInput{
		Date: "2026-09-25", Description: "Two account journal",
		Lines: []ManualLine{
			{AccountPublicID: cashCOA, Debit: "25", Credit: "0"},
			{AccountPublicID: secondCOA, Debit: "0", Credit: "25"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.PostManualJournal(ctx, user, entity, ManualJournalInput{
		Date: "2026-09-25", Description: "No financial account journal",
		Lines: []ManualLine{
			{AccountPublicID: expensePublic, Debit: "15", Credit: "0"},
			{AccountPublicID: equityPublic, Debit: "0", Credit: "15"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	afterJournal, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Type: "MANUAL_JOURNAL", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	multi := 0
	var plainRow TransactionListItem
	for _, item := range afterJournal.Items {
		if item.PublicID == journal["transaction_id"].(string) {
			multi++
			if item.FinancialAccountID == nil || item.Balance == nil {
				t.Fatalf("journal movement missing account %+v", item)
			}
		}
		if item.PublicID == plain["transaction_id"].(string) {
			plainRow = item
		}
	}
	if multi != 2 {
		t.Fatalf("manual journal movements=%d", multi)
	}
	if plainRow.PublicID == "" || plainRow.FinancialAccountID != nil || plainRow.Balance != nil {
		t.Fatalf("plain journal row %+v", plainRow)
	}

	otherList, err := s.ListTransactionsFiltered(ctx, other.ID, TransactionListFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if otherList.Count != 0 {
		t.Fatalf("cross-entity rows=%d", otherList.Count)
	}
	_ = usdCash
}

func TestAccessibleFinancialAccountsStayInsideGrantedEntities(t *testing.T) {
	ctx, s := integrationStore(t)
	user, mine, _, myCash := seedServiceEntity(t, ctx, s, "DASHA")
	_, personal, _, personalCash := seedServiceEntity(t, ctx, s, "DASHB")
	_, hidden, _, hiddenCash := seedServiceEntity(t, ctx, s, "DASHC")
	grantEntityRole(t, ctx, s, user.ID, mine.ID, "OWNER")
	grantEntityRole(t, ctx, s, user.ID, personal.ID, "OWNER")
	usd := insertCashAccount(t, ctx, s, mine, user, "USD")
	_, incomePublic := seedTypedAccountCommitted(t, ctx, s, mine, user, "INCOME", "DASH_INCOME")
	posted, err := s.CreateTransaction(ctx, user, mine, CreateTransactionInput{
		Type: "INCOME", Date: "2026-09-26", Description: "Dashboard income",
		FinancialAccountPublicID: myCash, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: incomePublic, Amount: "3500000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, mine, posted.PublicID); err != nil {
		t.Fatal(err)
	}

	items, err := s.AccessibleFinancialAccounts(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]map[string]any{}
	for _, item := range items {
		if item["entity_name"] == hidden.Name || item["id"] == hiddenCash {
			t.Fatalf("hidden entity leaked: %+v", item)
		}
		seen[item["id"].(string)] = item
	}
	mineRow, ok := seen[myCash]
	if !ok || mineRow["entity_name"] != mine.Name || mineRow["kind"] != "CASH" || mineRow["currency"] != "MMK" {
		t.Fatalf("my account %+v", mineRow)
	}
	amountNear(t, mineRow["balance"].(string), 3500000)
	personalRow, ok := seen[personalCash]
	if !ok || personalRow["entity_name"] != personal.Name || personalRow["currency"] != "MMK" {
		t.Fatalf("personal account %+v", personalRow)
	}
	usdRow, ok := seen[usd]
	if !ok || usdRow["currency"] != "USD" {
		t.Fatalf("usd account %+v", usdRow)
	}
	amountNear(t, usdRow["balance"].(string), 0)
	if mineRow["balance"] == usdRow["balance"] && mineRow["currency"] == usdRow["currency"] {
		t.Fatal("unlike currencies were combined")
	}
}

func dateOf(v string) time.Time {
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		panic(err)
	}
	return d
}

func grantEntityRole(t *testing.T, ctx context.Context, s *Store, userID, entityID, role string) {
	t.Helper()
	roleIDs := map[string]string{
		"OWNER":      "00000000-0000-7000-8000-000000000001",
		"ADMIN":      "00000000-0000-7000-8000-000000000002",
		"ACCOUNTANT": "00000000-0000-7000-8000-000000000003",
		"BOOKKEEPER": "00000000-0000-7000-8000-000000000004",
		"VIEWER":     "00000000-0000-7000-8000-000000000005",
	}
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,$4,$2)`, mustUUID(t), userID, entityID, roleIDs[role]); err != nil {
		t.Fatal(err)
	}
}
