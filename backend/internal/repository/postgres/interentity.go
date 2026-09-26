package postgres

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/accounting"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func (s *Store) ListInterEntityMappings(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT ce.public_id::text,ce.name,dfa.public_id::text,dfa.code,dfa.name,dta.public_id::text,dta.code,dta.name
FROM inter_entity_account_mappings m
JOIN entities ce ON ce.id=m.counterparty_entity_id
JOIN accounts dfa ON dfa.id=m.due_from_account_id
JOIN accounts dta ON dta.id=m.due_to_account_id
WHERE m.entity_id=$1 ORDER BY ce.name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var cp, cpName, df, dfCode, dfName, dt, dtCode, dtName string
		if err := rows.Scan(&cp, &cpName, &df, &dfCode, &dfName, &dt, &dtCode, &dtName); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"counterparty_entity_id": cp, "counterparty_name": cpName, "due_from_account": map[string]any{"id": df, "code": dfCode, "name": dfName}, "due_to_account": map[string]any{"id": dt, "code": dtCode, "name": dtName}})
	}
	return out, rows.Err()
}

func (s *Store) UpsertInterEntityMapping(ctx context.Context, user User, e, counterparty Entity, dueFromPub, dueToPub string) (map[string]any, error) {
	if e.ID == counterparty.ID {
		return nil, fmt.Errorf("counterparty must differ")
	}
	var dueFrom, dueTo, dfType, dtType string
	var dfPost, dtPost bool
	if err := s.Pool.QueryRow(ctx, `SELECT id::text,account_type,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, dueFromPub).Scan(&dueFrom, &dfType, &dfPost); err != nil {
		return nil, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT id::text,account_type,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, e.ID, dueToPub).Scan(&dueTo, &dtType, &dtPost); err != nil {
		return nil, err
	}
	if !dfPost || dfType != "ASSET" {
		return nil, fmt.Errorf("due-from must be a postable ASSET account")
	}
	if !dtPost || dtType != "LIABILITY" {
		return nil, fmt.Errorf("due-to must be a postable LIABILITY account")
	}
	id, _ := ids.UUIDv7()
	_, err := s.Pool.Exec(ctx, `INSERT INTO inter_entity_account_mappings(id,entity_id,counterparty_entity_id,due_from_account_id,due_to_account_id,created_by) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(entity_id,counterparty_entity_id) DO UPDATE SET due_from_account_id=EXCLUDED.due_from_account_id,due_to_account_id=EXCLUDED.due_to_account_id,updated_at=now()`, id, e.ID, counterparty.ID, dueFrom, dueTo, user.ID)
	if err != nil {
		return nil, err
	}
	_ = s.Audit(ctx, user, &e, "INTER_ENTITY_MAPPING_UPSERT", "INTER_ENTITY_MAPPING", nil, "SUCCESS", map[string]any{"counterparty_entity_id": counterparty.PublicID, "due_from_account_id": dueFromPub, "due_to_account_id": dueToPub})
	return map[string]any{"counterparty_entity_id": counterparty.PublicID, "due_from_account_id": dueFromPub, "due_to_account_id": dueToPub}, nil
}

type InterEntityPairInput struct {
	PayerDueFromAccountID, PayerDueToAccountID, CounterpartyDueFromAccountID, CounterpartyDueToAccountID string
}

func (s *Store) SaveInterEntityPair(ctx context.Context, user User, payer, counterparty Entity, in InterEntityPairInput) (map[string]any, error) {
	if payer.ID == counterparty.ID {
		return nil, fmt.Errorf("counterparty must differ")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	payerFromID, payerFromName, err := mappedAccount(ctx, tx, payer.ID, in.PayerDueFromAccountID, "ASSET")
	if err != nil {
		return nil, err
	}
	payerToID, payerToName, err := mappedAccount(ctx, tx, payer.ID, in.PayerDueToAccountID, "LIABILITY")
	if err != nil {
		return nil, err
	}
	cpFromID, cpFromName, err := mappedAccount(ctx, tx, counterparty.ID, in.CounterpartyDueFromAccountID, "ASSET")
	if err != nil {
		return nil, err
	}
	cpToID, cpToName, err := mappedAccount(ctx, tx, counterparty.ID, in.CounterpartyDueToAccountID, "LIABILITY")
	if err != nil {
		return nil, err
	}

	beforePayer := mappingPublicIDs(ctx, tx, payer.ID, counterparty.ID)
	beforeCounterparty := mappingPublicIDs(ctx, tx, counterparty.ID, payer.ID)
	if err := upsertMappingTx(ctx, tx, user, payer.ID, counterparty.ID, payerFromID, payerToID); err != nil {
		return nil, err
	}
	if err := upsertMappingTx(ctx, tx, user, counterparty.ID, payer.ID, cpFromID, cpToID); err != nil {
		return nil, err
	}
	after := map[string]any{
		"counterparty_entity_id":           counterparty.PublicID,
		"payer_due_from_account_id":        in.PayerDueFromAccountID,
		"payer_due_to_account_id":          in.PayerDueToAccountID,
		"counterparty_due_from_account_id": in.CounterpartyDueFromAccountID,
		"counterparty_due_to_account_id":   in.CounterpartyDueToAccountID,
		"before":                           map[string]any{"payer": beforePayer, "counterparty": beforeCounterparty},
	}
	if err := insertAuditTx(ctx, tx, user, payer, "INTER_ENTITY_PAIR_SAVE", "INTER_ENTITY_MAPPING", counterparty.PublicID, after); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, user, counterparty, "INTER_ENTITY_PAIR_SAVE", "INTER_ENTITY_MAPPING", payer.PublicID, map[string]any{
		"counterparty_entity_id": payer.PublicID,
		"due_from_account_id":    in.CounterpartyDueFromAccountID,
		"due_to_account_id":      in.CounterpartyDueToAccountID,
		"before":                 beforeCounterparty,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"payer_entity_id": payer.PublicID, "counterparty_entity_id": counterparty.PublicID,
		"payer_due_from_account_id": in.PayerDueFromAccountID, "payer_due_from_name": payerFromName,
		"payer_due_to_account_id": in.PayerDueToAccountID, "payer_due_to_name": payerToName,
		"counterparty_due_from_account_id": in.CounterpartyDueFromAccountID, "counterparty_due_from_name": cpFromName,
		"counterparty_due_to_account_id": in.CounterpartyDueToAccountID, "counterparty_due_to_name": cpToName,
	}, nil
}

func mappedAccount(ctx context.Context, tx pgx.Tx, entityID, publicID, wantType string) (string, string, error) {
	var id, name, accountType string
	var postable, active bool
	err := tx.QueryRow(ctx, `SELECT id::text,name,account_type,is_postable,active FROM accounts WHERE entity_id=$1 AND public_id=$2`, entityID, publicID).Scan(&id, &name, &accountType, &postable, &active)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", fmt.Errorf("%s account was not found on this entity", wantType)
		}
		return "", "", err
	}
	if !active || !postable || accountType != wantType {
		label := "due-from"
		if wantType == "LIABILITY" {
			label = "due-to"
		}
		return "", "", fmt.Errorf("%s must be an active postable %s account on the same entity", label, wantType)
	}
	return id, name, nil
}

func mappingPublicIDs(ctx context.Context, tx pgx.Tx, entityID, counterpartyID string) map[string]string {
	var dueFrom, dueTo string
	err := tx.QueryRow(ctx, `
SELECT dfa.public_id::text,dta.public_id::text
FROM inter_entity_account_mappings m
JOIN accounts dfa ON dfa.id=m.due_from_account_id
JOIN accounts dta ON dta.id=m.due_to_account_id
WHERE m.entity_id=$1 AND m.counterparty_entity_id=$2`, entityID, counterpartyID).Scan(&dueFrom, &dueTo)
	if err != nil {
		return map[string]string{}
	}
	return map[string]string{"due_from_account_id": dueFrom, "due_to_account_id": dueTo}
}

func upsertMappingTx(ctx context.Context, tx pgx.Tx, user User, entityID, counterpartyID, dueFromID, dueToID string) error {
	id, _ := ids.UUIDv7()
	_, err := tx.Exec(ctx, `INSERT INTO inter_entity_account_mappings(id,entity_id,counterparty_entity_id,due_from_account_id,due_to_account_id,created_by) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(entity_id,counterparty_entity_id) DO UPDATE SET due_from_account_id=EXCLUDED.due_from_account_id,due_to_account_id=EXCLUDED.due_to_account_id,updated_at=now()`, id, entityID, counterpartyID, dueFromID, dueToID, user.ID)
	return err
}

func (s *Store) ListInterEntitySetup(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT ce.public_id::text,ce.name,
       dfa.public_id::text,dfa.code,dfa.name,
       dta.public_id::text,dta.code,dta.name,
       rdf.public_id::text,rdf.code,rdf.name,
       rdt.public_id::text,rdt.code,rdt.name
FROM inter_entity_account_mappings m
JOIN entities ce ON ce.id=m.counterparty_entity_id
JOIN accounts dfa ON dfa.id=m.due_from_account_id
JOIN accounts dta ON dta.id=m.due_to_account_id
LEFT JOIN inter_entity_account_mappings rev ON rev.entity_id=m.counterparty_entity_id AND rev.counterparty_entity_id=m.entity_id
LEFT JOIN accounts rdf ON rdf.id=rev.due_from_account_id
LEFT JOIN accounts rdt ON rdt.id=rev.due_to_account_id
WHERE m.entity_id=$1
ORDER BY ce.name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var cp, cpName, df, dfCode, dfName, dt, dtCode, dtName string
		var rdf, rdfCode, rdfName, rdt, rdtCode, rdtName *string
		if err := rows.Scan(&cp, &cpName, &df, &dfCode, &dfName, &dt, &dtCode, &dtName, &rdf, &rdfCode, &rdfName, &rdt, &rdtCode, &rdtName); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"counterparty_entity_id": cp, "counterparty_name": cpName,
			"payer_due_from_account":        map[string]any{"id": df, "code": dfCode, "name": dfName},
			"payer_due_to_account":          map[string]any{"id": dt, "code": dtCode, "name": dtName},
			"counterparty_due_from_account": accountRef(rdf, rdfCode, rdfName),
			"counterparty_due_to_account":   accountRef(rdt, rdtCode, rdtName),
		})
	}
	return out, rows.Err()
}

func accountRef(id, code, name *string) any {
	if id == nil || code == nil || name == nil {
		return nil
	}
	return map[string]any{"id": *id, "code": *code, "name": *name}
}

func (s *Store) InterEntityStatus(ctx context.Context, payer, counterparty Entity) (map[string]any, error) {
	if payer.ID == counterparty.ID {
		return nil, fmt.Errorf("counterparty must differ")
	}
	var dueFromName, dueToName string
	errFrom := s.Pool.QueryRow(ctx, `
SELECT dfa.name
FROM inter_entity_account_mappings m
JOIN accounts dfa ON dfa.id=m.due_from_account_id
WHERE m.entity_id=$1 AND m.counterparty_entity_id=$2`, payer.ID, counterparty.ID).Scan(&dueFromName)
	errTo := s.Pool.QueryRow(ctx, `
SELECT dta.name
FROM inter_entity_account_mappings m
JOIN accounts dta ON dta.id=m.due_to_account_id
WHERE m.entity_id=$1 AND m.counterparty_entity_id=$2`, counterparty.ID, payer.ID).Scan(&dueToName)
	configured := errFrom == nil && errTo == nil
	out := map[string]any{
		"configured":            configured,
		"same_currency":         payer.FunctionalCurrency == counterparty.FunctionalCurrency,
		"payer_currency":        payer.FunctionalCurrency,
		"counterparty_currency": counterparty.FunctionalCurrency,
		"payer_name":            payer.Name,
		"counterparty_name":     counterparty.Name,
	}
	if configured {
		out["payer_due_from_name"] = dueFromName
		out["counterparty_due_to_name"] = dueToName
	}
	if errFrom != nil && errFrom != pgx.ErrNoRows {
		return nil, errFrom
	}
	if errTo != nil && errTo != pgx.ErrNoRows {
		return nil, errTo
	}
	return out, nil
}

type InterEntityExpenseInput struct {
	Date, InitiatingFinancialAccountPublicID, InitiatingAmount, CounterpartyAmount, CounterpartyExpenseAccountPublicID, Description string
}

func (s *Store) PostInterEntityExpense(ctx context.Context, user User, initiating, counterparty Entity, in InterEntityExpenseInput) (map[string]any, error) {
	if initiating.ID == counterparty.ID {
		return nil, fmt.Errorf("entities must differ")
	}
	if initiating.FunctionalCurrency == counterparty.FunctionalCurrency && in.InitiatingAmount != "" && in.CounterpartyAmount != "" {
		iaCheck, iaErr := accounting.ParseAmount(in.InitiatingAmount)
		caCheck, caErr := accounting.ParseAmount(in.CounterpartyAmount)
		if iaErr == nil && caErr == nil && iaCheck.Cmp(caCheck) != 0 {
			return nil, fmt.Errorf("same-currency inter-entity payment must use one amount")
		}
	}
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		return nil, fmt.Errorf("invalid date")
	}
	ia, err := accounting.ParseAmount(in.InitiatingAmount)
	if err != nil || ia.Sign() <= 0 {
		return nil, fmt.Errorf("invalid initiating amount")
	}
	ca, err := accounting.ParseAmount(in.CounterpartyAmount)
	if err != nil || ca.Sign() <= 0 {
		return nil, fmt.Errorf("invalid counterparty amount")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	order := []string{initiating.ID, counterparty.ID}
	sort.Strings(order)
	for _, eid := range order {
		if err := s.EnsureOpenDateTx(ctx, tx, eid, in.Date); err != nil {
			return nil, err
		}
	}

	var faID, faCOA, faCurrency string
	if err := tx.QueryRow(ctx, `SELECT id::text,account_id::text,currency_code FROM financial_accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, initiating.ID, in.InitiatingFinancialAccountPublicID).Scan(&faID, &faCOA, &faCurrency); err != nil {
		return nil, err
	}
	if faCurrency != initiating.FunctionalCurrency {
		return nil, fmt.Errorf("inter-entity V1 requires initiating account in entity functional currency")
	}

	var expenseID, expenseType string
	var postable bool
	if err := tx.QueryRow(ctx, `SELECT id::text,account_type,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`, counterparty.ID, in.CounterpartyExpenseAccountPublicID).Scan(&expenseID, &expenseType, &postable); err != nil {
		return nil, err
	}
	if !postable || expenseType != "EXPENSE" {
		return nil, fmt.Errorf("counterparty account must be postable EXPENSE")
	}

	var initiatingDueFrom, counterpartyDueTo string
	if err := tx.QueryRow(ctx, `SELECT due_from_account_id::text FROM inter_entity_account_mappings WHERE entity_id=$1 AND counterparty_entity_id=$2`, initiating.ID, counterparty.ID).Scan(&initiatingDueFrom); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("inter-entity accounting has not been set up between %s and %s", initiating.Name, counterparty.Name)
		}
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT due_to_account_id::text FROM inter_entity_account_mappings WHERE entity_id=$1 AND counterparty_entity_id=$2`, counterparty.ID, initiating.ID).Scan(&counterpartyDueTo); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("inter-entity accounting has not been set up between %s and %s", initiating.Name, counterparty.Name)
		}
		return nil, err
	}

	initTxID, _ := ids.UUIDv7()
	initTxPub, _ := ids.ULID()
	cpTxID, _ := ids.UUIDv7()
	cpTxPub, _ := ids.ULID()
	interID, _ := ids.UUIDv7()
	interPub, _ := ids.ULID()
	initJID, _ := ids.UUIDv7()
	initJPub, _ := ids.ULID()
	cpJID, _ := ids.UUIDv7()
	cpJPub, _ := ids.ULID()

	if _, err := tx.Exec(ctx, `INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,primary_financial_account_id,currency_code,total_amount,created_by) VALUES($1,$2,$3,'INTER_ENTITY','DRAFT',$4,$5,$6,$7,$8,$9)`, initTxID, initTxPub, initiating.ID, in.Date, in.Description, faID, initiating.FunctionalCurrency, in.InitiatingAmount, user.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,currency_code,total_amount,created_by) VALUES($1,$2,$3,'INTER_ENTITY','DRAFT',$4,$5,$6,$7,$8)`, cpTxID, cpTxPub, counterparty.ID, in.Date, in.Description, counterparty.FunctionalCurrency, in.CounterpartyAmount, user.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO inter_entity_transactions(id,public_id,initiating_entity_id,counterparty_entity_id,purpose,initiating_amount,initiating_currency_code,counterparty_amount,counterparty_currency_code,initiating_transaction_id,counterparty_transaction_id,description,created_by) VALUES($1,$2,$3,$4,'ON_BEHALF_EXPENSE',$5,$6,$7,$8,$9,$10,$11,$12)`, interID, interPub, initiating.ID, counterparty.ID, in.InitiatingAmount, initiating.FunctionalCurrency, in.CounterpartyAmount, counterparty.FunctionalCurrency, initTxID, cpTxID, in.Description, user.ID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO journal_entries(id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8),($9,$10,$11,$12,$5,$6,'DRAFT',$13,$8)`, initJID, initJPub, initiating.ID, initTxID, in.Date, in.Description, initiating.FunctionalCurrency, user.ID, cpJID, cpJPub, counterparty.ID, cpTxID, counterparty.FunctionalCurrency); err != nil {
		return nil, err
	}

	type line struct {
		jid, eid, account, fa, currency, amount string
		debit                                   bool
	}
	lines := []line{
		{initJID, initiating.ID, initiatingDueFrom, "", initiating.FunctionalCurrency, ia.FloatString(6), true},
		{initJID, initiating.ID, faCOA, faID, initiating.FunctionalCurrency, ia.FloatString(6), false},
		{cpJID, counterparty.ID, expenseID, "", counterparty.FunctionalCurrency, ca.FloatString(6), true},
		{cpJID, counterparty.ID, counterpartyDueTo, "", counterparty.FunctionalCurrency, ca.FloatString(6), false},
	}
	for i, l := range lines {
		lid, _ := ids.UUIDv7()
		td, tc, fd, fc := "0", "0", "0", "0"
		if l.debit {
			td = l.amount
			fd = l.amount
		} else {
			tc = l.amount
			fc = l.amount
		}
		var fa any
		if l.fa != "" {
			fa = l.fa
		}
		if _, err := tx.Exec(ctx, `INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,financial_account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$8,1,$11,$12)`, lid, l.jid, l.eid, i%2+1, l.account, fa, in.Description, l.currency, td, tc, fd, fc); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE journal_entries SET status='POSTED',posted_by=$3,posted_at=$4 WHERE id IN ($1,$2)`, initJID, cpJID, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE transactions SET status='POSTED',posted_by=$3,posted_at=$4,updated_at=$4 WHERE id IN ($1,$2)`, initTxID, cpTxID, user.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE inter_entity_transactions SET posted_at=$2 WHERE id=$1`, interID, now); err != nil {
		return nil, err
	}

	if err := insertAuditTx(ctx, tx, user, initiating, "INTER_ENTITY_POST", "TRANSACTION", initTxPub, map[string]any{"inter_entity_id": interPub, "counterparty_transaction_id": cpTxPub}); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, user, counterparty, "INTER_ENTITY_POST", "TRANSACTION", cpTxPub, map[string]any{"inter_entity_id": interPub, "initiating_transaction_id": initTxPub}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return map[string]any{"id": interPub, "initiating_transaction_id": initTxPub, "counterparty_transaction_id": cpTxPub, "initiating_journal_id": initJPub, "counterparty_journal_id": cpJPub, "posted_at": now}, nil
}
