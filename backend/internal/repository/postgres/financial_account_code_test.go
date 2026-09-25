package postgres

import "testing"

func TestNormalizeFinancialAccountCodeCollapsesWhitespace(t *testing.T) {
	got := NormalizeFinancialAccountCode("  test  cash box ")
	if got != "_TEST_CASH_BOX_" {
		t.Fatalf("got %q", got)
	}
}
