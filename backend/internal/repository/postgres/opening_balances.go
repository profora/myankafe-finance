package postgres

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/accounting"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type OpeningBalanceLineInput struct {
	AccountPublicID          string
	FinancialAccountPublicID string
	Direction                string
	Amount                   string
}

type openingPosition struct {
	AccountID     string
	AccountPublic string
	AccountCode   string
	AccountName   string
	AccountType   string
	FinancialID   string
	FinancialPub  string
	Currency      string
	Direction     string
	Amount        string
	Rate          string
	RateID        string
	Functional    string
}

func (s *Store) canEditOpeningBalances(role string) bool {
	return role == "OWNER" || role == "ACCOUNTANT"
}

func (s *Store) SetAccountingStartDate(ctx context.Context, user User, e Entity, date *string) error {
	if e.Role != "OWNER" && e.Role != "ADMIN" && e.Role != "ACCOUNTANT" {
		return fmt.Errorf("accounting start date requires OWNER, ADMIN, or ACCOUNTANT")
	}
	var next *string
	if date != nil && strings.TrimSpace(*date) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*date))
		if err != nil {
			return fmt.Errorf("invalid accounting start date")
		}
		value := parsed.Format("2006-01-02")
		next = &value
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current *string
	if err := tx.QueryRow(ctx, `SELECT accounting_start_date::text FROM entities WHERE id=$1 FOR UPDATE`, e.ID).Scan(&current); err != nil {
		return err
	}
	if sameDate(current, next) {
		return nil
	}
	var frozen bool
	if err := tx.QueryRow(ctx, `
SELECT
  EXISTS(SELECT 1 FROM opening_balance_sets WHERE entity_id=$1)
  OR EXISTS(SELECT 1 FROM journal_entries WHERE entity_id=$1 AND status IN ('POSTED','REVERSED'))`, e.ID).Scan(&frozen); err != nil {
		return err
	}
	if frozen {
		return fmt.Errorf("accounting start date is fixed once opening balances or posted accounting exist")
	}
	if _, err := tx.Exec(ctx, `UPDATE entities SET accounting_start_date=$2, updated_at=now() WHERE id=$1`, e.ID, next); err != nil {
		return err
	}
	after := map[string]any{"accounting_start_date": nil}
	if next != nil {
		after["accounting_start_date"] = *next
	}
	if err := insertAuditTx(ctx, tx, user, e, "ENTITY_ACCOUNTING_START_DATE_SET", "ENTITY", e.PublicID, after); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetOpeningBalances(ctx context.Context, e Entity) (map[string]any, error) {
	var start *string
	if err := s.Pool.QueryRow(ctx, `SELECT accounting_start_date::text FROM entities WHERE id=$1`, e.ID).Scan(&start); err != nil {
		return nil, err
	}
	var locked *string
	_ = s.Pool.QueryRow(ctx, `SELECT transactions_locked_through_date::text FROM entity_accounting_controls WHERE entity_id=$1`, e.ID).Scan(&locked)
	editable := openingEditable(start, locked)
	var initial bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM transactions WHERE entity_id=$1 AND transaction_type='OPENING_BALANCE' AND status='POSTED')`, e.ID).Scan(&initial); err != nil {
		return nil, err
	}
	lines, err := s.loadOpeningLines(ctx, s.Pool, e.ID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.openingAccounts(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	financial, err := s.openingFinancialAccounts(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	equity, err := s.systemBalance(ctx, e.ID, SystemRoleOpeningBalanceEquity)
	if err != nil {
		return nil, err
	}
	adjustment, err := s.systemBalance(ctx, e.ID, SystemRoleOpeningBalanceAdjustment)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"accounting_start_date":      start,
		"editable":                   editable && s.canEditOpeningBalances(e.Role),
		"view_only":                  !s.canEditOpeningBalances(e.Role),
		"locked_through":             locked,
		"lock_message":               openingLockMessage(start, locked),
		"lines":                      lines,
		"accounts":                   accounts,
		"financial_accounts":         financial,
		"opening_balance_equity":     equity,
		"opening_balance_adjustment": adjustment,
		"initial_opening_entry":      initial,
		"functional_currency":        e.FunctionalCurrency,
	}
	return out, nil
}

func (s *Store) PreviewOpeningBalances(ctx context.Context, e Entity, inputs []OpeningBalanceLineInput) (map[string]any, error) {
	if !s.canEditOpeningBalances(e.Role) {
		return nil, fmt.Errorf("opening balances require OWNER or ACCOUNTANT")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	preview, _, err := s.planOpeningBalances(ctx, tx, e, inputs)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

func (s *Store) SaveOpeningBalances(ctx context.Context, user User, e Entity, inputs []OpeningBalanceLineInput) (map[string]any, error) {
	if !s.canEditOpeningBalances(e.Role) {
		return nil, fmt.Errorf("opening balances require OWNER or ACCOUNTANT")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var start string
	if err := tx.QueryRow(ctx, `SELECT accounting_start_date::text FROM entities WHERE id=$1 FOR UPDATE`, e.ID).Scan(&start); err != nil {
		return nil, fmt.Errorf("Set the accounting start date before entering accounting transactions.")
	}
	if start == "" {
		return nil, fmt.Errorf("Set the accounting start date before entering accounting transactions.")
	}
	var locked *string
	_ = tx.QueryRow(ctx, `SELECT transactions_locked_through_date::text FROM entity_accounting_controls WHERE entity_id=$1`, e.ID).Scan(&locked)
	startValue := start
	if message := openingLockMessage(&startValue, locked); message != "" {
		return nil, fmt.Errorf("%s", message)
	}
	if err := s.EnsureOpenDateTx(ctx, tx, e.ID, start); err != nil {
		return nil, err
	}

	preview, positions, err := s.planOpeningBalances(ctx, tx, e, inputs)
	if err != nil {
		return nil, err
	}
	if preview["unchanged"] == true {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		current, err := s.GetOpeningBalances(ctx, e)
		if err != nil {
			return nil, err
		}
		current["unchanged"] = true
		return current, nil
	}

	specs, _ := preview["journal_lines"].([]map[string]any)
	if len(specs) == 0 {
		return nil, fmt.Errorf("opening balance journal has no lines")
	}
	kind := "OPENING_BALANCE"
	action := "OPENING_BALANCE_CREATE"
	description := "Opening balances"
	if preview["initial"] != true {
		kind = "OPENING_BALANCE_ADJUSTMENT"
		action = "OPENING_BALANCE_ADJUSTMENT"
		description = "Opening balance adjustment"
	}
	txPub, err := s.postOpeningJournal(ctx, tx, user, e, start, kind, description, specs)
	if err != nil {
		return nil, err
	}
	if err := s.replaceOpeningLines(ctx, tx, user, e, start, positions); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, user, e, action, "OPENING_BALANCE", txPub, map[string]any{
		"transaction_id": txPub,
		"date":           start,
		"line_count":     len(positions),
		"plug":           preview["plug"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	current, err := s.GetOpeningBalances(ctx, e)
	if err != nil {
		return nil, err
	}
	current["transaction_id"] = txPub
	current["unchanged"] = false
	return current, nil
}

func (s *Store) planOpeningBalances(ctx context.Context, tx pgx.Tx, e Entity, inputs []OpeningBalanceLineInput) (map[string]any, []openingPosition, error) {
	var start *string
	if err := tx.QueryRow(ctx, `SELECT accounting_start_date::text FROM entities WHERE id=$1`, e.ID).Scan(&start); err != nil || start == nil || *start == "" {
		return nil, nil, fmt.Errorf("Set the accounting start date before entering accounting transactions.")
	}
	var locked *string
	_ = tx.QueryRow(ctx, `SELECT transactions_locked_through_date::text FROM entity_accounting_controls WHERE entity_id=$1`, e.ID).Scan(&locked)
	if !openingEditable(start, locked) {
		if message := openingLockMessage(start, locked); message != "" {
			return nil, nil, fmt.Errorf("%s", message)
		}
		return nil, nil, fmt.Errorf("Set the accounting start date before entering accounting transactions.")
	}
	desired, err := s.resolveOpeningInputs(ctx, tx, e, *start, inputs)
	if err != nil {
		return nil, nil, err
	}
	if len(desired) == 0 {
		return nil, nil, fmt.Errorf("enter at least one opening balance")
	}
	var setID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM opening_balance_sets WHERE entity_id=$1 FOR UPDATE`, e.ID).Scan(&setID)
	initial := err != nil
	if err != nil && err != pgx.ErrNoRows {
		return nil, nil, err
	}
	current := map[string]openingPosition{}
	if !initial {
		rows, err := s.loadOpeningPositions(ctx, tx, e.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			current[openingKey(row)] = row
		}
	}
	if !initial && sameOpeningPositions(current, desired) {
		return map[string]any{
			"unchanged":  true,
			"initial":    false,
			"difference": "0.000000",
		}, nil, nil
	}

	var specs []map[string]any
	if initial {
		for _, row := range desired {
			specs = append(specs, journalSpec(row, row.Direction, row.Amount, row.Functional, "Opening balance"))
		}
	} else {
		seen := map[string]bool{}
		for key, row := range desired {
			seen[key] = true
			old, ok := current[key]
			specs = append(specs, deltaSpecs(old, row, ok)...)
		}
		for key, old := range current {
			if !seen[key] {
				specs = append(specs, deltaSpecs(old, openingPosition{}, false)...)
			}
		}
	}
	if len(specs) == 0 {
		return map[string]any{"unchanged": true, "initial": initial, "difference": "0.000000"}, desiredList(desired), nil
	}

	debits, credits := new(big.Rat), new(big.Rat)
	for _, spec := range specs {
		amount, _ := accounting.ParseAmount(spec["functional"].(string))
		if spec["direction"] == "DEBIT" {
			debits.Add(debits, amount)
		} else {
			credits.Add(credits, amount)
		}
	}
	userDebits, userCredits := new(big.Rat), new(big.Rat)
	fxDetails := []map[string]any{}
	for _, row := range desired {
		amount, _ := accounting.ParseAmount(row.Functional)
		if row.Direction == "DEBIT" {
			userDebits.Add(userDebits, amount)
		} else {
			userCredits.Add(userCredits, amount)
		}
		if row.Currency != e.FunctionalCurrency {
			fxDetails = append(fxDetails, map[string]any{
				"currency": row.Currency, "amount": row.Amount, "direction": row.Direction,
				"rate": row.Rate, "functional_amount": row.Functional,
			})
		}
	}
	plugRole := SystemRoleOpeningBalanceEquity
	plugName := "Opening Balance Equity"
	if !initial {
		plugRole = SystemRoleOpeningBalanceAdjustment
		plugName = "Opening Balance Adjustment"
	}
	plug := map[string]any{"account": plugName, "system_role": plugRole, "direction": "", "amount": "0.000000"}
	if debits.Cmp(credits) != 0 {
		diff := new(big.Rat).Sub(debits, credits)
		direction := "CREDIT"
		amount := diff
		if diff.Sign() < 0 {
			direction = "DEBIT"
			amount = new(big.Rat).Neg(diff)
		}
		var plugAccount string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM accounts WHERE entity_id=$1 AND system_role=$2 AND active=true AND is_postable=true`, e.ID, plugRole).Scan(&plugAccount); err != nil {
			return nil, nil, fmt.Errorf("system opening balance account is missing")
		}
		specs = append(specs, map[string]any{
			"account_id": plugAccount, "financial_account_id": "", "currency": e.FunctionalCurrency,
			"direction": direction, "amount": amount.FloatString(6), "rate": "1", "rate_id": "",
			"functional": amount.FloatString(6), "description": plugName,
		})
		if direction == "DEBIT" {
			debits.Add(debits, amount)
		} else {
			credits.Add(credits, amount)
		}
		plug = map[string]any{"account": plugName, "system_role": plugRole, "direction": direction, "amount": amount.FloatString(6)}
	}
	if debits.Cmp(credits) != 0 {
		return nil, nil, fmt.Errorf("opening balance journal does not balance")
	}
	preview := map[string]any{
		"unchanged":             false,
		"initial":               initial,
		"accounting_start_date": *start,
		"user_debits":           userDebits.FloatString(6),
		"user_credits":          userCredits.FloatString(6),
		"fx_details":            fxDetails,
		"plug":                  plug,
		"resulting_debits":      debits.FloatString(6),
		"resulting_credits":     credits.FloatString(6),
		"difference":            "0.000000",
		"functional_currency":   e.FunctionalCurrency,
		"journal_lines":         specs,
	}
	return preview, desiredList(desired), nil
}

func deltaSpecs(old openingPosition, next openingPosition, hasNext bool) []map[string]any {
	if hasNext && samePosition(old, next) {
		return nil
	}
	if hasNext && old.AccountID != "" && old.Currency == next.Currency && old.RateID == next.RateID && ratesEqual(old.Rate, next.Rate) {
		oldSigned := signedAmount(old.Direction, old.Amount)
		newSigned := signedAmount(next.Direction, next.Amount)
		delta := new(big.Rat).Sub(newSigned, oldSigned)
		if delta.Sign() == 0 {
			return nil
		}
		direction := "DEBIT"
		amount := delta
		if delta.Sign() < 0 {
			direction = "CREDIT"
			amount = new(big.Rat).Neg(delta)
		}
		functional := roundedFunctional(amount.FloatString(6), next.Rate)
		row := next
		return []map[string]any{journalSpec(row, direction, amount.FloatString(6), functional, "Opening balance adjustment")}
	}
	var out []map[string]any
	if old.AccountID != "" {
		opposite := "CREDIT"
		if old.Direction == "CREDIT" {
			opposite = "DEBIT"
		}
		out = append(out, journalSpec(old, opposite, old.Amount, old.Functional, "Opening balance adjustment"))
	}
	if hasNext {
		out = append(out, journalSpec(next, next.Direction, next.Amount, next.Functional, "Opening balance adjustment"))
	}
	return out
}

func journalSpec(row openingPosition, direction, amount, functional, description string) map[string]any {
	return map[string]any{
		"account_id": row.AccountID, "financial_account_id": row.FinancialID, "currency": row.Currency,
		"direction": direction, "amount": amount, "rate": row.Rate, "rate_id": row.RateID,
		"functional": functional, "description": description,
	}
}

func (s *Store) resolveOpeningInputs(ctx context.Context, tx pgx.Tx, e Entity, start string, inputs []OpeningBalanceLineInput) (map[string]openingPosition, error) {
	out := map[string]openingPosition{}
	for _, in := range inputs {
		amountText := strings.TrimSpace(in.Amount)
		if amountText == "" || amountText == "0" || amountText == "0.0" || amountText == "0.00" {
			continue
		}
		amount, err := accounting.ParseAmount(amountText)
		if err != nil || amount.Sign() == 0 {
			return nil, fmt.Errorf("opening balance amount is invalid")
		}
		direction := strings.ToUpper(strings.TrimSpace(in.Direction))
		if direction != "DEBIT" && direction != "CREDIT" {
			return nil, fmt.Errorf("opening balance direction must be DEBIT or CREDIT")
		}
		var accountID, code, name, accountType, systemRole string
		var postable bool
		if err := tx.QueryRow(ctx, `
SELECT id::text,code,name,account_type,is_postable,COALESCE(system_role,'')
FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, strings.TrimSpace(in.AccountPublicID)).
			Scan(&accountID, &code, &name, &accountType, &postable, &systemRole); err != nil {
			return nil, fmt.Errorf("opening balance account was not found")
		}
		if !postable || (accountType != "ASSET" && accountType != "LIABILITY" && accountType != "EQUITY") {
			return nil, fmt.Errorf("opening balances only use asset, liability, and equity accounts")
		}
		if systemRole == SystemRoleOpeningBalanceEquity || systemRole == SystemRoleOpeningBalanceAdjustment {
			return nil, fmt.Errorf("opening balance system accounts are not editable")
		}
		row := openingPosition{
			AccountID: accountID, AccountPublic: strings.TrimSpace(in.AccountPublicID), AccountCode: code, AccountName: name, AccountType: accountType,
			Currency: e.FunctionalCurrency, Direction: direction, Amount: amount.FloatString(6), Rate: "1",
		}
		if faPub := strings.TrimSpace(in.FinancialAccountPublicID); faPub != "" {
			var faID, faAccount, currency string
			if err := tx.QueryRow(ctx, `
SELECT id::text,account_id::text,currency_code
FROM financial_accounts
WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, faPub).Scan(&faID, &faAccount, &currency); err != nil {
				return nil, fmt.Errorf("financial account was not found")
			}
			if faAccount != accountID {
				return nil, fmt.Errorf("financial account does not belong to the selected chart account")
			}
			row.FinancialID = faID
			row.FinancialPub = faPub
			row.Currency = currency
		}
		if row.Currency != e.FunctionalCurrency {
			var rate, rateID string
			if err := tx.QueryRow(ctx, `
SELECT rate::text,id::text
FROM exchange_rates
WHERE entity_id=$1 AND from_currency_code=$2 AND to_currency_code=$3 AND rate_date<=$4
ORDER BY rate_date DESC, created_at DESC
LIMIT 1`, e.ID, row.Currency, e.FunctionalCurrency, start).Scan(&rate, &rateID); err != nil {
				return nil, fmt.Errorf("exchange rate missing for %s on or before the accounting start date", row.Currency)
			}
			row.Rate = rate
			row.RateID = rateID
		}
		row.Functional = roundedFunctional(row.Amount, row.Rate)
		key := openingKey(row)
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate opening balance line")
		}
		out[key] = row
	}
	return out, nil
}

func (s *Store) postOpeningJournal(ctx context.Context, tx pgx.Tx, user User, e Entity, date, kind, description string, specs []map[string]any) (string, error) {
	txID, _ := ids.UUIDv7()
	txPub, _ := ids.ULID()
	journalID, _ := ids.UUIDv7()
	journalPub, _ := ids.ULID()
	total := new(big.Rat)
	for _, spec := range specs {
		if spec["direction"] == "DEBIT" {
			amount, _ := accounting.ParseAmount(spec["functional"].(string))
			total.Add(total, amount)
		}
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions(
  id,public_id,entity_id,transaction_type,status,transaction_date,description,currency_code,total_amount,source_type,created_by
) VALUES($1,$2,$3,$4,'DRAFT',$5,$6,$7,$8,'SYSTEM',$9)`,
		txID, txPub, e.ID, kind, date, description, e.FunctionalCurrency, total.FloatString(6), user.ID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO journal_entries(
  id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by
) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8)`,
		journalID, journalPub, e.ID, txID, date, description, e.FunctionalCurrency, user.ID); err != nil {
		return "", err
	}
	for i, spec := range specs {
		lineID, _ := ids.UUIDv7()
		td, tc, fd, fc := "0", "0", "0", "0"
		if spec["direction"] == "DEBIT" {
			td = spec["amount"].(string)
			fd = spec["functional"].(string)
		} else {
			tc = spec["amount"].(string)
			fc = spec["functional"].(string)
		}
		var fa any
		if id := spec["financial_account_id"].(string); id != "" {
			fa = id
		}
		var rateID any
		if id := spec["rate_id"].(string); id != "" {
			rateID = id
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO journal_lines(
  id,journal_entry_id,entity_id,line_no,account_id,financial_account_id,description,
  transaction_currency_code,transaction_debit_amount,transaction_credit_amount,
  functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			lineID, journalID, e.ID, i+1, spec["account_id"], fa, spec["description"],
			spec["currency"], td, tc, e.FunctionalCurrency, spec["rate"], fd, fc, rateID); err != nil {
			return "", err
		}
	}
	var debits, credits string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(debit_amount),0)::text,COALESCE(sum(credit_amount),0)::text FROM journal_lines WHERE journal_entry_id=$1`, journalID).Scan(&debits, &credits); err != nil {
		return "", err
	}
	debitTotal, _ := accounting.ParseAmount(debits)
	creditTotal, _ := accounting.ParseAmount(credits)
	if debitTotal.Cmp(creditTotal) != 0 {
		return "", fmt.Errorf("opening balance journal is unbalanced")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED', posted_by=$2, posted_at=$3 WHERE id=$1`, journalID, user.ID, now); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED', posted_by=$2, posted_at=$3, updated_at=$3 WHERE id=$1`, txID, user.ID, now); err != nil {
		return "", err
	}
	return txPub, nil
}

func (s *Store) replaceOpeningLines(ctx context.Context, tx pgx.Tx, user User, e Entity, start string, lines []openingPosition) error {
	var setID, setPub string
	err := tx.QueryRow(ctx, `SELECT id::text, public_id::text FROM opening_balance_sets WHERE entity_id=$1`, e.ID).Scan(&setID, &setPub)
	if err == pgx.ErrNoRows {
		setID, _ = ids.UUIDv7()
		setPub, _ = ids.ULID()
		if _, err := tx.Exec(ctx, `
INSERT INTO opening_balance_sets(id,public_id,entity_id,accounting_start_date,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$5)`, setID, setPub, e.ID, start, user.ID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if _, err := tx.Exec(ctx, `UPDATE opening_balance_sets SET updated_by=$2, updated_at=now() WHERE id=$1`, setID, user.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM opening_balance_lines WHERE opening_balance_set_id=$1`, setID); err != nil {
		return err
	}
	for _, line := range lines {
		id, _ := ids.UUIDv7()
		var fa any
		if line.FinancialID != "" {
			fa = line.FinancialID
		}
		var rateID any
		if line.RateID != "" {
			rateID = line.RateID
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO opening_balance_lines(
  id,opening_balance_set_id,entity_id,account_id,financial_account_id,currency_code,direction,amount,fx_rate_to_functional,exchange_rate_id
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			id, setID, e.ID, line.AccountID, fa, line.Currency, line.Direction, line.Amount, line.Rate, rateID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadOpeningLines(ctx context.Context, q queryRower, entityID string) ([]map[string]any, error) {
	rows, err := s.loadOpeningPositions(ctx, q, entityID)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, row := range rows {
		out = append(out, map[string]any{
			"account_public_id": row.AccountPublic, "account_code": row.AccountCode, "account_name": row.AccountName,
			"account_type": row.AccountType, "financial_account_id": row.FinancialPub,
			"currency": row.Currency, "direction": row.Direction, "amount": row.Amount,
			"fx_rate_to_functional": row.Rate, "functional_amount": row.Functional,
		})
	}
	return out, nil
}

type queryRower interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (s *Store) loadOpeningPositions(ctx context.Context, q queryRower, entityID string) ([]openingPosition, error) {
	rows, err := q.Query(ctx, `
SELECT a.id::text,a.public_id::text,a.code,a.name,a.account_type,
       COALESCE(fa.id::text,''),COALESCE(fa.public_id::text,''),
       l.currency_code,l.direction,l.amount::text,l.fx_rate_to_functional::text,COALESCE(l.exchange_rate_id::text,'')
FROM opening_balance_lines l
JOIN opening_balance_sets s ON s.id=l.opening_balance_set_id
JOIN accounts a ON a.id=l.account_id
LEFT JOIN financial_accounts fa ON fa.id=l.financial_account_id
WHERE s.entity_id=$1
ORDER BY a.code, fa.name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []openingPosition{}
	for rows.Next() {
		var row openingPosition
		if err := rows.Scan(&row.AccountID, &row.AccountPublic, &row.AccountCode, &row.AccountName, &row.AccountType, &row.FinancialID, &row.FinancialPub, &row.Currency, &row.Direction, &row.Amount, &row.Rate, &row.RateID); err != nil {
			return nil, err
		}
		if parsed, err := accounting.ParseAmount(row.Amount); err == nil {
			row.Amount = parsed.FloatString(6)
		}
		row.Functional = roundedFunctional(row.Amount, row.Rate)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) openingAccounts(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT public_id::text,code,name,account_type
FROM accounts
WHERE entity_id=$1 AND active=true AND is_postable=true
  AND account_type IN ('ASSET','LIABILITY','EQUITY')
  AND COALESCE(system_role,'') NOT IN ('OPENING_BALANCE_EQUITY','OPENING_BALANCE_ADJUSTMENT')
  AND NOT EXISTS (SELECT 1 FROM financial_accounts fa WHERE fa.account_id=accounts.id AND fa.entity_id=accounts.entity_id AND fa.active=true)
ORDER BY code`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, code, name, typ string
		if err := rows.Scan(&id, &code, &name, &typ); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "code": code, "name": name, "type": typ})
	}
	return out, rows.Err()
}

func (s *Store) openingFinancialAccounts(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT fa.public_id::text,fa.name,fa.currency_code,a.public_id::text,a.code,a.name,a.account_type
FROM financial_accounts fa
JOIN accounts a ON a.id=fa.account_id
WHERE fa.entity_id=$1 AND fa.active=true AND a.active=true
  AND a.account_type IN ('ASSET','LIABILITY','EQUITY')
ORDER BY a.code, fa.name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, currency, accountID, code, accountName, typ string
		if err := rows.Scan(&id, &name, &currency, &accountID, &code, &accountName, &typ); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "name": name, "currency": currency, "account_id": accountID,
			"account_code": code, "account_name": accountName, "account_type": typ,
		})
	}
	return out, rows.Err()
}

func (s *Store) systemBalance(ctx context.Context, entityID, role string) (map[string]any, error) {
	var debit, credit string
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(SUM(jl.debit_amount),0)::text, COALESCE(SUM(jl.credit_amount),0)::text
FROM journal_lines jl
JOIN accounts a ON a.id=jl.account_id
JOIN journal_entries je ON je.id=jl.journal_entry_id
WHERE a.entity_id=$1 AND a.system_role=$2 AND je.status IN ('POSTED','REVERSED')`, entityID, role).Scan(&debit, &credit)
	if err != nil {
		return nil, err
	}
	d, _ := accounting.ParseAmount(debit)
	c, _ := accounting.ParseAmount(credit)
	net := new(big.Rat).Sub(c, d)
	direction := "CREDIT"
	amount := net
	if net.Sign() < 0 {
		direction = "DEBIT"
		amount = new(big.Rat).Neg(net)
	}
	if net.Sign() == 0 {
		direction = ""
	}
	return map[string]any{"direction": direction, "amount": amount.FloatString(6), "debit": debit, "credit": credit}, nil
}

func desiredList(rows map[string]openingPosition) []openingPosition {
	out := make([]openingPosition, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

func openingKey(row openingPosition) string {
	return row.AccountID + "|" + row.FinancialID
}

func sameOpeningPositions(current map[string]openingPosition, desired map[string]openingPosition) bool {
	if len(current) != len(desired) {
		return false
	}
	for key, row := range desired {
		old, ok := current[key]
		if !ok || !samePosition(old, row) {
			return false
		}
	}
	return true
}

func samePosition(a, b openingPosition) bool {
	return a.AccountID == b.AccountID && a.FinancialID == b.FinancialID && a.Currency == b.Currency && a.Direction == b.Direction && a.Amount == b.Amount
}

func signedAmount(direction, amount string) *big.Rat {
	value, _ := accounting.ParseAmount(amount)
	if direction == "CREDIT" {
		return new(big.Rat).Neg(value)
	}
	return value
}

func ratesEqual(a, b string) bool {
	ar, aok := new(big.Rat).SetString(a)
	br, bok := new(big.Rat).SetString(b)
	if !aok || !bok {
		return a == b
	}
	return ar.Cmp(br) == 0
}

func roundedFunctional(amount, rate string) string {
	a, _ := accounting.ParseAmount(amount)
	r, _ := new(big.Rat).SetString(rate)
	if r == nil {
		r = new(big.Rat).SetInt64(1)
	}
	return new(big.Rat).Mul(a, r).FloatString(6)
}

func sameDate(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	return b != nil && *a == *b
}

func openingEditable(start, locked *string) bool {
	if start == nil || *start == "" {
		return false
	}
	if locked == nil || *locked == "" {
		return true
	}
	return *locked < *start
}

func openingLockMessage(start, locked *string) string {
	if start == nil || *start == "" || locked == nil || *locked == "" || *locked < *start {
		return ""
	}
	parsed, err := time.Parse("2006-01-02", *start)
	if err != nil {
		return "Opening balances are locked because the accounting start date is within the locked accounting period."
	}
	return "Opening balances are locked because " + parsed.Format("Jan 2, 2006") + " is within the locked accounting period."
}
