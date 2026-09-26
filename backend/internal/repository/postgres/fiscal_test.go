package postgres

import "testing"

func TestFiscalYearLabelAndValidation(t *testing.T) {
	label, err := FiscalYearLabel(4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if label != "Apr 1 → Mar 31" {
		t.Fatalf("label=%q", label)
	}
	if err := ValidateFiscalStart(4, 1); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFiscalStart(4, 31); err == nil {
		t.Fatal("April 31 was accepted")
	}
	if err := ValidateFiscalStart(2, 30); err == nil {
		t.Fatal("February 30 was accepted")
	}
	if err := ValidateFiscalStart(2, 29); err == nil {
		t.Fatal("February 29 was accepted")
	}
}
