package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func createHeaderAccount(t *testing.T, ctx context.Context, s *Store, user User, entity Entity, accountType, name string) Account {
	t.Helper()
	account, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: name, Type: accountType, Postable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func TestReparentAccountPreservesJournalAndAudit(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "REPARENT")
	header := createHeaderAccount(t, ctx, s, user, entity, "EXPENSE", "Expense Header")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type: "EXPENSE", Date: "2026-09-24", Description: "Reparent proof",
		FinancialAccountPublicID: financialAccount, Currency: "MMK",
		Splits: []SplitInput{{AccountPublicID: expenseAccount, Amount: "2500", Description: "proof"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
		t.Fatal(err)
	}
	before := journalSnapshot(t, ctx, s, draft.PublicID)
	if before == "" {
		t.Fatal("expected posted journal lines")
	}

	var code, accountType string
	var postable bool
	if err := s.Pool.QueryRow(ctx, `SELECT code,account_type,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2`, entity.ID, expenseAccount).Scan(&code, &accountType, &postable); err != nil {
		t.Fatal(err)
	}

	moved, err := s.UpdateAccount(ctx, user, entity, expenseAccount, UpdateAccountInput{
		Name: "Test Expense", ParentPublicID: header.PublicID, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.ParentPublicID == nil || *moved.ParentPublicID != header.PublicID {
		t.Fatalf("parent = %v", moved.ParentPublicID)
	}
	if moved.Code != code || moved.Type != accountType || moved.Postable != postable {
		t.Fatalf("identity changed: %+v", moved)
	}
	if journalSnapshot(t, ctx, s, draft.PublicID) != before {
		t.Fatal("journal lines changed after re-parent")
	}

	rooted, err := s.UpdateAccount(ctx, user, entity, expenseAccount, UpdateAccountInput{
		Name: "Test Expense", ParentPublicID: "", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rooted.ParentPublicID != nil {
		t.Fatalf("expected root, parent %v", rooted.ParentPublicID)
	}
	if journalSnapshot(t, ctx, s, draft.PublicID) != before {
		t.Fatal("journal lines changed after moving to root")
	}

	var after string
	if err := s.Pool.QueryRow(ctx, `
SELECT after_data::text
FROM audit_events
WHERE entity_id=$1 AND action='COA_UPDATE' AND resource_public_id=$2
ORDER BY occurred_at DESC
LIMIT 1`, entity.ID, expenseAccount).Scan(&after); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(after), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["old_parent_public_id"] != header.PublicID || payload["new_parent_public_id"] != "" {
		t.Fatalf("audit parents = %#v", payload)
	}
	if _, ok := payload["request"]; ok {
		t.Fatal("audit stored a request body")
	}
}

func TestReparentValidation(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, _ := seedServiceEntity(t, ctx, s, "REPARENT_RULES")
	otherUser, otherEntity, _, _ := seedServiceEntity(t, ctx, s, "REPARENT_OTHER")
	expenseHeader := createHeaderAccount(t, ctx, s, user, entity, "EXPENSE", "Expense Header")
	assetHeader := createHeaderAccount(t, ctx, s, user, entity, "ASSET", "Asset Header")
	otherHeader := createHeaderAccount(t, ctx, s, otherUser, otherEntity, "EXPENSE", "Other Header")
	inactive := createHeaderAccount(t, ctx, s, user, entity, "EXPENSE", "Inactive Header")
	if _, err := s.UpdateAccount(ctx, user, entity, inactive.PublicID, UpdateAccountInput{Name: inactive.Name, Active: false}); err != nil {
		t.Fatal(err)
	}
	posting, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Posting Expense", Type: "EXPENSE", Postable: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		parent string
		want   string
	}{
		{"other entity", otherHeader.PublicID, "not found"},
		{"different type", assetHeader.PublicID, "same fundamental account type"},
		{"inactive parent", inactive.PublicID, "inactive"},
		{"posting parent", posting.PublicID, "header"},
		{"self parent", expenseHeader.PublicID, "own parent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := expenseAccount
			if tc.name == "self parent" {
				target = expenseHeader.PublicID
			}
			_, err := s.UpdateAccount(ctx, user, entity, target, UpdateAccountInput{
				Name: "Moved", ParentPublicID: tc.parent, Active: true,
			})
			if err == nil || (tc.want != "" && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("error = %v", err)
			}
		})
	}

	child, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Child Header", Type: "EXPENSE", ParentPublicID: expenseHeader.PublicID, Postable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAccount(ctx, user, entity, expenseHeader.PublicID, UpdateAccountInput{
		Name: expenseHeader.Name, ParentPublicID: child.PublicID, Active: true,
	}); err == nil || !strings.Contains(err.Error(), "descendant") {
		t.Fatalf("direct cycle error = %v", err)
	}

	grandchild, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Grandchild Header", Type: "EXPENSE", ParentPublicID: child.PublicID, Postable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAccount(ctx, user, entity, expenseHeader.PublicID, UpdateAccountInput{
		Name: expenseHeader.Name, ParentPublicID: grandchild.PublicID, Active: true,
	}); err == nil || !strings.Contains(err.Error(), "descendant") {
		t.Fatalf("deep cycle error = %v", err)
	}
}

func TestSubtreeMoveKeepsDescendants(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "SUBTREE")
	root := createHeaderAccount(t, ctx, s, user, entity, "EXPENSE", "Root")
	branch, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Branch", Type: "EXPENSE", ParentPublicID: root.PublicID, Postable: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := s.CreateAccount(ctx, user, entity, CreateAccountInput{
		Code: nextAccountCode(t), Name: "Leaf", Type: "EXPENSE", ParentPublicID: branch.PublicID, Postable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	destination := createHeaderAccount(t, ctx, s, user, entity, "EXPENSE", "Destination")
	moved, err := s.UpdateAccount(ctx, user, entity, branch.PublicID, UpdateAccountInput{
		Name: branch.Name, ParentPublicID: destination.PublicID, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.ParentPublicID == nil || *moved.ParentPublicID != destination.PublicID {
		t.Fatalf("branch parent = %v", moved.ParentPublicID)
	}
	var leafParent string
	if err := s.Pool.QueryRow(ctx, `
SELECT p.public_id::text
FROM accounts a JOIN accounts p ON p.id=a.parent_id
WHERE a.entity_id=$1 AND a.public_id=$2`, entity.ID, leaf.PublicID).Scan(&leafParent); err != nil {
		t.Fatal(err)
	}
	if leafParent != branch.PublicID {
		t.Fatalf("leaf parent = %s", leafParent)
	}
}

func TestSystemAccountHierarchyIsFixed(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "SYSTEM_COA")
	equity := createHeaderAccount(t, ctx, s, user, entity, "EQUITY", "Equity Header")
	other := createHeaderAccount(t, ctx, s, user, entity, "EQUITY", "Other Equity")
	publicID := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,parent_id,account_type,system_role,is_postable,is_system,created_by)
SELECT $1,$2,$3,$4,'Opening Balance Equity',id,'EQUITY','OPENING_BALANCE_EQUITY',true,true,$5
FROM accounts WHERE entity_id=$3 AND public_id=$6`,
		mustUUID(t), publicID, entity.ID, nextAccountCode(t), user.ID, equity.PublicID); err != nil {
		t.Fatal(err)
	}

	if _, err := s.UpdateAccount(ctx, user, entity, publicID, UpdateAccountInput{
		Name: "Opening Balance Equity", ParentPublicID: other.PublicID, Active: true,
	}); err == nil || !strings.Contains(err.Error(), "system account hierarchy is fixed") {
		t.Fatalf("error = %v", err)
	}

	updated, err := s.UpdateAccount(ctx, user, entity, publicID, UpdateAccountInput{
		Name: "Opening Balance Equity", ParentPublicID: equity.PublicID, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HierarchyLocked || updated.ParentPublicID == nil || *updated.ParentPublicID != equity.PublicID {
		t.Fatalf("system account = %+v", updated)
	}
	var role string
	if err := s.Pool.QueryRow(ctx, `SELECT system_role FROM accounts WHERE entity_id=$1 AND public_id=$2`, entity.ID, publicID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != SystemRoleOpeningBalanceEquity {
		t.Fatalf("system role = %s", role)
	}
}
