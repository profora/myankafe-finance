package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestCurrencyCreateUpdateDeactivateAndReferenceGuard(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, financialAccount := seedServiceEntity(t, ctx, s, "CUR")

	created, err := s.CreateCurrency(ctx, user, CurrencyInput{
		Code: "eur", Name: "Euro", Symbol: "€", DecimalPlaces: 2, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Code != "EUR" || created.Symbol != "€" || created.DecimalPlaces != 2 {
		t.Fatalf("created currency: %+v", created)
	}

	updated, err := s.UpdateCurrency(ctx, user, "EUR", CurrencyInput{
		Name: "Eurozone Euro", Symbol: "€", DecimalPlaces: 2, Active: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Eurozone Euro" || updated.Active {
		t.Fatalf("updated currency: %+v", updated)
	}
	if err := s.RequireActiveCurrency(ctx, "EUR"); err == nil {
		t.Fatal("inactive currency should be excluded from new selection")
	}

	active, err := s.ListCurrencies(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range active {
		if item.Code == "EUR" {
			t.Fatal("inactive currency returned by active list")
		}
	}
	all, err := s.ListCurrencies(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range all {
		if item.Code == "EUR" && !item.Active && item.Name == "Eurozone Euro" {
			found = true
		}
	}
	if !found {
		t.Fatal("inactive currency missing from the full currency list")
	}

	if err := s.DeleteCurrency(ctx, user, "MMK"); err == nil || !strings.Contains(err.Error(), "deactivate") {
		t.Fatalf("referenced currency delete err=%v", err)
	}
	account, err := s.ListFinancialAccounts(ctx, entity.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stillMMK bool
	for _, item := range account {
		if item.PublicID == financialAccount && strings.TrimSpace(item.Currency) == "MMK" {
			stillMMK = true
		}
	}
	if !stillMMK {
		t.Fatal("existing financial account no longer displays MMK")
	}

	if err := s.DeleteCurrency(ctx, user, "EUR"); err != nil {
		t.Fatal(err)
	}
}

func TestContactTypesAreSeededAndScoped(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "CTYPES")
	_, other, _, _ := seedServiceEntity(t, ctx, s, "CTYPESB")

	seeded, err := s.ListContactTypes(ctx, entity.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"CUSTOMER": "Customer",
		"SUPPLIER": "Supplier",
		"EMPLOYEE": "Employee",
		"OWNER":    "Owner",
		"OTHER":    "Other",
	}
	if len(seeded) != len(want) {
		t.Fatalf("seeded types=%d %+v", len(seeded), seeded)
	}
	for _, item := range seeded {
		if want[item.Code] != item.Name || !item.Active {
			t.Fatalf("unexpected seeded type %+v", item)
		}
	}

	created, err := s.CreateContactType(ctx, user, entity, ContactTypeInput{Code: "wholesale customer", Name: "Wholesale Customer", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.Code != "WHOLESALE_CUSTOMER" {
		t.Fatalf("code=%s", created.Code)
	}
	updated, err := s.UpdateContactType(ctx, user, entity, created.Code, "Wholesale Buyer", true)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Wholesale Buyer" || updated.Code != "WHOLESALE_CUSTOMER" {
		t.Fatalf("updated %+v", updated)
	}

	contact, err := s.CreateContact(ctx, user, entity, "WHOLESALE_CUSTOMER", "Historical Buyer", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateContactType(ctx, user, entity, "WHOLESALE_CUSTOMER", "Wholesale Buyer", false); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteContactType(ctx, user, entity, "WHOLESALE_CUSTOMER"); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatalf("in-use delete err=%v", err)
	}
	if _, err := s.CreateContact(ctx, user, entity, "WHOLESALE_CUSTOMER", "New Buyer", "", "", ""); err == nil {
		t.Fatal("inactive type accepted for a new contact")
	}
	if _, err := s.UpdateContact(ctx, user, entity, contact["id"].(string), UpdateContactInput{
		Type: "WHOLESALE_CUSTOMER", DisplayName: "Historical Buyer Renamed", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateContact(ctx, user, other, "WHOLESALE_CUSTOMER", "Cross Entity", "", "", ""); err == nil {
		t.Fatal("cross-entity contact type was accepted")
	}

	listed, err := s.ListContacts(ctx, entity.ID)
	if err != nil {
		t.Fatal(err)
	}
	var historical bool
	for _, item := range listed {
		if item["id"] == contact["id"] && item["contact_type"] == "WHOLESALE_CUSTOMER" {
			historical = true
		}
	}
	if !historical {
		t.Fatal("historical contact was not preserved")
	}
}

func TestSameCurrencyTransferFeeJournal(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expensePublic, fromPublic := seedServiceEntity(t, ctx, s, "XFER")
	toPublic := insertCashAccount(t, ctx, s, entity, user, "MMK")
	otherUser, otherEntity, otherExpense, _ := seedServiceEntity(t, ctx, s, "XFERB")
	_ = otherUser

	posted, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "100000", ToAmount: "100000", Description: "Equal MMK transfer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if posted["status"] != "POSTED" {
		t.Fatalf("status=%v", posted["status"])
	}

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "100000", ToAmount: "99000", Description: "Unequal without fee",
	}); err == nil {
		t.Fatal("unequal same-currency transfer without a fee was accepted")
	}
	assertNoTransaction(t, ctx, s, entity.ID, "Unequal without fee")

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "99000", ToAmount: "100000", Description: "Destination exceeds source",
	}); err == nil || !strings.Contains(err.Error(), "cannot exceed") {
		t.Fatalf("to greater than from err=%v", err)
	}

	feePosted, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "100000", ToAmount: "99000", FeeAmount: "1000", FeeExpenseAccountPublicID: expensePublic,
		Description: "MMK transfer with bank fee",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertBalancedTransfer(t, ctx, s, feePosted["id"].(string))

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "100000", ToAmount: "99000", FeeAmount: "1000", FeeExpenseAccountPublicID: otherExpense,
		Description: "Other entity fee account",
	}); err == nil {
		t.Fatal("other-entity expense account was accepted")
	}

	inactive := insertAccount(t, ctx, s, entity, user, "EXPENSE", false, true)
	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "500", ToAmount: "400", FeeAmount: "100", FeeExpenseAccountPublicID: inactive,
		Description: "Inactive fee account",
	}); err == nil {
		t.Fatal("inactive expense account was accepted")
	}
	nonPostable := insertAccount(t, ctx, s, entity, user, "EXPENSE", true, false)
	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "500", ToAmount: "400", FeeAmount: "100", FeeExpenseAccountPublicID: nonPostable,
		Description: "Non-postable fee account",
	}); err == nil {
		t.Fatal("non-postable expense account was accepted")
	}

	if err := s.Lock(ctx, user, entity, "2026-09-26", "transfer fee lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: fromPublic, ToFinancialAccountPublicID: toPublic,
		FromAmount: "10", ToAmount: "10", Description: "Locked date transfer",
	}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("locked date err=%v", err)
	}
	assertNoTransaction(t, ctx, s, entity.ID, "Locked date transfer")
	_ = otherEntity
}

func TestTransactionPaginationDoesNotDuplicateRows(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expensePublic, financialPublic := seedServiceEntity(t, ctx, s, "PAGE")
	_, other, _, _ := seedServiceEntity(t, ctx, s, "PAGEB")
	for i, amount := range []string{"10", "20", "30"} {
		draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
			Type: "EXPENSE", Date: "2026-09-0" + string(rune('1'+i)), Description: "Page row " + amount,
			FinancialAccountPublicID: financialPublic, Currency: "MMK",
			Splits: []SplitInput{{AccountPublicID: expensePublic, Amount: amount}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateTransaction(ctx, user, other, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-02", Description: "Other entity row",
		FinancialAccountPublicID: financialPublic, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expensePublic, Amount: "99"}},
	}); err == nil {
		t.Fatal("other entity accepted this entity's financial account")
	}

	first, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 3 || second.Count != 3 || len(first.Items) != 2 || len(second.Items) != 1 {
		t.Fatalf("pages first=%d second=%d count=%d/%d", len(first.Items), len(second.Items), first.Count, second.Count)
	}
	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		if seen[item.PublicID] {
			t.Fatalf("duplicate row %s", item.PublicID)
		}
		seen[item.PublicID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("union=%d", len(seen))
	}
	filtered, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{Type: "INCOME", Limit: 25, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Count != 0 {
		t.Fatalf("income filter count=%d", filtered.Count)
	}
}

func TestAuditActionsAreEntityScopedAndExact(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "AACT")
	_, other, _, _ := seedServiceEntity(t, ctx, s, "AACTB")
	if err := s.Audit(ctx, user, &entity, "CURRENCY_CREATE", "CURRENCY", nil, "SUCCESS", map[string]any{"code": "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Audit(ctx, user, &other, "CONTACT_TYPE_CREATE", "CONTACT_TYPE", nil, "SUCCESS", map[string]any{"code": "X"}); err != nil {
		t.Fatal(err)
	}

	actions, err := s.ListAuditActions(ctx, entity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(actions, "CURRENCY_CREATE") || containsString(actions, "CONTACT_TYPE_CREATE") {
		t.Fatalf("actions=%v", actions)
	}
	for i := 1; i < len(actions); i++ {
		if actions[i] < actions[i-1] {
			t.Fatalf("actions not sorted: %v", actions)
		}
	}
	exact, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Action: "CURRENCY_CREATE", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if exact.Count != 1 || exact.Items[0]["action"] != "CURRENCY_CREATE" {
		t.Fatalf("exact filter %+v", exact)
	}
}

func insertCashAccount(t *testing.T, ctx context.Context, s *Store, entity Entity, user User, currency string) string {
	t.Helper()
	_, publicID := seedTypedAccountCommitted(t, ctx, s, entity, user, "ASSET", "CASHBOX")
	var accountID string
	if err := s.Pool.QueryRow(ctx, `SELECT id::text FROM accounts WHERE entity_id=$1 AND public_id=$2`, entity.ID, publicID).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	faID := mustUUID(t)
	faPublic := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO financial_accounts(id,public_id,entity_id,account_id,code,name,kind,currency_code,created_by)
VALUES($1,$2,$3,$4,$5,$6,'CASH',$7,$8)`,
		faID, faPublic, entity.ID, accountID, "FA_"+faPublic, "Second cash", currency, user.ID); err != nil {
		t.Fatal(err)
	}
	return faPublic
}

func insertAccount(t *testing.T, ctx context.Context, s *Store, entity Entity, user User, accountType string, active, postable bool) string {
	t.Helper()
	id := mustUUID(t)
	publicID := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,active,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, publicID, entity.ID, "ACC_"+publicID, accountType, accountType, postable, active, user.ID); err != nil {
		t.Fatal(err)
	}
	return publicID
}

func assertNoTransaction(t *testing.T, ctx context.Context, s *Store, entityID, description string) {
	t.Helper()
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE entity_id=$1 AND description=$2`, entityID, description).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("transaction %q was stored", description)
	}
}

func assertBalancedTransfer(t *testing.T, ctx context.Context, s *Store, publicID string) {
	t.Helper()
	rows, err := s.Pool.Query(ctx, `
SELECT a.account_type, jl.debit_amount::float8, jl.credit_amount::float8
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
JOIN accounts a ON a.id=jl.account_id
WHERE t.public_id=$1
ORDER BY jl.line_no`, publicID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var debits, credits float64
	var sawFee, sawDest, sawSource bool
	var lines int
	for rows.Next() {
		var accountType string
		var debit, credit float64
		if err := rows.Scan(&accountType, &debit, &credit); err != nil {
			t.Fatal(err)
		}
		lines++
		if accountType == "EXPENSE" && debit == 1000 {
			sawFee = true
		}
		if accountType == "ASSET" && debit == 99000 {
			sawDest = true
		}
		if accountType == "ASSET" && credit == 100000 {
			sawSource = true
		}
		debits += debit
		credits += credit
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if lines != 3 || !sawFee || !sawDest || !sawSource || debits != credits {
		t.Fatalf("lines=%d fee=%v dest=%v source=%v debits=%v credits=%v", lines, sawFee, sawDest, sawSource, debits, credits)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
