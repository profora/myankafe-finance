package postgres

import (
	"context"
	"testing"
	"time"
)

func seedIncomeAccount(t *testing.T, ctx context.Context, s *Store, entity Entity, user User, name string) string {
	t.Helper()
	id := mustUUID(t)
	pub := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,'INCOME',true,$6)`,
		id, pub, entity.ID, nextAccountCode(t), name, user.ID); err != nil {
		t.Fatal(err)
	}
	return pub
}

func seedSegmentedContact(t *testing.T, ctx context.Context, s *Store, entity Entity, user User, segment string) string {
	t.Helper()
	id := mustUUID(t)
	pub := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO contacts(id,public_id,entity_id,contact_type,display_name,customer_segment,created_by)
VALUES($1,$2,$3,'OTHER',$4,$5,$6)`,
		id, pub, entity.ID, "Sales Dimension Contact "+pub, segment, user.ID); err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestIncomeSalesDimensionsAndReversalStayInSameRoute(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, financialAccount := seedServiceEntity(t, ctx, s, "SALES_DIM")
	incomeAccount := seedIncomeAccount(t, ctx, s, entity, user, "2Plus1 Sales")
	contact := seedSegmentedContact(t, ctx, s, entity, user, "RETAILER")

	channel, err := s.CreateSalesChannel(ctx, user, entity, SalesChannelInput{
		Code: "ONLINE", Name: "Online", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "INCOME",
		Date:                     "2026-10-05",
		Description:              "retailer ordered online",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		ContactPublicID:          contact,
		SalesChannelPublicID:     channel.PublicID,
		Splits:                   []SplitInput{{AccountPublicID: incomeAccount, Amount: "125000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	rows, err := s.SalesAnalysis(ctx, entity.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("sales rows=%d want 1: %+v", len(rows), rows)
	}
	if rows[0]["route_code"] != "RETAILER" ||
		rows[0]["sales_channel_code"] != "ONLINE" ||
		rows[0]["customer_segment"] != "RETAILER" ||
		rows[0]["amount"] != "125000.000000" {
		t.Fatalf("unexpected sales analysis: %+v", rows[0])
	}

	if _, err := s.Pool.Exec(ctx, `
UPDATE contacts SET customer_segment='CONSUMER' WHERE entity_id=$1 AND public_id=$2`, entity.ID, contact); err != nil {
		t.Fatal(err)
	}
	rows, err = s.SalesAnalysis(ctx, entity.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["route_code"] != "RETAILER" || rows[0]["customer_segment"] != "RETAILER" {
		t.Fatalf("posted sale attribution changed after contact reclassification: %+v", rows)
	}

	if _, err := s.ReverseTransaction(ctx, user, entity, draft.PublicID, "2026-10-06", "customer cancelled"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.SalesAnalysis(ctx, entity.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("reversed sale must net to zero, got %+v", rows)
	}
}

func TestSalesChannelValidation(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "SALES_VALID")
	channel, err := s.CreateSalesChannel(ctx, user, entity, SalesChannelInput{
		Code: "WALK_IN", Name: "Walk-in", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-10-05",
		Description:              "not a sale",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		SalesChannelPublicID:     channel.PublicID,
		Splits:                   []SplitInput{{AccountPublicID: expenseAccount, Amount: "1000"}},
	}); err == nil {
		t.Fatal("expected sales channel on expense to be rejected")
	}

	incomeAccount := seedIncomeAccount(t, ctx, s, entity, user, "Test Income")
	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "INCOME",
		Date:                     "2026-10-05",
		Description:              "missing required channel",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits:                   []SplitInput{{AccountPublicID: incomeAccount, Amount: "1000"}},
	}); err == nil {
		t.Fatal("expected income without a channel to be rejected while active channels exist")
	}

	if _, err := s.UpdateSalesChannel(ctx, user, entity, channel.Code, channel.Name, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "INCOME",
		Date:                     "2026-10-05",
		Description:              "inactive channel",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		SalesChannelPublicID:     channel.PublicID,
		Splits:                   []SplitInput{{AccountPublicID: incomeAccount, Amount: "1000"}},
	}); err == nil {
		t.Fatal("expected inactive sales channel to be rejected")
	}
}
