package postgres

import (
	"strings"
	"testing"
)

func TestCrossCurrencyTransferUsesStoredHistoricalRate(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expensePublic, mmkCash := seedServiceEntity(t, ctx, s, "XFX")
	usdCash := insertCashAccount(t, ctx, s, entity, user, "USD")

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: usdCash, ToFinancialAccountPublicID: mmkCash,
		FromAmount: "100", ToAmount: "450000", Description: "Missing historical rate",
	}); err == nil || !strings.Contains(err.Error(), "exchange rate") {
		t.Fatalf("missing rate err=%v", err)
	}
	assertNoTransaction(t, ctx, s, entity.ID, "Missing historical rate")

	created, err := s.CreateExchangeRate(ctx, user, entity, "2026-09-01", "USD", "MMK", "4500", "MANUAL", "historical")
	if err != nil {
		t.Fatal(err)
	}
	rateID := created["id"].(string)

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: usdCash, ToFinancialAccountPublicID: mmkCash,
		FromAmount: "100", ToAmount: "450000", FeeAmount: "1", FeeExpenseAccountPublicID: expensePublic,
		Description: "Cross-currency fee",
	}); err == nil || !strings.Contains(err.Error(), "same currency") {
		t.Fatalf("cross-currency fee err=%v", err)
	}
	assertNoTransaction(t, ctx, s, entity.ID, "Cross-currency fee")

	if _, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: usdCash, ToFinancialAccountPublicID: mmkCash,
		FromAmount: "100", ToAmount: "100", Description: "Unbalanced functional transfer",
	}); err == nil || !strings.Contains(err.Error(), "do not balance") {
		t.Fatalf("unbalanced transfer err=%v", err)
	}
	assertNoTransaction(t, ctx, s, entity.ID, "Unbalanced functional transfer")

	posted, err := s.PostTransfer(ctx, user, entity, TransferInput{
		Date: "2026-09-26", FromFinancialAccountPublicID: usdCash, ToFinancialAccountPublicID: mmkCash,
		FromAmount: "100", ToAmount: "450000", Description: "Historical USD to MMK",
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := s.Pool.Query(ctx, `
SELECT jl.transaction_currency_code, jl.fx_rate_to_functional::text,
       jl.debit_amount::text, jl.credit_amount::text, COALESCE(er.public_id::text,'')
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
LEFT JOIN exchange_rates er ON er.id=jl.exchange_rate_id
WHERE t.public_id=$1
ORDER BY jl.line_no`, posted["id"])
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var sawUSD, sawMMK bool
	for rows.Next() {
		var currency, rate, debit, credit, storedRate string
		if err := rows.Scan(&currency, &rate, &debit, &credit, &storedRate); err != nil {
			t.Fatal(err)
		}
		if currency == "USD" {
			sawUSD = true
			if !strings.HasPrefix(rate, "4500") || storedRate != rateID || credit != "450000.000000" {
				t.Fatalf("USD line rate=%s stored=%s credit=%s", rate, storedRate, credit)
			}
		}
		if currency == "MMK" {
			sawMMK = true
			if !strings.HasPrefix(rate, "1") || storedRate != "" || debit != "450000.000000" {
				t.Fatalf("MMK line rate=%s stored=%s debit=%s", rate, storedRate, debit)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !sawUSD || !sawMMK {
		t.Fatalf("transfer lines usd=%v mmk=%v", sawUSD, sawMMK)
	}
	var balanced bool
	if err := s.Pool.QueryRow(ctx, `
SELECT sum(jl.debit_amount)=sum(jl.credit_amount)
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1`, posted["id"]).Scan(&balanced); err != nil {
		t.Fatal(err)
	}
	if !balanced {
		t.Fatal("cross-currency transfer journal does not balance")
	}

	if _, err := s.CreateExchangeRate(ctx, user, entity, "2026-09-27", "USD", "MMK", "5000", "MANUAL", "later"); err != nil {
		t.Fatal(err)
	}
	var still string
	if err := s.Pool.QueryRow(ctx, `
SELECT jl.fx_rate_to_functional::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id=$1 AND jl.transaction_currency_code='USD'`, posted["id"]).Scan(&still); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(still, "4500") {
		t.Fatalf("historical rate changed to %s", still)
	}
	if _, err := s.UpdateExchangeRate(ctx, user, entity, rateID, UpdateExchangeRateInput{
		RateDate: "2026-09-01", FromCurrency: "USD", ToCurrency: "MMK", Rate: "1", SourceReference: "rewrite",
	}); err == nil {
		t.Fatal("used historical rate was rewritten")
	}
}

func TestDifferentFunctionalCurrencyInterEntity(t *testing.T) {
	ctx, s := integrationStore(t)
	user, payer, _, payerCash := seedServiceEntity(t, ctx, s, "DFX")
	_, counterparty, counterpartyExpense, _ := seedServiceEntity(t, ctx, s, "DFXB")
	if _, err := s.Pool.Exec(ctx, `UPDATE entities SET functional_currency_code='USD' WHERE id=$1`, counterparty.ID); err != nil {
		t.Fatal(err)
	}
	counterparty.FunctionalCurrency = "USD"
	_, payerDueFrom := seedTypedAccountCommitted(t, ctx, s, payer, user, "ASSET", "DUE_FROM")
	_, payerDueTo := seedTypedAccountCommitted(t, ctx, s, payer, user, "LIABILITY", "DUE_TO")
	_, counterpartyDueFrom := seedTypedAccountCommitted(t, ctx, s, counterparty, user, "ASSET", "DUE_FROM")
	_, counterpartyDueTo := seedTypedAccountCommitted(t, ctx, s, counterparty, user, "LIABILITY", "DUE_TO")
	if _, err := s.SaveInterEntityPair(ctx, user, payer, counterparty, InterEntityPairInput{
		PayerDueFromAccountID: payerDueFrom, PayerDueToAccountID: payerDueTo,
		CounterpartyDueFromAccountID: counterpartyDueFrom, CounterpartyDueToAccountID: counterpartyDueTo,
	}); err != nil {
		t.Fatal(err)
	}

	posted, err := s.PostInterEntityExpense(ctx, user, payer, counterparty, InterEntityExpenseInput{
		Date: "2026-09-26", InitiatingFinancialAccountPublicID: payerCash, InitiatingAmount: "450000",
		CounterpartyAmount: "100", CounterpartyExpenseAccountPublicID: counterpartyExpense,
		Description: "MMK payment for USD entity",
	})
	if err != nil {
		t.Fatal(err)
	}

	var initCurrency, cpCurrency, initAmount, cpAmount string
	if err := s.Pool.QueryRow(ctx, `
SELECT initiating_currency_code, counterparty_currency_code, initiating_amount::text, counterparty_amount::text
FROM inter_entity_transactions
WHERE public_id=$1`, posted["id"]).Scan(&initCurrency, &cpCurrency, &initAmount, &cpAmount); err != nil {
		t.Fatal(err)
	}
	if initCurrency != "MMK" || cpCurrency != "USD" || initAmount != "450000.000000" || cpAmount != "100.000000" {
		t.Fatalf("stored inter-entity snapshot %s %s / %s %s", initAmount, initCurrency, cpAmount, cpCurrency)
	}

	rows, err := s.Pool.Query(ctx, `
SELECT t.public_id::text, sum(jl.debit_amount)::text, sum(jl.credit_amount)::text, min(jl.fx_rate_to_functional)::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.public_id IN ($1,$2)
GROUP BY t.public_id`, posted["initiating_transaction_id"], posted["counterparty_transaction_id"])
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id, debit, credit, rate string
		if err := rows.Scan(&id, &debit, &credit, &rate); err != nil {
			t.Fatal(err)
		}
		seen++
		if debit != credit || !strings.HasPrefix(rate, "1") {
			t.Fatalf("journal %s debit=%s credit=%s rate=%s", id, debit, credit, rate)
		}
		if id == posted["initiating_transaction_id"] && debit != "450000.000000" {
			t.Fatalf("payer functional total=%s", debit)
		}
		if id == posted["counterparty_transaction_id"] && debit != "100.000000" {
			t.Fatalf("counterparty functional total=%s", debit)
		}
	}
	if seen != 2 {
		t.Fatalf("journals=%d", seen)
	}

	var rates int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM exchange_rates WHERE entity_id IN ($1,$2)`, payer.ID, counterparty.ID).Scan(&rates); err != nil {
		t.Fatal(err)
	}
	if rates != 0 {
		t.Fatalf("different-currency inter-entity looked up %d exchange rates", rates)
	}
}
