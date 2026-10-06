package postgres

import (
	"fmt"
	"time"
)

func ValidateFiscalStart(month, day int) error {
	if month < 1 || month > 12 {
		return fmt.Errorf("fiscal month must be 1-12")
	}
	if day < 1 || day > 31 {
		return fmt.Errorf("fiscal day must be 1-31")
	}
	// 2001 is not a leap year, so February 29 is rejected as an ambiguous fiscal start.
	testDate := time.Date(2001, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if int(testDate.Month()) != month || testDate.Day() != day {
		return fmt.Errorf("invalid fiscal year start date")
	}
	return nil
}

func FiscalYearLabel(month, day int) (string, error) {
	if err := ValidateFiscalStart(month, day); err != nil {
		return "", err
	}
	start := time.Date(2001, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, -1)
	return start.Format("Jan 2") + " → " + end.Format("Jan 2"), nil
}
