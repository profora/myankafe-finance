package accounting

import "testing"

func TestBalanced(t *testing.T) {
  a,_:=ParseAmount("100")
  z,_:=ParseAmount("0")
  if err:=ValidateBalanced([]Line{{AccountID:"a",Debit:a,Credit:z},{AccountID:"b",Debit:z,Credit:a}}); err!=nil { t.Fatal(err) }
}

func TestUnbalanced(t *testing.T) {
  a,_:=ParseAmount("100"); b,_:=ParseAmount("99"); z,_:=ParseAmount("0")
  if err:=ValidateBalanced([]Line{{AccountID:"a",Debit:a,Credit:z},{AccountID:"b",Debit:z,Credit:b}}); err==nil { t.Fatal("expected imbalance") }
}
