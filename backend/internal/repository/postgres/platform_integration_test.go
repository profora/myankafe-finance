package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/integration"
)

func TestPlatformSalePostsOnceAndConflictsOnPayloadChange(t *testing.T) {
	ctx, s, user := openingUser(t)
	entity := openingEntity(t, ctx, s, user, TemplateMyanKafeBusiness, "INT")
	if _, err := s.Pool.Exec(ctx, `UPDATE entities SET accounting_start_date='2020-01-01' WHERE id=$1`, entity.ID); err != nil {
		t.Fatal(err)
	}
	conn, err := s.CreateIntegrationConnection(ctx, user, entity, "MyanKafe Platform")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateIntegrationConnection(ctx, user, entity, integration.SystemMyanKafePlatform, "", true); err != nil {
		t.Fatal(err)
	}
	channel, err := s.CreateSalesChannel(ctx, user, entity, SalesChannelInput{Code: "ONLINE", Name: "Online", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	maps := map[string]IntegrationMappingInput{
		"AR":                      {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "1210")},
		"INVENTORY_ASSET":         {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "1310")},
		"PURCHASE_RECEIPT_CREDIT": {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "2110")},
		"TAX_PAYABLE":             {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "2130")},
		"CUSTOMER_DEPOSITS":       {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "2150")},
		"DELIVERY_REVENUE":        {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "4210")},
		"REVENUE:CHFT-2PLUS1":     {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "4110")},
		"COGS:CHFT-2PLUS1":        {AccountPublicID: accountPublic(t, ctx, s, entity.ID, "5110")},
		"SALES_CHANNEL:MESSENGER": {SalesChannelPublicID: channel.PublicID},
	}
	for key, in := range maps {
		in.SourceKey = key
		if err := s.PutIntegrationMapping(ctx, user, entity, in); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	body := []byte(`{"external_event_id":"MYANKAFE:SALE:order-e2e:v1","event_type":"SALE_FINALIZED_V1","payload_version":1,"occurred_at":"2026-10-09T12:00:00+06:30","accounting_date":"2026-10-09","payload":{"order_public_id":"order-e2e","accounting_date":"2026-10-09","source_channel":"MESSENGER","customer_segment":"RETAILER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","revenue_mmk":165000,"tax_mmk":0}],"delivery_fee_mmk":0,"settlement_adjustments":[],"net_receivable_mmk":165000,"deposit_applied_mmk":0,"inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","historical_cost_mmk":100000}]}}`)
	first, err := s.PostIntegrationEvent(ctx, conn, "MMK", body)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "POSTED" || first.Replayed || first.TransactionPublicID == "" {
		t.Fatalf("first=%+v", first)
	}
	assertLine := func(code string, debit, credit string) {
		t.Helper()
		var gotDebit, gotCredit string
		err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(sum(jl.debit_amount),0)::text, COALESCE(sum(jl.credit_amount),0)::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN accounts a ON a.id=jl.account_id
JOIN transactions tr ON tr.id=je.transaction_id
WHERE tr.public_id=$1 AND a.code=$2`, first.TransactionPublicID, code).Scan(&gotDebit, &gotCredit)
		if err != nil {
			t.Fatal(err)
		}
		if gotDebit != debit || gotCredit != credit {
			t.Fatalf("%s debit=%s credit=%s", code, gotDebit, gotCredit)
		}
	}
	assertLine("1210", "165000.000000", "0.000000")
	assertLine("4110", "0.000000", "165000.000000")
	assertLine("5110", "100000.000000", "0.000000")
	assertLine("1310", "0.000000", "100000.000000")
	var sourceType, sourceSystem, segment, channelCode string
	if err := s.Pool.QueryRow(ctx, `
SELECT tr.source_type, tr.source_system, tr.customer_segment_snapshot, sc.code
FROM transactions tr
JOIN sales_channels sc ON sc.id=tr.sales_channel_id
WHERE tr.public_id=$1`, first.TransactionPublicID).Scan(&sourceType, &sourceSystem, &segment, &channelCode); err != nil {
		t.Fatal(err)
	}
	if sourceType != "INTEGRATION" || sourceSystem != "MYANKAFE_PLATFORM" || segment != "RETAILER" || channelCode != "ONLINE" {
		t.Fatalf("source=%s/%s segment=%s channel=%s", sourceType, sourceSystem, segment, channelCode)
	}
	again, err := s.PostIntegrationEvent(ctx, conn, "MMK", body)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.TransactionPublicID != first.TransactionPublicID {
		t.Fatalf("replay=%+v", again)
	}
	var txCount, journalCount int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE source_type='INTEGRATION' AND entity_id=$1`, entity.ID).Scan(&txCount); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM journal_entries je JOIN transactions tr ON tr.id=je.transaction_id WHERE tr.entity_id=$1 AND tr.source_type='INTEGRATION'`, entity.ID).Scan(&journalCount); err != nil {
		t.Fatal(err)
	}
	if txCount != 1 || journalCount != 1 {
		t.Fatalf("transactions=%d journals=%d", txCount, journalCount)
	}
	changed := []byte(`{"external_event_id":"MYANKAFE:SALE:order-e2e:v1","event_type":"SALE_FINALIZED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"order_public_id":"order-e2e","source_channel":"MESSENGER","customer_segment":"RETAILER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","revenue_mmk":1,"tax_mmk":0}],"net_receivable_mmk":1,"inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","historical_cost_mmk":1}]}}`)
	_, err = s.PostIntegrationEvent(ctx, conn, "MMK", changed)
	var ie *integration.Error
	if !errors.As(err, &ie) || ie.Status != 409 {
		t.Fatalf("conflict err=%v", err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE source_type='INTEGRATION' AND entity_id=$1`, entity.ID).Scan(&txCount); err != nil {
		t.Fatal(err)
	}
	if txCount != 1 {
		t.Fatalf("conflict created transactions=%d", txCount)
	}

	if _, err := s.Pool.Exec(ctx, `DELETE FROM integration_mappings WHERE source_key='REVENUE:CHFT-2PLUS1' AND integration_connection_id=$1`, conn.ID); err != nil {
		t.Fatal(err)
	}
	missingBody := []byte(`{"external_event_id":"MYANKAFE:SALE:order-missing:v1","event_type":"SALE_FINALIZED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"order_public_id":"order-missing","source_channel":"MESSENGER","customer_segment":"CONSUMER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"1","revenue_mmk":165000,"tax_mmk":0}],"net_receivable_mmk":165000,"inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"1","historical_cost_mmk":100000}]}}`)
	_, err = s.PostIntegrationEvent(ctx, conn, "MMK", missingBody)
	if !errors.As(err, &ie) || ie.Code != "mapping_required" {
		t.Fatalf("missing mapping err=%v", err)
	}
	var failedJournals int
	if err := s.Pool.QueryRow(ctx, `
SELECT count(*) FROM journal_entries je
JOIN transactions tr ON tr.id=je.transaction_id
WHERE tr.external_reference='MYANKAFE:SALE:order-missing:v1'`).Scan(&failedJournals); err != nil {
		t.Fatal(err)
	}
	if failedJournals != 0 {
		t.Fatalf("missing mapping created journals=%d", failedJournals)
	}
	revenue := maps["REVENUE:CHFT-2PLUS1"]
	revenue.SourceKey = "REVENUE:CHFT-2PLUS1"
	if err := s.PutIntegrationMapping(ctx, user, entity, revenue); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.PostIntegrationEvent(ctx, conn, "MMK", missingBody)
	if err != nil || recovered.Status != "POSTED" || recovered.Replayed {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}

	receipt := []byte(`{"external_event_id":"MYANKAFE:PURCHASE_RECEIPT:grn:v1","event_type":"PURCHASE_RECEIPT_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"purchase_receipt_public_id":"grn","currency":"MMK","lines":[{"sku":"CHFT-2PLUS1","quantity":"50","value_mmk":600000}]}}`)
	postedReceipt, err := s.PostIntegrationEvent(ctx, conn, "MMK", receipt)
	if err != nil {
		t.Fatal(err)
	}
	assertReceipt := func(publicID, code, debit, credit string) {
		t.Helper()
		var gotDebit, gotCredit string
		if err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(sum(jl.debit_amount),0)::text, COALESCE(sum(jl.credit_amount),0)::text
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions tr ON tr.id=je.transaction_id
WHERE tr.public_id=$1 AND a.code=$2`, publicID, code).Scan(&gotDebit, &gotCredit); err != nil {
			t.Fatal(err)
		}
		if gotDebit != debit || gotCredit != credit {
			t.Fatalf("receipt %s debit=%s credit=%s", code, gotDebit, gotCredit)
		}
	}
	assertReceipt(postedReceipt.TransactionPublicID, "1310", "600000.000000", "0.000000")
	assertReceipt(postedReceipt.TransactionPublicID, "2110", "0.000000", "600000.000000")

	ret := []byte(`{"external_event_id":"MYANKAFE:SALES_RETURN:ret:v1","event_type":"SALES_RETURN_RESTOCK_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"return_public_id":"ret","order_public_id":"order-e2e","currency":"MMK","lines":[{"sku":"CHFT-2PLUS1","quantity":"4","historical_cost_mmk":40000}]}}`)
	postedReturn, err := s.PostIntegrationEvent(ctx, conn, "MMK", ret)
	if err != nil {
		t.Fatal(err)
	}
	assertReceipt(postedReturn.TransactionPublicID, "1310", "40000.000000", "0.000000")
	assertReceipt(postedReturn.TransactionPublicID, "5110", "0.000000", "40000.000000")

	if err := s.Lock(ctx, user, entity, "2026-10-09", "integration test lock"); err != nil {
		t.Fatal(err)
	}
	lockedBody := []byte(`{"external_event_id":"MYANKAFE:SALE:locked:v1","event_type":"SALE_FINALIZED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"order_public_id":"locked","source_channel":"MESSENGER","customer_segment":"CONSUMER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"1","revenue_mmk":1000,"tax_mmk":0}],"net_receivable_mmk":1000}}`)
	_, err = s.PostIntegrationEvent(ctx, conn, "MMK", lockedBody)
	if !errors.As(err, &ie) || ie.Code != "accounting_locked" {
		t.Fatalf("lock err=%v", err)
	}
	if err := s.Unlock(ctx, user, entity, "OWNER", "integration test unlock"); err != nil {
		t.Fatal(err)
	}
	unlocked, err := s.PostIntegrationEvent(ctx, conn, "MMK", lockedBody)
	if err != nil || unlocked.Status != "POSTED" {
		t.Fatalf("unlocked=%+v err=%v", unlocked, err)
	}

	rows, err := s.SalesAnalysis(ctx, entity.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, row := range rows {
		if row["code"] == "4110" && row["route_code"] == "RETAILER" && row["sales_channel_code"] == "ONLINE" && row["customer_segment"] == "RETAILER" && row["amount"] == "165000.000000" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sales analysis rows=%v", rows)
	}
}
