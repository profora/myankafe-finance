package accounting

import (
	"fmt"
	"math/big"
)

type Line struct {
	AccountID     string
	Debit, Credit *big.Rat
}

func ParseAmount(v string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(v); !ok {
		return nil, fmt.Errorf("invalid decimal %q", v)
	}
	if r.Sign() < 0 {
		return nil, fmt.Errorf("amount cannot be negative")
	}
	return r, nil
}

func ValidateBalanced(lines []Line) error {
	if len(lines) < 2 {
		return fmt.Errorf("journal requires at least two lines")
	}
	debits, credits := new(big.Rat), new(big.Rat)
	for _, l := range lines {
		if l.Debit == nil || l.Credit == nil {
			return fmt.Errorf("line values are required")
		}
		if l.Debit.Sign() > 0 && l.Credit.Sign() > 0 {
			return fmt.Errorf("line cannot be both debit and credit")
		}
		if l.Debit.Sign() == 0 && l.Credit.Sign() == 0 {
			return fmt.Errorf("line must have a debit or credit")
		}
		debits.Add(debits, l.Debit)
		credits.Add(credits, l.Credit)
	}
	if debits.Cmp(credits) != 0 {
		return fmt.Errorf("journal is not balanced")
	}
	return nil
}
