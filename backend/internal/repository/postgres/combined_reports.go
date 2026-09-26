package postgres

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

type CombinedEntitySummary struct {
	EntityID           string `json:"entity_id"`
	EntityName         string `json:"entity_name"`
	FunctionalCurrency string `json:"functional_currency"`
	Income             string `json:"income"`
	Expenses           string `json:"expenses"`
	NetProfit          string `json:"net_profit"`
	ReportingRate      string `json:"reporting_rate"`
}

type CombinedDashboard struct {
	ReportingCurrency string                  `json:"reporting_currency"`
	Income            string                  `json:"income"`
	Expenses          string                  `json:"expenses"`
	NetProfit         string                  `json:"net_profit"`
	Entities          []CombinedEntitySummary `json:"entities"`
}

func ratFromDecimal(v string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(v); !ok {
		return nil, fmt.Errorf("invalid decimal %q", v)
	}
	return r, nil
}

func (s *Store) reportingRate(ctx context.Context, entityID, from, to string, asOf time.Time) (*big.Rat, error) {
	if from == to {
		return new(big.Rat).SetInt64(1), nil
	}
	var rate string
	err := s.Pool.QueryRow(ctx, `
SELECT rate::text FROM exchange_rates
WHERE entity_id=$1 AND from_currency_code=$2 AND to_currency_code=$3 AND rate_date <= $4
ORDER BY rate_date DESC,created_at DESC LIMIT 1`, entityID, from, to, asOf).Scan(&rate)
	if err != nil {
		return nil, fmt.Errorf("reporting rate %s -> %s missing for entity: %w", from, to, err)
	}
	return ratFromDecimal(rate)
}

func (s *Store) CombinedDashboard(ctx context.Context, userID, reportingCurrency string, from, to time.Time) (CombinedDashboard, error) {
	if reportingCurrency == "" {
		reportingCurrency = "MMK"
	}
	entities, err := s.ListEntities(ctx, userID)
	if err != nil {
		return CombinedDashboard{}, err
	}
	totalIncome, totalExpenses, totalProfit := new(big.Rat), new(big.Rat), new(big.Rat)
	out := CombinedDashboard{ReportingCurrency: reportingCurrency, Entities: []CombinedEntitySummary{}}
	for _, e := range entities {
		d, err := s.Dashboard(ctx, e.ID, from, to)
		if err != nil {
			return CombinedDashboard{}, err
		}
		rate, err := s.reportingRate(ctx, e.ID, e.FunctionalCurrency, reportingCurrency, to)
		if err != nil {
			return CombinedDashboard{}, err
		}
		income, _ := ratFromDecimal(d.Income)
		expense, _ := ratFromDecimal(d.Expenses)
		profit, _ := ratFromDecimal(d.NetProfit)
		totalIncome.Add(totalIncome, new(big.Rat).Mul(income, rate))
		totalExpenses.Add(totalExpenses, new(big.Rat).Mul(expense, rate))
		totalProfit.Add(totalProfit, new(big.Rat).Mul(profit, rate))
		out.Entities = append(out.Entities, CombinedEntitySummary{
			EntityID: e.PublicID, EntityName: e.Name, FunctionalCurrency: e.FunctionalCurrency,
			Income: d.Income, Expenses: d.Expenses, NetProfit: d.NetProfit, ReportingRate: rate.FloatString(12),
		})
	}
	out.Income = totalIncome.FloatString(6)
	out.Expenses = totalExpenses.FloatString(6)
	out.NetProfit = totalProfit.FloatString(6)
	return out, nil
}
