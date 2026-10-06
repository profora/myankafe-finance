package postgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestChartTemplatesSeedAtomicallyWithFourDigitCodes(t *testing.T) {
	ctx, s, user := openingUser(t)
	cases := []struct {
		template  string
		count     int
		child     string
		parent    string
		childType string
	}{
		{TemplateMyanKafeBusiness, 72, "4110", "4000", "INCOME"},
		{TemplateRoyalMasterpiece, 73, "4220", "4000", "INCOME"},
		{TemplatePersonal, 49, "5110", "5000", "EXPENSE"},
	}
	for i, tc := range cases {
		entity := openingEntity(t, ctx, s, user, tc.template, "TPL"+string(rune('A'+i)))
		var count int
		if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE entity_id=$1`, entity.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != tc.count {
			t.Fatalf("%s account count=%d want %d", tc.template, count, tc.count)
		}
		var bad int
		if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE entity_id=$1 AND code !~ '^[0-9]{4}$'`, entity.ID).Scan(&bad); err != nil {
			t.Fatal(err)
		}
		if bad != 0 {
			t.Fatalf("%s non-four-digit codes=%d", tc.template, bad)
		}
		var parentCode, childType, adjustmentRole, equityRole string
		if err := s.Pool.QueryRow(ctx, `
SELECT p.code,c.account_type
FROM accounts c JOIN accounts p ON p.id=c.parent_id
WHERE c.entity_id=$1 AND c.code=$2`, entity.ID, tc.child).Scan(&parentCode, &childType); err != nil {
			t.Fatal(err)
		}
		if parentCode != tc.parent || childType != tc.childType {
			t.Fatalf("%s parent/type=%s/%s", tc.template, parentCode, childType)
		}
		if err := s.Pool.QueryRow(ctx, `SELECT system_role FROM accounts WHERE entity_id=$1 AND code='3980'`, entity.ID).Scan(&adjustmentRole); err != nil {
			t.Fatal(err)
		}
		if err := s.Pool.QueryRow(ctx, `SELECT system_role FROM accounts WHERE entity_id=$1 AND code='3990'`, entity.ID).Scan(&equityRole); err != nil {
			t.Fatal(err)
		}
		if adjustmentRole != SystemRoleOpeningBalanceAdjustment || equityRole != SystemRoleOpeningBalanceEquity {
			t.Fatalf("system roles %s %s", adjustmentRole, equityRole)
		}
		var reserved int
		if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE entity_id=$1 AND code BETWEEN '1121' AND '1129'`, entity.ID).Scan(&reserved); err != nil {
			t.Fatal(err)
		}
		if reserved != 0 {
			t.Fatalf("reserved bank accounts were seeded: %d", reserved)
		}
		var owners, contacts int
		if err := s.Pool.QueryRow(ctx, `
SELECT count(*) FROM user_entity_roles uer
JOIN roles r ON r.id=uer.role_id
WHERE uer.entity_id=$1 AND uer.user_id=$2 AND uer.revoked_at IS NULL AND r.code='OWNER'`, entity.ID, user.ID).Scan(&owners); err != nil {
			t.Fatal(err)
		}
		if owners != 1 {
			t.Fatalf("owner roles=%d", owners)
		}
		if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM contact_types WHERE entity_id=$1`, entity.ID).Scan(&contacts); err != nil {
			t.Fatal(err)
		}
		if contacts != 5 {
			t.Fatalf("contact types=%d", contacts)
		}
	}

	if _, err := s.CreateEntity(ctx, user, CreateEntityInput{
		Code: "ROLLBACK" + mustULID(t)[:6], Name: "Should Roll Back", EntityType: "BUSINESS", Template: "NOT_A_TEMPLATE",
	}); err == nil {
		t.Fatal("expected unknown template to fail")
	}
	var leftover int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE name='Should Roll Back'`).Scan(&leftover); err != nil {
		t.Fatal(err)
	}
	if leftover != 0 {
		t.Fatal("failed chart seed left an entity")
	}
	entity := openingEntity(t, ctx, s, user, TemplateGeneralBusiness, "DIG")
	if _, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{Code: "12", Name: "Bad", Type: "EXPENSE", Postable: true}); err == nil || !strings.Contains(err.Error(), "four digits") {
		t.Fatalf("short code error=%v", err)
	}
}

func TestAccountingStartDateGuards(t *testing.T) {
	ctx, s, user := openingUser(t)
	entity := openingEntity(t, ctx, s, user, TemplateGeneralBusiness, "START")
	cash := accountPublic(t, ctx, s, entity.ID, "1110")
	expense := accountPublic(t, ctx, s, entity.ID, "6110")
	fa := createCashAccount(t, ctx, s, user, entity, cash, "MMK")
	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-01", Description: "Before setup", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "10"}},
	}); err == nil || !strings.Contains(err.Error(), "Set the accounting start date") {
		t.Fatalf("null start error=%v", err)
	}
	start := "2026-09-01"
	if err := s.SetAccountingStartDate(ctx, user, entity, &start); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-08-31", Description: "Too early", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "10"}},
	}); err == nil || !strings.Contains(err.Error(), "before the accounting start date") {
		t.Fatalf("early date error=%v", err)
	}
	onStart, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-01", Description: "On start", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "10"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, onStart.PublicID); err != nil {
		t.Fatal(err)
	}
	after, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-02", Description: "After start", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "10"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, after.PublicID); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, entity, "2026-09-02", "close"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-02", Description: "Locked", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "10"}},
	}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("lock error=%v", err)
	}
	moved := "2026-10-01"
	if err := s.SetAccountingStartDate(ctx, user, entity, &moved); err == nil || !strings.Contains(err.Error(), "fixed") {
		t.Fatalf("start date change error=%v", err)
	}
}

func TestOpeningBalanceJournalsStayImmutable(t *testing.T) {
	ctx, s, user := openingUser(t)
	entity := openingEntity(t, ctx, s, user, TemplateMyanKafeBusiness, "OPEN")
	start := "2026-04-01"
	if err := s.SetAccountingStartDate(ctx, user, entity, &start); err != nil {
		t.Fatal(err)
	}
	cash := accountPublic(t, ctx, s, entity.ID, "1110")
	loan := accountPublic(t, ctx, s, entity.ID, "2220")
	income := accountPublic(t, ctx, s, entity.ID, "4110")
	expense := accountPublic(t, ctx, s, entity.ID, "6110")
	fa := createCashAccount(t, ctx, s, user, entity, cash, "MMK")
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: income, Direction: "CREDIT", Amount: "10",
	}}); err == nil || !strings.Contains(err.Error(), "asset, liability, and equity") {
		t.Fatalf("income opening error=%v", err)
	}

	first, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, FinancialAccountPublicID: fa.PublicID, Direction: "DEBIT", Amount: "5000000",
	}})
	if err != nil {
		t.Fatal(err)
	}
	openingID := first["transaction_id"].(string)
	if systemLineCount(t, ctx, s, entity.ID, SystemRoleOpeningBalanceEquity) != 1 {
		t.Fatal("unbalanced opening should use 3990")
	}
	var faLine string
	if err := s.Pool.QueryRow(ctx, `
SELECT jl.financial_account_id::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1 AND jl.financial_account_id IS NOT NULL`, openingID).Scan(&faLine); err != nil || faLine == "" {
		t.Fatalf("financial account line=%q err=%v", faLine, err)
	}
	original := journalSnapshot(t, ctx, s, openingID)
	again, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, FinancialAccountPublicID: fa.PublicID, Direction: "DEBIT", Amount: "5000000",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if again["unchanged"] != true || transactionCount(t, ctx, s, entity.ID, "OPENING_BALANCE_ADJUSTMENT") != 0 {
		t.Fatalf("unchanged save created an adjustment: %+v", again["unchanged"])
	}
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, FinancialAccountPublicID: fa.PublicID, Direction: "DEBIT", Amount: "5250000",
	}}); err != nil {
		t.Fatal(err)
	}
	if transactionCount(t, ctx, s, entity.ID, "OPENING_BALANCE_ADJUSTMENT") != 1 {
		t.Fatal("expected one adjustment")
	}
	if journalSnapshot(t, ctx, s, openingID) != original {
		t.Fatal("original opening journal changed")
	}
	if systemLineCount(t, ctx, s, entity.ID, SystemRoleOpeningBalanceAdjustment) == 0 {
		t.Fatal("unbalanced adjustment should use 3980")
	}
	var adjustmentID string
	if err := s.Pool.QueryRow(ctx, `SELECT public_id::text FROM transactions WHERE entity_id=$1 AND transaction_type='OPENING_BALANCE_ADJUSTMENT'`, entity.ID).Scan(&adjustmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReverseTransaction(ctx, user, entity, openingID, start, "should fail"); err == nil || !strings.Contains(err.Error(), "Opening Balances") {
		t.Fatalf("opening reversal error=%v", err)
	}
	if _, err := s.ReverseTransaction(ctx, user, entity, adjustmentID, start, "should fail"); err == nil || !strings.Contains(err.Error(), "Opening Balances") {
		t.Fatalf("adjustment reversal error=%v", err)
	}

	balanced := openingEntity(t, ctx, s, user, TemplateGeneralBusiness, "BAL")
	if err := s.SetAccountingStartDate(ctx, user, balanced, &start); err != nil {
		t.Fatal(err)
	}
	balancedCash := accountPublic(t, ctx, s, balanced.ID, "1110")
	balancedLoan := accountPublic(t, ctx, s, balanced.ID, "2220")
	if _, err := s.SaveOpeningBalances(ctx, user, balanced, []OpeningBalanceLineInput{
		{AccountPublicID: balancedCash, Direction: "DEBIT", Amount: "100"},
		{AccountPublicID: balancedLoan, Direction: "CREDIT", Amount: "100"},
	}); err != nil {
		t.Fatal(err)
	}
	if systemLineCount(t, ctx, s, balanced.ID, SystemRoleOpeningBalanceEquity) != 0 {
		t.Fatal("balanced opening used 3990")
	}
	if _, err := s.SaveOpeningBalances(ctx, user, balanced, []OpeningBalanceLineInput{
		{AccountPublicID: balancedCash, Direction: "DEBIT", Amount: "350"},
		{AccountPublicID: balancedLoan, Direction: "CREDIT", Amount: "350"},
	}); err != nil {
		t.Fatal(err)
	}
	if systemLineCount(t, ctx, s, balanced.ID, SystemRoleOpeningBalanceAdjustment) != 0 {
		t.Fatal("self-balancing adjustment used 3980")
	}

	posted, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: start, Description: "Same day rent", FinancialAccountPublicID: fa.PublicID, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expense, Amount: "100000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, posted.PublicID); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTransactionsFiltered(ctx, entity.ID, TransactionListFilter{FinancialAccountID: fa.PublicID, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	var expenseBalance string
	order := []string{}
	for _, item := range list.Items {
		order = append(order, item.Type)
		if item.Type == "EXPENSE" && item.Balance != nil {
			expenseBalance = *item.Balance
		}
	}
	if len(order) < 3 || order[0] != "EXPENSE" {
		t.Fatalf("list order=%v", order)
	}
	ledger, err := s.AccountLedger(ctx, entity.ID, cash, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) < 3 {
		t.Fatalf("ledger rows=%d", len(ledger))
	}
	if !strings.Contains(ledger[0]["journal_description"].(string), "Opening balances") || !strings.Contains(ledger[1]["journal_description"].(string), "adjustment") {
		t.Fatalf("ledger order=%v then %v", ledger[0]["journal_description"], ledger[1]["journal_description"])
	}
	if ledger[len(ledger)-1]["running_balance"] != expenseBalance {
		t.Fatalf("ledger balance=%v list balance=%s", ledger[len(ledger)-1]["running_balance"], expenseBalance)
	}
	profit, err := s.ProfitLoss(ctx, entity.ID, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range profit {
		if row["code"] == "3990" || row["code"] == "3980" || row["code"] == "1110" {
			t.Fatalf("opening account appeared in profit and loss: %v", row["code"])
		}
	}
	var expenseSeen bool
	for _, row := range profit {
		if row["code"] == "6110" {
			expenseSeen = true
		}
	}
	if !expenseSeen {
		t.Fatal("operating expense missing from profit and loss")
	}
	sheet, err := s.BalanceSheet(ctx, entity.ID, time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !reportHasCode(sheet["items"].([]map[string]any), "1110") || !reportHasCode(sheet["items"].([]map[string]any), "3990") {
		t.Fatal("balance sheet missing opening accounts")
	}
	trial, err := s.TrialBalance(ctx, entity.ID, time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !reportHasCode(trial, "1110") || !reportHasCode(trial, "3980") {
		t.Fatal("trial balance missing opening accounts")
	}
	cashRows, err := s.CashMovement(ctx, entity.ID, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range cashRows {
		if row["movement"] == "5000000.000000" || row["movement"] == "250000.000000" {
			t.Fatalf("cash movement included opening activity: %v", row["movement"])
		}
	}
	_ = loan
}

func TestOpeningBalanceLockAndForeignCurrency(t *testing.T) {
	ctx, s, user := openingUser(t)
	entity := openingEntity(t, ctx, s, user, TemplateGeneralBusiness, "LOCK")
	start := "2026-04-01"
	if err := s.SetAccountingStartDate(ctx, user, entity, &start); err != nil {
		t.Fatal(err)
	}
	cash := accountPublic(t, ctx, s, entity.ID, "1110")
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, Direction: "DEBIT", Amount: "100",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, entity, "2026-03-31", "before start"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, Direction: "DEBIT", Amount: "150",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, entity, "2026-04-01", "includes start"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, Direction: "DEBIT", Amount: "175",
	}}); err == nil || !strings.Contains(err.Error(), "Apr 1, 2026") {
		t.Fatalf("locked opening error=%v", err)
	}
	if err := s.Unlock(ctx, user, entity, "OWNER", "reopen"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveOpeningBalances(ctx, user, entity, []OpeningBalanceLineInput{{
		AccountPublicID: cash, Direction: "DEBIT", Amount: "175",
	}}); err != nil {
		t.Fatal(err)
	}

	fxEntity := openingEntity(t, ctx, s, user, TemplateGeneralBusiness, "FXOB")
	if err := s.SetAccountingStartDate(ctx, user, fxEntity, &start); err != nil {
		t.Fatal(err)
	}
	fxCash := accountPublic(t, ctx, s, fxEntity.ID, "1110")
	usd := createCashAccount(t, ctx, s, user, fxEntity, fxCash, "USD")
	if _, err := s.CreateExchangeRate(ctx, user, fxEntity, start, "USD", "MMK", "4000", "MANUAL", "opening"); err != nil {
		t.Fatal(err)
	}
	saved, err := s.SaveOpeningBalances(ctx, user, fxEntity, []OpeningBalanceLineInput{{
		AccountPublicID: fxCash, FinancialAccountPublicID: usd.PublicID, Direction: "DEBIT", Amount: "1000",
	}})
	if err != nil {
		t.Fatal(err)
	}
	openingID := saved["transaction_id"].(string)
	var rate string
	if err := s.Pool.QueryRow(ctx, `
SELECT jl.fx_rate_to_functional::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1 AND jl.financial_account_id IS NOT NULL`, openingID).Scan(&rate); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateExchangeRate(ctx, user, fxEntity, "2026-04-02", "USD", "MMK", "4500", "MANUAL", "later"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveOpeningBalances(ctx, user, fxEntity, []OpeningBalanceLineInput{{
		AccountPublicID: fxCash, FinancialAccountPublicID: usd.PublicID, Direction: "DEBIT", Amount: "1000",
	}}); err != nil {
		t.Fatal(err)
	}
	var rateAfter string
	if err := s.Pool.QueryRow(ctx, `
SELECT jl.fx_rate_to_functional::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1 AND jl.financial_account_id IS NOT NULL`, openingID).Scan(&rateAfter); err != nil {
		t.Fatal(err)
	}
	if rateAfter != rate {
		t.Fatalf("opening FX changed from %s to %s", rate, rateAfter)
	}
	var adjustments int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE entity_id=$1 AND transaction_type='OPENING_BALANCE_ADJUSTMENT'`, fxEntity.ID).Scan(&adjustments); err != nil {
		t.Fatal(err)
	}
	if adjustments != 0 {
		t.Fatalf("later rate created %d adjustments", adjustments)
	}
}

func openingUser(t *testing.T) (context.Context, *Store, User) {
	t.Helper()
	ctx, s := integrationStore(t)
	userID, userPublic := mustUUID(t), mustULID(t)
	username := strings.ToLower("ob-" + userPublic)
	if _, err := s.Pool.Exec(ctx, `INSERT INTO users(id,public_id,username,display_name,platform_owner) VALUES($1,$2,$3,'Opening Tester',true)`, userID, userPublic, username); err != nil {
		t.Fatal(err)
	}
	return ctx, s, User{ID: userID, PublicID: userPublic, Username: username, DisplayName: "Opening Tester", PlatformOwner: true}
}

func openingEntity(t *testing.T, ctx context.Context, s *Store, user User, template, code string) Entity {
	t.Helper()
	entity, err := s.CreateEntity(ctx, user, CreateEntityInput{
		Code: code + mustULID(t)[:8], Name: code + " Entity", EntityType: "BUSINESS",
		FunctionalCurrency: "MMK", Timezone: "Asia/Yangon", FiscalMonth: 4, FiscalDay: 1, Template: template,
	})
	if err != nil {
		t.Fatal(err)
	}
	entity.Role = "OWNER"
	return entity
}

func accountPublic(t *testing.T, ctx context.Context, s *Store, entityID, code string) string {
	t.Helper()
	var publicID string
	if err := s.Pool.QueryRow(ctx, `SELECT public_id::text FROM accounts WHERE entity_id=$1 AND code=$2`, entityID, code).Scan(&publicID); err != nil {
		t.Fatal(err)
	}
	return publicID
}

func createCashAccount(t *testing.T, ctx context.Context, s *Store, user User, entity Entity, accountPublicID, currency string) FinancialAccount {
	t.Helper()
	fa, err := s.CreateFinancialAccount(ctx, user, entity, CreateFinancialAccountInput{
		Code: "CASH" + mustULID(t)[:6], Name: currency + " Cash", Kind: "CASH", Currency: currency, AccountPublicID: accountPublicID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return fa
}

func systemLineCount(t *testing.T, ctx context.Context, s *Store, entityID, role string) int {
	t.Helper()
	var count int
	if err := s.Pool.QueryRow(ctx, `
SELECT count(*)
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
WHERE a.entity_id=$1 AND a.system_role=$2`, entityID, role).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func transactionCount(t *testing.T, ctx context.Context, s *Store, entityID, kind string) int {
	t.Helper()
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE entity_id=$1 AND transaction_type=$2`, entityID, kind).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func journalSnapshot(t *testing.T, ctx context.Context, s *Store, publicID string) string {
	t.Helper()
	rows, err := s.Pool.Query(ctx, `
SELECT jl.line_no::text,jl.account_id::text,COALESCE(jl.financial_account_id::text,''),jl.debit_amount::text,jl.credit_amount::text,jl.fx_rate_to_functional::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1
ORDER BY jl.line_no`, publicID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line, account, fa, debit, credit, rate string
		if err := rows.Scan(&line, &account, &fa, &debit, &credit, &rate); err != nil {
			t.Fatal(err)
		}
		b.WriteString(line + "|" + account + "|" + fa + "|" + debit + "|" + credit + "|" + rate + "\n")
	}
	return b.String()
}

func reportHasCode(rows []map[string]any, code string) bool {
	for _, row := range rows {
		if row["code"] == code {
			return true
		}
	}
	return false
}
