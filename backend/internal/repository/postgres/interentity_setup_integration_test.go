package postgres

import (
	"strings"
	"testing"
)

func TestInterEntityPairAndPostingGuards(t *testing.T) {
	ctx, s := integrationStore(t)
	user, payer, _, payerCash := seedServiceEntity(t, ctx, s, "PAIR")
	_, counterparty, counterpartyExpense, _ := seedServiceEntity(t, ctx, s, "PAIRB")
	_, payerDueFrom := seedTypedAccountCommitted(t, ctx, s, payer, user, "ASSET", "DUE_FROM")
	_, payerDueTo := seedTypedAccountCommitted(t, ctx, s, payer, user, "LIABILITY", "DUE_TO")
	_, counterpartyDueFrom := seedTypedAccountCommitted(t, ctx, s, counterparty, user, "ASSET", "DUE_FROM")
	_, counterpartyDueTo := seedTypedAccountCommitted(t, ctx, s, counterparty, user, "LIABILITY", "DUE_TO")

	if _, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-20", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "100000",
		CounterpartyAmount: "100000", CounterpartyExpenseAccountPublicID: counterpartyExpense, Description: "Missing setup",
	}); err == nil || !strings.Contains(err.Error(), "has not been set up") {
		t.Fatalf("missing setup err=%v", err)
	}
	assertNoTransaction(t, ctx, s, payer.ID, "Missing setup")

	if _, err := s.SaveInterEntityPair(ctx, user, payer, payer, InterEntityPairInput{}); err == nil {
		t.Fatal("self counterparty was accepted")
	}
	if _, err := s.SaveInterEntityPair(ctx, user, payer, counterparty, InterEntityPairInput{
		PayerDueFromAccountID: counterpartyDueFrom, PayerDueToAccountID: payerDueTo,
		CounterpartyDueFromAccountID: counterpartyDueFrom, CounterpartyDueToAccountID: counterpartyDueTo,
	}); err == nil {
		t.Fatal("wrong-entity account was accepted")
	}
	var stored int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM inter_entity_account_mappings WHERE entity_id=$1`, payer.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Fatalf("failed validation stored mappings=%d", stored)
	}

	if _, err := s.SaveInterEntityPair(ctx, user, payer, counterparty, InterEntityPairInput{
		PayerDueFromAccountID: payerDueFrom, PayerDueToAccountID: payerDueTo,
		CounterpartyDueFromAccountID: counterpartyDueFrom, CounterpartyDueToAccountID: counterpartyDueTo,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInterEntityPair(ctx, user, payer, counterparty, InterEntityPairInput{
		PayerDueFromAccountID: payerDueFrom, PayerDueToAccountID: payerDueTo,
		CounterpartyDueFromAccountID: counterpartyExpense, CounterpartyDueToAccountID: counterpartyDueTo,
	}); err == nil {
		t.Fatal("invalid pair update was accepted")
	}
	setup, err := s.ListInterEntitySetup(ctx, payer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(setup) != 1 {
		t.Fatalf("setup=%d", len(setup))
	}
	cpFrom := setup[0]["counterparty_due_from_account"].(map[string]any)
	if cpFrom["id"] != counterpartyDueFrom {
		t.Fatalf("partial update persisted: %+v", setup[0])
	}

	if _, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-20", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "100000",
		CounterpartyAmount: "99000", CounterpartyExpenseAccountPublicID: counterpartyExpense, Description: "Unequal same currency",
	}); err == nil || !strings.Contains(err.Error(), "one amount") {
		t.Fatalf("unequal same-currency err=%v", err)
	}
	posted, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-20", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "100000",
		CounterpartyAmount: "100000", CounterpartyExpenseAccountPublicID: counterpartyExpense, Description: "Pay for counterparty",
	})
	if err != nil {
		t.Fatal(err)
	}
	if posted["initiating_transaction_id"] == "" || posted["counterparty_transaction_id"] == "" {
		t.Fatalf("posted=%+v", posted)
	}

	if err := s.Lock(ctx, user, payer, "2026-09-26", "payer lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-26", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "10",
		CounterpartyAmount: "10", CounterpartyExpenseAccountPublicID: counterpartyExpense, Description: "Locked payer",
	}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("locked payer err=%v", err)
	}
	assertNoTransaction(t, ctx, s, payer.ID, "Locked payer")
	assertNoTransaction(t, ctx, s, counterparty.ID, "Locked payer")
	if err := s.Unlock(ctx, user, payer, "OWNER", "release payer"); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, counterparty, "2026-09-27", "counterparty lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-27", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "10",
		CounterpartyAmount: "10", CounterpartyExpenseAccountPublicID: counterpartyExpense, Description: "Locked counterparty",
	}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("locked counterparty err=%v", err)
	}
	assertNoTransaction(t, ctx, s, payer.ID, "Locked counterparty")

	var audits int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action='INTER_ENTITY_PAIR_SAVE' AND entity_id=$1`, payer.ID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("pair audits=%d", audits)
	}
}
