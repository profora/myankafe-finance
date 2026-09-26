package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Currency struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	DecimalPlaces int    `json:"decimal_places"`
	Active        bool   `json:"active"`
}

type CurrencyInput struct {
	Code          string
	Name          string
	Symbol        string
	DecimalPlaces int
	Active        bool
}

func normalizeCurrencyCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return "", fmt.Errorf("currency code must be exactly 3 letters")
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return "", fmt.Errorf("currency code must be exactly 3 letters")
		}
	}
	return code, nil
}

func (s *Store) RequireActiveCurrency(ctx context.Context, code string) error {
	code, err := normalizeCurrencyCode(code)
	if err != nil {
		return err
	}
	var active bool
	err = s.Pool.QueryRow(ctx, `SELECT active FROM currencies WHERE code=$1`, code).Scan(&active)
	if err != nil {
		return fmt.Errorf("unknown currency")
	}
	if !active {
		return fmt.Errorf("currency %s is inactive", code)
	}
	return nil
}

func (s *Store) ListCurrencies(ctx context.Context, activeOnly bool) ([]Currency, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT code, name, symbol, decimal_places, active
FROM currencies
WHERE ($1 = false OR active = true)
ORDER BY code`, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Currency{}
	for rows.Next() {
		var c Currency
		if err := rows.Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces, &c.Active); err != nil {
			return nil, err
		}
		c.Code = strings.TrimSpace(c.Code)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCurrency(ctx context.Context, user User, in CurrencyInput) (Currency, error) {
	code, err := normalizeCurrencyCode(in.Code)
	if err != nil {
		return Currency{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Currency{}, fmt.Errorf("currency name is required")
	}
	if in.DecimalPlaces < 0 || in.DecimalPlaces > 6 {
		return Currency{}, fmt.Errorf("decimal places must be between 0 and 6")
	}
	symbol := strings.TrimSpace(in.Symbol)
	if len(symbol) > 8 {
		return Currency{}, fmt.Errorf("currency symbol is too long")
	}
	c := Currency{Code: code, Name: name, Symbol: symbol, DecimalPlaces: in.DecimalPlaces, Active: in.Active}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO currencies(code, name, symbol, decimal_places, active)
VALUES($1,$2,$3,$4,$5)`, code, name, symbol, in.DecimalPlaces, in.Active)
	if err != nil {
		return Currency{}, err
	}
	if err := s.auditPlatform(ctx, user, "CURRENCY_CREATE", map[string]any{
		"code": code, "name": name, "symbol": symbol, "decimal_places": in.DecimalPlaces, "active": in.Active,
	}); err != nil {
		return Currency{}, err
	}
	return c, nil
}

func (s *Store) UpdateCurrency(ctx context.Context, user User, code string, in CurrencyInput) (Currency, error) {
	code, err := normalizeCurrencyCode(code)
	if err != nil {
		return Currency{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Currency{}, fmt.Errorf("currency name is required")
	}
	if in.DecimalPlaces < 0 || in.DecimalPlaces > 6 {
		return Currency{}, fmt.Errorf("decimal places must be between 0 and 6")
	}
	symbol := strings.TrimSpace(in.Symbol)
	if len(symbol) > 8 {
		return Currency{}, fmt.Errorf("currency symbol is too long")
	}

	var previous bool
	err = s.Pool.QueryRow(ctx, `SELECT active FROM currencies WHERE code=$1`, code).Scan(&previous)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Currency{}, fmt.Errorf("unknown currency")
		}
		return Currency{}, err
	}
	_, err = s.Pool.Exec(ctx, `
UPDATE currencies
SET name=$2, symbol=$3, decimal_places=$4, active=$5
WHERE code=$1`, code, name, symbol, in.DecimalPlaces, in.Active)
	if err != nil {
		return Currency{}, err
	}
	action := "CURRENCY_UPDATE"
	if previous != in.Active {
		if in.Active {
			action = "CURRENCY_ACTIVATE"
		} else {
			action = "CURRENCY_DEACTIVATE"
		}
	}
	if err := s.auditPlatform(ctx, user, action, map[string]any{
		"code": code, "name": name, "symbol": symbol, "decimal_places": in.DecimalPlaces, "active": in.Active,
	}); err != nil {
		return Currency{}, err
	}
	return Currency{Code: code, Name: name, Symbol: symbol, DecimalPlaces: in.DecimalPlaces, Active: in.Active}, nil
}

func (s *Store) DeleteCurrency(ctx context.Context, user User, code string) error {
	code, err := normalizeCurrencyCode(code)
	if err != nil {
		return err
	}
	var referenced bool
	err = s.Pool.QueryRow(ctx, `
SELECT
  EXISTS(SELECT 1 FROM entities WHERE functional_currency_code=$1)
  OR EXISTS(SELECT 1 FROM financial_accounts WHERE currency_code=$1)
  OR EXISTS(SELECT 1 FROM transactions WHERE currency_code=$1)
  OR EXISTS(SELECT 1 FROM journal_entries WHERE functional_currency_code=$1)
  OR EXISTS(SELECT 1 FROM journal_lines WHERE transaction_currency_code=$1 OR functional_currency_code=$1)
  OR EXISTS(SELECT 1 FROM exchange_rates WHERE from_currency_code=$1 OR to_currency_code=$1)
  OR EXISTS(SELECT 1 FROM account_transfer_details WHERE from_currency_code=$1 OR to_currency_code=$1 OR fee_currency_code=$1)
  OR EXISTS(SELECT 1 FROM inter_entity_transactions WHERE initiating_currency_code=$1 OR counterparty_currency_code=$1)`, code).Scan(&referenced)
	if err != nil {
		return err
	}
	if referenced {
		return fmt.Errorf("currency %s is referenced by accounting data; deactivate it instead", code)
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM currencies WHERE code=$1`, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("unknown currency")
	}
	return s.auditPlatform(ctx, user, "CURRENCY_DELETE", map[string]any{"code": code})
}

func (s *Store) auditPlatform(ctx context.Context, user User, action string, after map[string]any) error {
	rows, err := s.Pool.Query(ctx, `SELECT id::text FROM entities ORDER BY code`)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return s.Audit(ctx, user, nil, action, "CURRENCY", nil, "SUCCESS", after)
	}
	for _, id := range ids {
		entity := Entity{ID: id}
		if err := s.Audit(ctx, user, &entity, action, "CURRENCY", nil, "SUCCESS", after); err != nil {
			return err
		}
	}
	return nil
}
