package postgres

import "testing"

func TestInactivePostingAccountCannotBeUsedForNewTransaction(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "COA_INACTIVE")

	updated, err := s.UpdateAccount(ctx, user, entity, expenseAccount, UpdateAccountInput{
		Name: "Archived Expense", Subtype: "", Active: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active {
		t.Fatal("expected account inactive")
	}

	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-24", Description: "Should reject inactive expense",
		FinancialAccountPublicID: financialAccount, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	}); err == nil {
		t.Fatal("expected inactive posting account to be rejected")
	}
}

func TestCannotDeactivateAccountWithActiveFinancialAccount(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, financialAccount := seedServiceEntity(t, ctx, s, "COA_FIN_LINK")

	var linkedAccount string
	if err := s.Pool.QueryRow(ctx, `
SELECT a.public_id::text
FROM financial_accounts fa
JOIN accounts a ON a.id=fa.account_id
WHERE fa.entity_id=$1 AND fa.public_id=$2`, entity.ID, financialAccount).Scan(&linkedAccount); err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateAccount(ctx, user, entity, linkedAccount, UpdateAccountInput{
		Name: "Cash Account", Active: false,
	}); err == nil {
		t.Fatal("expected linked account deactivation to be rejected")
	}
}

func TestCannotDeactivateAccountWithActiveDescendant(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "COA_CHILD")

	parent, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Header", Type: "EXPENSE", Postable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Child", Type: "EXPENSE", ParentPublicID: parent.PublicID, Postable: true,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateAccount(ctx, user, entity, parent.PublicID, UpdateAccountInput{
		Name: "Header", Active: false,
	}); err == nil {
		t.Fatal("expected parent with active descendant to be rejected")
	}
}
