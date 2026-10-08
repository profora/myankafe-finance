package integration

import (
	"testing"
	"time"
)

func TestSignatureAcceptsExactBodyAndRejectsTampering(t *testing.T) {
	secret := "test-secret-not-production"
	body := []byte(`{"external_event_id":"MYANKAFE:SALE:abc:v1","amount":1}`)
	ts := "1710000000"
	sig := Sign(secret, Canonical("POST", "/api/v1/integrations/events", ts, body))
	if err := VerifySignature(secret, "POST", "/api/v1/integrations/events", ts, body, sig); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature("other-secret", "POST", "/api/v1/integrations/events", ts, body, sig); err == nil {
		t.Fatal("wrong secret was accepted")
	}
	altered := []byte(`{"external_event_id":"MYANKAFE:SALE:abc:v1","amount":2}`)
	if err := VerifySignature(secret, "POST", "/api/v1/integrations/events", ts, altered, sig); err == nil {
		t.Fatal("altered body was accepted")
	}
	if err := VerifySignature(secret, "POST", "/api/v1/integrations/events", "", body, sig); err == nil {
		t.Fatal("missing timestamp was accepted")
	}
	now := time.Unix(1710000000, 0)
	if err := VerifyTimestamp(ts, now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTimestamp(ts, now.Add(10*time.Minute)); err == nil {
		t.Fatal("stale timestamp was accepted")
	}
	if err := VerifyTimestamp("", now); err == nil {
		t.Fatal("blank timestamp was accepted")
	}
}

func TestSalePlanBalancesRevenueAndHistoricalCost(t *testing.T) {
	body := []byte(`{
	  "external_event_id":"MYANKAFE:SALE:order:v1",
	  "event_type":"SALE_FINALIZED_V1",
	  "payload_version":1,
	  "accounting_date":"2026-10-09",
	  "payload":{
	    "order_public_id":"order",
	    "source_channel":"MESSENGER",
	    "customer_segment":"RETAILER",
	    "currency":"MMK",
	    "revenue_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","revenue_mmk":165000,"tax_mmk":0}],
	    "delivery_fee_mmk":0,
	    "settlement_adjustments":[],
	    "net_receivable_mmk":165000,
	    "deposit_applied_mmk":0,
	    "inventory_cost_lines":[{"sku":"CHFT-2PLUS1","quantity":"10","historical_cost_mmk":100000}]
	  }
	}`)
	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanEvent(env, testResolver)
	if err != nil {
		t.Fatal(err)
	}
	var ar, revenue, cogs, inventory int64
	for _, line := range plan.Lines {
		switch line.SourceKey {
		case "AR":
			ar += line.Debit
		case "REVENUE:CHFT-2PLUS1":
			revenue += line.Credit
		case "COGS:CHFT-2PLUS1":
			cogs += line.Debit
		case "INVENTORY_ASSET":
			inventory += line.Credit
		}
	}
	if ar != 165000 || revenue != 165000 || cogs != 100000 || inventory != 100000 || plan.Total != 265000 {
		t.Fatalf("lines=%+v total=%d", plan.Lines, plan.Total)
	}
	if plan.SalesChannelKey != "SALES_CHANNEL:MESSENGER" || plan.CustomerSegment != "RETAILER" {
		t.Fatalf("dimensions channel=%s segment=%s", plan.SalesChannelKey, plan.CustomerSegment)
	}
}

func TestMissingRevenueMappingIsRejected(t *testing.T) {
	body := []byte(`{
	  "external_event_id":"MYANKAFE:SALE:order:v1",
	  "event_type":"SALE_FINALIZED_V1",
	  "accounting_date":"2026-10-09",
	  "payload":{
	    "order_public_id":"order","source_channel":"MESSENGER","customer_segment":"CONSUMER","currency":"MMK",
	    "revenue_lines":[{"sku":"UNKNOWN","quantity":"1","revenue_mmk":100,"tax_mmk":0}],
	    "net_receivable_mmk":100
	  }
	}`)
	env, err := ParseEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	_, err = PlanEvent(env, testResolver)
	var ie *Error
	if err == nil || !asIntegration(err, &ie) || ie.Code != "mapping_required" {
		t.Fatalf("err=%v", err)
	}
}

func asIntegration(err error, target **Error) bool {
	ie, ok := err.(*Error)
	if ok {
		*target = ie
	}
	return ok
}

func testResolver(key string) (Target, error) {
	switch key {
	case "AR", "INVENTORY_ASSET":
		return Target{Kind: "ACCOUNT", AccountID: "a", AccountType: "ASSET", Postable: true, Active: true}, nil
	case "REVENUE:CHFT-2PLUS1", "DELIVERY_REVENUE":
		return Target{Kind: "ACCOUNT", AccountID: "i", AccountType: "INCOME", Postable: true, Active: true}, nil
	case "COGS:CHFT-2PLUS1":
		return Target{Kind: "ACCOUNT", AccountID: "e", AccountType: "EXPENSE", Postable: true, Active: true}, nil
	case "SALES_CHANNEL:MESSENGER":
		return Target{Kind: "SALES_CHANNEL", SalesChannelID: "c", Active: true}, nil
	default:
		return Target{}, Mapping(key)
	}
}
