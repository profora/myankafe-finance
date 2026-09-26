package postgres

import "testing"

func TestAuditListFiltersSearchAndPaginates(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, _, _ := seedServiceEntity(t, ctx, s, "AUDIT_FILTER")

	if err := s.Audit(ctx, user, &entity, "TRANSACTION_DRAFT_UPDATE", "TRANSACTION", nil, "SUCCESS", map[string]any{"field": "description"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Audit(ctx, user, &entity, "TRANSACTION_DRAFT_CANCEL", "TRANSACTION", nil, "SUCCESS", map[string]any{"reason": "duplicate"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Audit(ctx, user, &entity, "EXCHANGE_RATE_CREATE", "EXCHANGE_RATE", nil, "FAILED", map[string]any{"rate": "bad"}); err != nil {
		t.Fatal(err)
	}

	drafts, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Action: "TRANSACTION_DRAFT_UPDATE", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if drafts.Count != 1 || len(drafts.Items) != 1 {
		t.Fatalf("draft filter count=%d items=%d", drafts.Count, len(drafts.Items))
	}

	failed, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Outcome: "FAILED", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Count != 1 || len(failed.Items) != 1 {
		t.Fatalf("failed filter count=%d items=%d", failed.Count, len(failed.Items))
	}
	if failed.Items[0]["action"] != "EXCHANGE_RATE_CREATE" {
		t.Fatalf("unexpected failed event: %+v", failed.Items[0])
	}

	search, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Search: "Service Test User", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if search.Count != 3 {
		t.Fatalf("actor search count=%d want 3", search.Count)
	}

	first, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 3 || len(first.Items) != 2 || !first.HasMore {
		t.Fatalf("first page=%+v", first)
	}
	second, err := s.ListAuditEvents(ctx, entity.ID, AuditEventFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.Count != 3 || len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page=%+v", second)
	}
}
