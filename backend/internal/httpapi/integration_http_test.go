package httpapi

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/config"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/integration"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func TestIntegrationHTTPAuthAndSalePosting(t *testing.T) {
	store := testHTTPStore(t)
	ctx := context.Background()
	userID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	userPublic, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	username := strings.ToLower("int-http-" + userPublic)
	if _, err := store.Pool.Exec(ctx, `INSERT INTO users(id,public_id,username,display_name,platform_owner) VALUES($1,$2,$3,'Integration HTTP',true)`, userID, userPublic, username); err != nil {
		t.Fatal(err)
	}
	user := postgres.User{ID: userID, PublicID: userPublic, Username: username, DisplayName: "Integration HTTP", PlatformOwner: true}
	entity, err := store.CreateEntity(ctx, user, postgres.CreateEntityInput{
		Code: "H" + userPublic, Name: "HTTP Entity", EntityType: "BUSINESS",
		FunctionalCurrency: "MMK", Timezone: "Asia/Yangon", FiscalMonth: 4, FiscalDay: 1,
		Template: postgres.TemplateMyanKafeBusiness,
	})
	if err != nil {
		t.Fatal(err)
	}
	entity.Role = "OWNER"
	if _, err := store.Pool.Exec(ctx, `UPDATE entities SET accounting_start_date='2020-01-01' WHERE id=$1`, entity.ID); err != nil {
		t.Fatal(err)
	}
	conn, err := store.CreateIntegrationConnection(ctx, user, entity, "MyanKafe Platform")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE integration_connections SET active=false WHERE id<>$1`, conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateIntegrationConnection(ctx, user, entity, integration.SystemMyanKafePlatform, "", true); err != nil {
		t.Fatal(err)
	}
	channel, err := store.CreateSalesChannel(ctx, user, entity, postgres.SalesChannelInput{Code: "ONLINE", Name: "Online", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	accountOf := func(code string) string {
		t.Helper()
		var publicID string
		if err := store.Pool.QueryRow(ctx, `SELECT public_id::text FROM accounts WHERE entity_id=$1 AND code=$2`, entity.ID, code).Scan(&publicID); err != nil {
			t.Fatal(err)
		}
		return publicID
	}
	maps := map[string]postgres.IntegrationMappingInput{
		"AR":                      {AccountPublicID: accountOf("1210")},
		"INVENTORY_ASSET":         {AccountPublicID: accountOf("1310")},
		"PURCHASE_RECEIPT_CREDIT": {AccountPublicID: accountOf("2110")},
		"TAX_PAYABLE":             {AccountPublicID: accountOf("2130")},
		"CUSTOMER_DEPOSITS":       {AccountPublicID: accountOf("2150")},
		"DELIVERY_REVENUE":        {AccountPublicID: accountOf("4210")},
		"REVENUE:CHFT-2PLUS1":     {AccountPublicID: accountOf("4110")},
		"COGS:CHFT-2PLUS1":        {AccountPublicID: accountOf("5110")},
		"SALES_CHANNEL:MESSENGER": {SalesChannelPublicID: channel.PublicID},
	}
	cash, err := store.CreateFinancialAccount(ctx, user, entity, postgres.CreateFinancialAccountInput{
		Code: "KBZ_TEST", Name: "KBZ test wallet", Kind: "MOBILE_WALLET", Currency: "MMK", AccountPublicID: accountOf("1110"),
	})
	if err != nil {
		t.Fatal(err)
	}
	maps["PAYMENT_METHOD:KBZPAY"] = postgres.IntegrationMappingInput{FinancialAccountPublicID: cash.PublicID}
	for key, in := range maps {
		in.SourceKey = key
		if err := store.PutIntegrationMapping(ctx, user, entity, in); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}

	const secret = "http-test-integration-secret-not-for-production"
	handler := New(store, config.Config{
		AppEnv: "development", AuthMode: "dev", CORSOrigin: "http://localhost:3000",
		IntegrationSecretMyanKafePlatform: secret,
	})
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	saleID := "MYANKAFE:SALE:http-" + userPublic + ":v1"
	sale := []byte(`{"external_event_id":"` + saleID + `","event_type":"SALE_FINALIZED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"order_public_id":"http-order","source_channel":"MESSENGER","customer_segment":"RETAILER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","revenue_mmk":165000,"tax_mmk":0}],"net_receivable_mmk":165000,"inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","historical_cost_mmk":100000}]}}`)
	path := "/api/v1/integrations/events"

	rec := func(system, eventID, signingSecret, timestamp string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set(integration.HeaderSystem, system)
		req.Header.Set(integration.HeaderEventID, eventID)
		if timestamp != "omit" {
			if timestamp == "" {
				timestamp = strconv.FormatInt(time.Now().Unix(), 10)
			}
			req.Header.Set(integration.HeaderTimestamp, timestamp)
		}
		signedAt := timestamp
		if timestamp == "omit" {
			signedAt = ""
		}
		req.Header.Set(integration.HeaderSignature, integration.Sign(signingSecret, integration.Canonical(http.MethodPost, path, signedAt, body)))
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if strings.Contains(out.Body.String(), secret) || strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "v1=") {
			t.Fatalf("secret or signature leaked status=%d logs=%s body=%s", out.Code, logs.String(), out.Body.String())
		}
		return out
	}

	if got := rec("OTHER", saleID, secret, "", sale); got.Code != http.StatusUnauthorized {
		t.Fatalf("unknown system %d %s", got.Code, got.Body.String())
	}
	if got := rec(integration.SystemMyanKafePlatform, saleID, "wrong-secret", "", sale); got.Code != http.StatusUnauthorized || !strings.Contains(got.Body.String(), "bad_signature") {
		t.Fatalf("wrong secret %d %s", got.Code, got.Body.String())
	}
	altered := bytes.Replace(sale, []byte(`165000`), []byte(`165001`), 1)
	alteredReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(altered))
	alteredReq.Header.Set(integration.HeaderSystem, integration.SystemMyanKafePlatform)
	alteredReq.Header.Set(integration.HeaderEventID, saleID)
	alteredStamp := strconv.FormatInt(time.Now().Unix(), 10)
	alteredReq.Header.Set(integration.HeaderTimestamp, alteredStamp)
	alteredReq.Header.Set(integration.HeaderSignature, integration.Sign(secret, integration.Canonical(http.MethodPost, path, alteredStamp, sale)))
	alteredRec := httptest.NewRecorder()
	handler.ServeHTTP(alteredRec, alteredReq)
	if alteredRec.Code != http.StatusUnauthorized {
		t.Fatalf("altered payload %d %s", alteredRec.Code, alteredRec.Body.String())
	}
	if got := rec(integration.SystemMyanKafePlatform, saleID, secret, "omit", sale); got.Code != http.StatusUnauthorized || !strings.Contains(got.Body.String(), "missing_timestamp") {
		t.Fatalf("missing timestamp %d %s", got.Code, got.Body.String())
	}
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if got := rec(integration.SystemMyanKafePlatform, saleID, secret, stale, sale); got.Code != http.StatusUnauthorized || !strings.Contains(got.Body.String(), "stale_timestamp") {
		t.Fatalf("stale timestamp %d %s", got.Code, got.Body.String())
	}

	if _, err := store.UpdateIntegrationConnection(ctx, user, entity, integration.SystemMyanKafePlatform, "", false); err != nil {
		t.Fatal(err)
	}
	if got := rec(integration.SystemMyanKafePlatform, saleID, secret, "", sale); got.Code != http.StatusForbidden {
		t.Fatalf("inactive %d %s", got.Code, got.Body.String())
	}
	if _, err := store.UpdateIntegrationConnection(ctx, user, entity, integration.SystemMyanKafePlatform, "", true); err != nil {
		t.Fatal(err)
	}

	posted := rec(integration.SystemMyanKafePlatform, saleID, secret, "", sale)
	if posted.Code != http.StatusOK || !strings.Contains(posted.Body.String(), `"status":"POSTED"`) {
		t.Fatalf("post %d %s", posted.Code, posted.Body.String())
	}
	replay := rec(integration.SystemMyanKafePlatform, saleID, secret, "", sale)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	changed := []byte(`{"external_event_id":"` + saleID + `","event_type":"SALE_FINALIZED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"order_public_id":"http-order","source_channel":"MESSENGER","customer_segment":"RETAILER","currency":"MMK","revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","revenue_mmk":1,"tax_mmk":0}],"net_receivable_mmk":1,"inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","historical_cost_mmk":1}]}}`)
	conflict := rec(integration.SystemMyanKafePlatform, saleID, secret, "", changed)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict %d %s", conflict.Code, conflict.Body.String())
	}

	var journals int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM journal_entries je JOIN transactions tr ON tr.id=je.transaction_id WHERE tr.external_reference=$1`, saleID).Scan(&journals); err != nil {
		t.Fatal(err)
	}
	if journals != 1 {
		t.Fatalf("journals=%d", journals)
	}
	assertAmount := func(code, debit, credit string) {
		t.Helper()
		var gotDebit, gotCredit string
		if err := store.Pool.QueryRow(ctx, `
SELECT COALESCE(sum(jl.debit_amount),0)::text, COALESCE(sum(jl.credit_amount),0)::text
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions tr ON tr.id=je.transaction_id
WHERE tr.external_reference=$2 AND a.code=$1`, code, saleID).Scan(&gotDebit, &gotCredit); err != nil {
			t.Fatal(err)
		}
		if gotDebit != debit || gotCredit != credit {
			t.Fatalf("%s debit=%s credit=%s", code, gotDebit, gotCredit)
		}
	}
	assertAmount("1210", "165000.000000", "0.000000")
	assertAmount("4110", "0.000000", "165000.000000")
	assertAmount("5110", "100000.000000", "0.000000")
	assertAmount("1310", "0.000000", "100000.000000")

	paymentID := "MYANKAFE:PAYMENT:" + userPublic + ":VERIFIED:v1"
	payment := []byte(`{"external_event_id":"` + paymentID + `","event_type":"PAYMENT_VERIFIED_V1","payload_version":1,"accounting_date":"2026-10-09","payload":{"payment_public_id":"pay-http","order_public_id":"http-order","method":"KBZPAY","amount_mmk":50000,"currency":"MMK","settlement_target":"AR"}}`)
	paid := rec(integration.SystemMyanKafePlatform, paymentID, secret, "", payment)
	if paid.Code != http.StatusOK {
		t.Fatalf("payment %d %s", paid.Code, paid.Body.String())
	}
	var faCredit string
	if err := store.Pool.QueryRow(ctx, `
SELECT COALESCE(sum(jl.credit_amount),0)::text
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions tr ON tr.id=je.transaction_id
WHERE tr.external_reference=$1 AND a.code='1210'`, paymentID).Scan(&faCredit); err != nil {
		t.Fatal(err)
	}
	if faCredit != "50000.000000" {
		t.Fatalf("payment AR credit=%s", faCredit)
	}
}
