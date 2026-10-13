package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/integration"
)

const integrationSystem = integration.SystemMyanKafePlatform

type IntegrationConnection struct {
	ID              string `json:"id"`
	PublicID        string `json:"public_id"`
	EntityID        string `json:"entity_id"`
	Name            string `json:"name"`
	SecretReference string `json:"-"`
	Active          bool   `json:"active"`
}

type IntegrationEventView struct {
	PublicID            string          `json:"public_id"`
	ExternalEventID     string          `json:"external_event_id"`
	EventType           string          `json:"event_type"`
	Status              string          `json:"status"`
	AccountingDate      string          `json:"accounting_date"`
	TransactionPublicID string          `json:"transaction_public_id"`
	ErrorCode           string          `json:"error_code"`
	ErrorMessage        string          `json:"error_message"`
	OccurredAt          *time.Time      `json:"occurred_at"`
	Payload             json.RawMessage `json:"payload,omitempty"`
}

type IntegrationResult struct {
	Status              string
	EventPublicID       string
	TransactionPublicID string
	Replayed            bool
	Lines               []integration.PlannedLine
	SalesChannelKey     string
	CustomerSegment     string
}

func (s *Store) ListIntegrationConnections(ctx context.Context, entityID string) ([]IntegrationConnection, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id::text, public_id::text, entity_id::text, name, COALESCE(secret_reference,''), active
FROM integration_connections
WHERE entity_id=$1
ORDER BY system_code`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationConnection
	for rows.Next() {
		var c IntegrationConnection
		if err := rows.Scan(&c.ID, &c.PublicID, &c.EntityID, &c.Name, &c.SecretReference, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateIntegrationConnection(ctx context.Context, user User, e Entity, name string) (IntegrationConnection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "MyanKafe Platform"
	}
	id, err := ids.UUIDv7()
	if err != nil {
		return IntegrationConnection{}, err
	}
	pub, err := ids.ULID()
	if err != nil {
		return IntegrationConnection{}, err
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO integration_connections(id,public_id,entity_id,system_code,name,secret_reference,active)
VALUES($1,$2,$3,$4,$5,$6,false)`, id, pub, e.ID, integrationSystem, name, integration.SecretReference)
	if err != nil {
		return IntegrationConnection{}, err
	}
	_ = s.Audit(ctx, user, &e, "integration.connection.create", "integration_connection", &pub, "SUCCESS", map[string]any{
		"system_code": integrationSystem, "active": false,
	})
	return IntegrationConnection{ID: id, PublicID: pub, EntityID: e.ID, Name: name, SecretReference: integration.SecretReference, Active: false}, nil
}

func (s *Store) UpdateIntegrationConnection(ctx context.Context, user User, e Entity, system, name string, active bool) (IntegrationConnection, error) {
	if system != integrationSystem {
		return IntegrationConnection{}, fmt.Errorf("unknown integration system")
	}
	name = strings.TrimSpace(name)
	var c IntegrationConnection
	err := s.Pool.QueryRow(ctx, `
UPDATE integration_connections
SET name=COALESCE(NULLIF($3,''), name), active=$4
WHERE entity_id=$1 AND system_code=$2
RETURNING id::text, public_id::text, entity_id::text, name, COALESCE(secret_reference,''), active`,
		e.ID, system, name, active).Scan(&c.ID, &c.PublicID, &c.EntityID, &c.Name, &c.SecretReference, &c.Active)
	if err != nil {
		return IntegrationConnection{}, err
	}
	_ = s.Audit(ctx, user, &e, "integration.connection.update", "integration_connection", &c.PublicID, "SUCCESS", map[string]any{
		"system_code": system, "active": active,
	})
	return c, nil
}

func (s *Store) ActiveIntegrationConnection(ctx context.Context, system string) (IntegrationConnection, string, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT c.id::text, c.public_id::text, c.entity_id::text, c.name, COALESCE(c.secret_reference,''), c.active, e.functional_currency_code
FROM integration_connections c
JOIN entities e ON e.id=c.entity_id
WHERE c.system_code=$1`, system)
	if err != nil {
		return IntegrationConnection{}, "", err
	}
	defer rows.Close()
	var found []IntegrationConnection
	var currency string
	var active IntegrationConnection
	var activeCurrency string
	var activeCount int
	for rows.Next() {
		var c IntegrationConnection
		var cur string
		if err := rows.Scan(&c.ID, &c.PublicID, &c.EntityID, &c.Name, &c.SecretReference, &c.Active, &cur); err != nil {
			return IntegrationConnection{}, "", err
		}
		found = append(found, c)
		if c.Active {
			active = c
			activeCurrency = cur
			activeCount++
		}
		currency = cur
	}
	if err := rows.Err(); err != nil {
		return IntegrationConnection{}, "", err
	}
	if len(found) == 0 {
		return IntegrationConnection{}, "", &integration.Error{Status: 401, Code: "unknown_system", Message: "unknown integration system"}
	}
	if activeCount == 0 {
		return found[0], currency, &integration.Error{Status: 403, Code: "inactive_connection", Message: "integration connection is inactive"}
	}
	if activeCount > 1 {
		return IntegrationConnection{}, "", &integration.Error{Status: 409, Code: "ambiguous_connection", Message: "more than one active integration connection"}
	}
	return active, activeCurrency, nil
}

type IntegrationMappingInput struct {
	SourceKey                string `json:"source_key"`
	AccountPublicID          string `json:"account_public_id"`
	FinancialAccountPublicID string `json:"financial_account_public_id"`
	SalesChannelPublicID     string `json:"sales_channel_public_id"`
}

func (s *Store) PutIntegrationMapping(ctx context.Context, user User, e Entity, in IntegrationMappingInput) error {
	key := strings.ToUpper(strings.TrimSpace(in.SourceKey))
	if !integration.ValidSourceKey(key) {
		return integration.Validation("source key is not a supported integration mapping")
	}
	conn, err := s.connectionForEntity(ctx, e.ID)
	if err != nil {
		return err
	}
	target, err := s.resolveMappingTarget(ctx, e.ID, key, in)
	if err != nil {
		return err
	}
	if err := integration.AcceptTarget(key, target); err != nil {
		return err
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	_, err = s.Pool.Exec(ctx, `
INSERT INTO integration_mappings(
  id, public_id, integration_connection_id, mapping_kind, source_key, account_id, financial_account_id, sales_channel_id
) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (integration_connection_id, source_key) DO UPDATE SET
  mapping_kind=EXCLUDED.mapping_kind,
  account_id=EXCLUDED.account_id,
  financial_account_id=EXCLUDED.financial_account_id,
  sales_channel_id=EXCLUDED.sales_channel_id,
  updated_at=now()`,
		id, pub, conn.ID, target.Kind, key, nullUUID(target.AccountID, target.Kind == "ACCOUNT"), nullUUID(target.FinancialAccountID, target.Kind == "FINANCIAL_ACCOUNT"), nullUUID(target.SalesChannelID, target.Kind == "SALES_CHANNEL"))
	if err != nil {
		return err
	}
	_ = s.Audit(ctx, user, &e, "integration.mapping.save", "integration_mapping", &pub, "SUCCESS", map[string]any{"source_key": key})
	return nil
}

func nullUUID(id string, use bool) any {
	if !use || id == "" {
		return nil
	}
	return id
}

func (s *Store) resolveMappingTarget(ctx context.Context, entityID, key string, in IntegrationMappingInput) (integration.Target, error) {
	kind := integration.MappingKind(key)
	switch kind {
	case "ACCOUNT":
		if in.AccountPublicID == "" || in.FinancialAccountPublicID != "" || in.SalesChannelPublicID != "" {
			return integration.Target{}, integration.Validation("account mapping requires only an account")
		}
		var id, typ string
		var postable, active bool
		err := s.Pool.QueryRow(ctx, `
SELECT id::text, account_type, is_postable, active
FROM accounts WHERE entity_id=$1 AND public_id=$2`, entityID, in.AccountPublicID).Scan(&id, &typ, &postable, &active)
		if err != nil {
			return integration.Target{}, integration.Validation("account was not found in this entity")
		}
		return integration.Target{Kind: "ACCOUNT", AccountID: id, AccountType: typ, Postable: postable, Active: active}, nil
	case "FINANCIAL_ACCOUNT":
		if in.FinancialAccountPublicID == "" || in.AccountPublicID != "" || in.SalesChannelPublicID != "" {
			return integration.Target{}, integration.Validation("payment mapping requires only a financial account")
		}
		var id, accountID, currency, typ string
		var active, postable, accountActive bool
		err := s.Pool.QueryRow(ctx, `
SELECT fa.id::text, fa.account_id::text, fa.currency_code, fa.active, a.is_postable, a.active, a.account_type
FROM financial_accounts fa
JOIN accounts a ON a.id=fa.account_id
WHERE fa.entity_id=$1 AND fa.public_id=$2`, entityID, in.FinancialAccountPublicID).Scan(&id, &accountID, &currency, &active, &postable, &accountActive, &typ)
		if err != nil {
			return integration.Target{}, integration.Validation("financial account was not found in this entity")
		}
		return integration.Target{Kind: "FINANCIAL_ACCOUNT", FinancialAccountID: id, AccountID: accountID, Currency: currency, Active: active && accountActive, Postable: postable, AccountType: typ}, nil
	default:
		if in.SalesChannelPublicID == "" || in.AccountPublicID != "" || in.FinancialAccountPublicID != "" {
			return integration.Target{}, integration.Validation("sales channel mapping requires only a sales channel")
		}
		var id string
		var active bool
		err := s.Pool.QueryRow(ctx, `
SELECT id::text, active FROM sales_channels WHERE entity_id=$1 AND public_id=$2`, entityID, in.SalesChannelPublicID).Scan(&id, &active)
		if err != nil {
			return integration.Target{}, integration.Validation("sales channel was not found in this entity")
		}
		return integration.Target{Kind: "SALES_CHANNEL", SalesChannelID: id, Active: active}, nil
	}
}

func (s *Store) connectionForEntity(ctx context.Context, entityID string) (IntegrationConnection, error) {
	var c IntegrationConnection
	err := s.Pool.QueryRow(ctx, `
SELECT id::text, public_id::text, entity_id::text, name, COALESCE(secret_reference,''), active
FROM integration_connections
WHERE entity_id=$1 AND system_code=$2`, entityID, integrationSystem).Scan(&c.ID, &c.PublicID, &c.EntityID, &c.Name, &c.SecretReference, &c.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IntegrationConnection{}, integration.Validation("create the MyanKafe Platform connection before saving mappings")
		}
		return IntegrationConnection{}, err
	}
	return c, nil
}

type IntegrationMappingView struct {
	SourceKey      string `json:"source_key"`
	MappingKind    string `json:"mapping_kind"`
	TargetPublicID string `json:"target_public_id"`
	TargetCode     string `json:"target_code"`
	TargetName     string `json:"target_name"`
}

func (s *Store) ListIntegrationMappings(ctx context.Context, entityID string) ([]IntegrationMappingView, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT m.source_key, m.mapping_kind,
       COALESCE(a.public_id::text, fa.public_id::text, sc.public_id::text, ''),
       COALESCE(a.code, fa.code, sc.code, ''),
       COALESCE(a.name, fa.name, sc.name, '')
FROM integration_mappings m
JOIN integration_connections c ON c.id=m.integration_connection_id
LEFT JOIN accounts a ON a.id=m.account_id
LEFT JOIN financial_accounts fa ON fa.id=m.financial_account_id
LEFT JOIN sales_channels sc ON sc.id=m.sales_channel_id
WHERE c.entity_id=$1 AND c.system_code=$2
ORDER BY m.source_key`, entityID, integrationSystem)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationMappingView
	for rows.Next() {
		var v IntegrationMappingView
		if err := rows.Scan(&v.SourceKey, &v.MappingKind, &v.TargetPublicID, &v.TargetCode, &v.TargetName); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type IntegrationReadiness struct {
	ConnectionActive   bool           `json:"connection_active"`
	SecretConfigured   bool           `json:"secret_configured"`
	EntityPublicID     string         `json:"entity_public_id"`
	EntityName         string         `json:"entity_name"`
	AccountingStart    *string        `json:"accounting_start_date"`
	LockedThrough      *string        `json:"locked_through"`
	FunctionalCurrency string         `json:"functional_currency"`
	MappingCounts      map[string]int `json:"mapping_counts"`
	Missing            []string       `json:"missing_mapping_keys"`
	Ready              bool           `json:"ready"`
}

func (s *Store) IntegrationReadiness(ctx context.Context, entityID string, secretConfigured bool) (IntegrationReadiness, error) {
	out := IntegrationReadiness{
		SecretConfigured: secretConfigured,
		MappingCounts:    map[string]int{"ACCOUNT": 0, "FINANCIAL_ACCOUNT": 0, "SALES_CHANNEL": 0},
		Missing:          []string{},
	}
	var start, locked *string
	err := s.Pool.QueryRow(ctx, `
SELECT e.public_id::text, e.name, e.functional_currency_code, e.accounting_start_date::text,
       (SELECT transactions_locked_through_date::text FROM entity_accounting_controls c WHERE c.entity_id=e.id)
FROM entities e WHERE e.id=$1`, entityID).Scan(&out.EntityPublicID, &out.EntityName, &out.FunctionalCurrency, &start, &locked)
	if err != nil {
		return out, err
	}
	out.AccountingStart = start
	out.LockedThrough = locked
	conn, err := s.connectionForEntity(ctx, entityID)
	if err != nil {
		var ie *integration.Error
		if errors.As(err, &ie) {
			out.Missing = integration.StructuralKeys()
			return out, nil
		}
		return out, err
	}
	out.ConnectionActive = conn.Active
	out.SecretConfigured = secretConfigured && conn.SecretReference != ""
	rows, err := s.Pool.Query(ctx, `SELECT mapping_kind, source_key FROM integration_mappings WHERE integration_connection_id=$1`, conn.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var kind, key string
		if err := rows.Scan(&kind, &key); err != nil {
			return out, err
		}
		out.MappingCounts[kind]++
		have[key] = true
	}
	for _, key := range integration.StructuralKeys() {
		if !have[key] {
			out.Missing = append(out.Missing, key)
		}
	}
	out.Ready = out.ConnectionActive && out.SecretConfigured && start != nil && *start != "" && len(out.Missing) == 0
	return out, rows.Err()
}

func (s *Store) ListIntegrationEvents(ctx context.Context, entityID string, limit int) ([]IntegrationEventView, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
SELECT ev.public_id::text, ev.external_event_id, ev.event_type, ev.status,
       COALESCE(ev.accounting_date::text,''), COALESCE(t.public_id::text,''),
       COALESCE(ev.error_code,''), COALESCE(ev.error_message,''), ev.occurred_at, ev.payload
FROM integration_events ev
JOIN integration_connections c ON c.id=ev.integration_connection_id
LEFT JOIN transactions t ON t.id=ev.transaction_id
WHERE c.entity_id=$1 AND c.system_code=$2
ORDER BY ev.received_at DESC
LIMIT $3`, entityID, integrationSystem, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (s *Store) PreviewIntegrationEvent(ctx context.Context, conn IntegrationConnection, currency string, raw []byte) (IntegrationResult, error) {
	env, err := integration.ParseEnvelope(raw)
	if err != nil {
		return IntegrationResult{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return IntegrationResult{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.EnsureOpenDateTx(ctx, tx, conn.EntityID, env.AccountingDate); err != nil {
		return IntegrationResult{}, accountingError(err)
	}
	if currency != "" && currency != "MMK" {
		return IntegrationResult{}, integration.Validation("entity functional currency must be MMK")
	}
	plan, err := integration.PlanEvent(env, s.mappingResolver(ctx, tx, conn.ID, conn.EntityID))
	if err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{Status: "VALIDATED", Lines: plan.Lines, SalesChannelKey: plan.SalesChannelKey, CustomerSegment: plan.CustomerSegment}, nil
}

func (s *Store) PostIntegrationEvent(ctx context.Context, conn IntegrationConnection, currency string, raw []byte) (IntegrationResult, error) {
	env, err := integration.ParseEnvelope(raw)
	if err != nil {
		return IntegrationResult{}, err
	}
	hash := integration.BodyHash(raw)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return IntegrationResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM integration_connections WHERE id=$1 FOR UPDATE`, conn.ID); err != nil {
		return IntegrationResult{}, err
	}
	eventID, pub, inserted, err := insertIntegrationEvent(ctx, tx, conn.ID, env, hash, raw)
	if err != nil {
		return IntegrationResult{}, err
	}
	if !inserted {
		var status, storedHash, txnID string
		var eventPublic string
		err = tx.QueryRow(ctx, `
SELECT id::text, public_id::text, status, COALESCE(payload_hash,''), COALESCE(transaction_id::text,'')
FROM integration_events
WHERE integration_connection_id=$1 AND external_event_id=$2
FOR UPDATE`, conn.ID, env.ExternalEventID).Scan(&eventID, &eventPublic, &status, &storedHash, &txnID)
		if err != nil {
			return IntegrationResult{}, err
		}
		if storedHash != hash {
			return IntegrationResult{}, integration.Conflict("external event id was already used with a different payload")
		}
		if status == "POSTED" {
			res, err := loadPostedResult(ctx, tx, eventPublic, txnID)
			if err != nil {
				return IntegrationResult{}, err
			}
			res.Replayed = true
			if err := tx.Commit(ctx); err != nil {
				return IntegrationResult{}, err
			}
			return res, nil
		}
		pub = eventPublic
	}
	if currency != "" && currency != "MMK" {
		return failEvent(ctx, tx, eventID, integration.Validation("entity functional currency must be MMK"))
	}
	if err := s.EnsureOpenDateTx(ctx, tx, conn.EntityID, env.AccountingDate); err != nil {
		return failEvent(ctx, tx, eventID, accountingError(err))
	}
	plan, err := integration.PlanEvent(env, s.mappingResolver(ctx, tx, conn.ID, conn.EntityID))
	if err != nil {
		return failEvent(ctx, tx, eventID, err)
	}
	txnPublic, err := s.insertIntegrationJournal(ctx, tx, conn, env, plan)
	if err != nil {
		return IntegrationResult{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE integration_events
SET status='POSTED', transaction_id=(SELECT id FROM transactions WHERE public_id=$2),
    error_code=NULL, error_message=NULL, processed_at=now()
WHERE id=$1`, eventID, txnPublic); err != nil {
		return IntegrationResult{}, err
	}
	if err := writeIntegrationAudit(ctx, tx, conn.EntityID, env, txnPublic); err != nil {
		return IntegrationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{
		Status: "POSTED", EventPublicID: pub, TransactionPublicID: txnPublic,
		Lines: plan.Lines, SalesChannelKey: plan.SalesChannelKey, CustomerSegment: plan.CustomerSegment,
	}, nil
}

func failEvent(ctx context.Context, tx pgx.Tx, eventID string, cause error) (IntegrationResult, error) {
	code, msg := "validation_failed", cause.Error()
	var ie *integration.Error
	if errors.As(cause, &ie) {
		code, msg = ie.Code, ie.Message
	}
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if _, err := tx.Exec(ctx, `
UPDATE integration_events
SET status='FAILED', error_code=$2, error_message=$3, processed_at=now(), transaction_id=NULL
WHERE id=$1 AND status<>'POSTED'`, eventID, code, msg); err != nil {
		return IntegrationResult{}, err
	}
	var pub string
	_ = tx.QueryRow(ctx, `SELECT public_id::text FROM integration_events WHERE id=$1`, eventID).Scan(&pub)
	if err := tx.Commit(ctx); err != nil {
		return IntegrationResult{}, err
	}
	if ie != nil {
		return IntegrationResult{Status: "FAILED", EventPublicID: pub}, ie
	}
	return IntegrationResult{Status: "FAILED", EventPublicID: pub}, &integration.Error{Status: 422, Code: code, Message: msg}
}

func insertIntegrationEvent(ctx context.Context, tx pgx.Tx, connectionID string, env integration.Envelope, hash string, raw []byte) (string, string, bool, error) {
	id, err := ids.UUIDv7()
	if err != nil {
		return "", "", false, err
	}
	pub, err := ids.ULID()
	if err != nil {
		return "", "", false, err
	}
	var occurred any
	if strings.TrimSpace(env.OccurredAt) != "" {
		if t, err := time.Parse(time.RFC3339, env.OccurredAt); err == nil {
			occurred = t
		}
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO integration_events(
  id, public_id, integration_connection_id, external_event_id, event_type, occurred_at,
  payload_hash, status, payload, payload_version, accounting_date
) VALUES($1,$2,$3,$4,$5,$6,$7,'RECEIVED',$8::jsonb,$9,$10)
ON CONFLICT (integration_connection_id, external_event_id) DO NOTHING`,
		id, pub, connectionID, env.ExternalEventID, env.EventType, occurred, hash, string(raw), env.PayloadVersion, env.AccountingDate)
	if err != nil {
		return "", "", false, err
	}
	return id, pub, tag.RowsAffected() == 1, nil
}

func (s *Store) insertIntegrationJournal(ctx context.Context, tx pgx.Tx, conn IntegrationConnection, env integration.Envelope, plan integration.Plan) (string, error) {
	var channelID any
	if plan.SalesChannelKey != "" {
		target, err := s.mappingResolver(ctx, tx, conn.ID, conn.EntityID)(plan.SalesChannelKey)
		if err != nil {
			return "", err
		}
		channelID = target.SalesChannelID
	}
	var segment any
	if plan.CustomerSegment != "" {
		segment = plan.CustomerSegment
	}
	var primaryFA any
	txnID, err := ids.UUIDv7()
	if err != nil {
		return "", err
	}
	txnPublic, err := ids.ULID()
	if err != nil {
		return "", err
	}
	journalID, err := ids.UUIDv7()
	if err != nil {
		return "", err
	}
	journalPublic, err := ids.ULID()
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO transactions(
  id, public_id, entity_id, transaction_type, status, transaction_date, description,
  currency_code, total_amount, source_type, source_system, external_reference,
  sales_channel_id, customer_segment_snapshot, created_by
) VALUES($1,$2,$3,'INTEGRATION','DRAFT',$4,$5,$6,$7,'INTEGRATION',$8,$9,$10,$11,NULL)`,
		txnID, txnPublic, conn.EntityID, plan.Date, plan.Description, plan.Currency, integration.Amount(plan.Total),
		integrationSystem, env.ExternalEventID, channelID, segment); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO journal_entries(
  id, public_id, entity_id, transaction_id, journal_date, description, status, functional_currency_code, created_by
) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,NULL)`,
		journalID, journalPublic, conn.EntityID, txnID, plan.Date, plan.Description, plan.Currency); err != nil {
		return "", err
	}
	for i, line := range plan.Lines {
		target, err := s.mappingResolver(ctx, tx, conn.ID, conn.EntityID)(line.SourceKey)
		if err != nil {
			return "", err
		}
		lineID, err := ids.UUIDv7()
		if err != nil {
			return "", err
		}
		var fa any
		if target.Kind == "FINANCIAL_ACCOUNT" {
			fa = target.FinancialAccountID
			primaryFA = target.FinancialAccountID
		}
		debit, credit := integration.Amount(line.Debit), integration.Amount(line.Credit)
		if _, err := tx.Exec(ctx, `
INSERT INTO journal_lines(
  id, journal_entry_id, entity_id, line_no, account_id, financial_account_id, description,
  transaction_currency_code, transaction_debit_amount, transaction_credit_amount,
  functional_currency_code, fx_rate_to_functional, debit_amount, credit_amount
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$8,1,$9,$10)`,
			lineID, journalID, conn.EntityID, i+1, target.AccountID, fa, line.Memo, plan.Currency, debit, credit); err != nil {
			return "", err
		}
	}
	if primaryFA != nil {
		if _, err := tx.Exec(ctx, `UPDATE transactions SET primary_financial_account_id=$2 WHERE id=$1`, txnID, primaryFA); err != nil {
			return "", err
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE journal_entries SET status='POSTED', posted_at=now() WHERE id=$1`, journalID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
UPDATE transactions SET status='POSTED', posted_at=now(), updated_at=now() WHERE id=$1`, txnID); err != nil {
		return "", err
	}
	return txnPublic, nil
}

func (s *Store) mappingResolver(ctx context.Context, tx pgx.Tx, connectionID, entityID string) integration.Resolver {
	return func(sourceKey string) (integration.Target, error) {
		var kind string
		var accountID, accountType *string
		var postable, accountActive *bool
		var faID, faCurrency *string
		var faActive *bool
		var faAccountID *string
		var faPostable, faAccountActive *bool
		var channelID *string
		var channelActive *bool
		err := tx.QueryRow(ctx, `
SELECT m.mapping_kind,
       a.id::text, a.account_type, a.is_postable, a.active,
       fa.id::text, fa.currency_code, fa.active, fa.account_id::text, faa.is_postable, faa.active,
       sc.id::text, sc.active
FROM integration_mappings m
LEFT JOIN accounts a ON a.id=m.account_id AND a.entity_id=$3
LEFT JOIN financial_accounts fa ON fa.id=m.financial_account_id AND fa.entity_id=$3
LEFT JOIN accounts faa ON faa.id=fa.account_id
LEFT JOIN sales_channels sc ON sc.id=m.sales_channel_id AND sc.entity_id=$3
WHERE m.integration_connection_id=$1 AND m.source_key=$2`, connectionID, sourceKey, entityID).Scan(
			&kind, &accountID, &accountType, &postable, &accountActive, &faID, &faCurrency, &faActive, &faAccountID, &faPostable, &faAccountActive, &channelID, &channelActive,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return integration.Target{}, integration.Mapping(sourceKey)
			}
			return integration.Target{}, err
		}
		switch kind {
		case "ACCOUNT":
			if accountID == nil {
				return integration.Target{}, integration.Mapping(sourceKey)
			}
			return integration.Target{Kind: "ACCOUNT", AccountID: *accountID, AccountType: deref(accountType), Postable: derefBool(postable), Active: derefBool(accountActive)}, nil
		case "FINANCIAL_ACCOUNT":
			if faID == nil || faAccountID == nil {
				return integration.Target{}, integration.Mapping(sourceKey)
			}
			return integration.Target{
				Kind: "FINANCIAL_ACCOUNT", FinancialAccountID: *faID, AccountID: *faAccountID,
				Currency: deref(faCurrency), Active: derefBool(faActive) && derefBool(faAccountActive), Postable: derefBool(faPostable),
			}, nil
		case "SALES_CHANNEL":
			if channelID == nil {
				return integration.Target{}, integration.Mapping(sourceKey)
			}
			return integration.Target{Kind: "SALES_CHANNEL", SalesChannelID: *channelID, Active: derefBool(channelActive)}, nil
		default:
			return integration.Target{}, integration.Mapping(sourceKey)
		}
	}
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefBool(v *bool) bool {
	return v != nil && *v
}

func accountingError(err error) error {
	msg := err.Error()
	code := "accounting_rejected"
	if strings.Contains(msg, "locked") {
		code = "accounting_locked"
	}
	if strings.Contains(msg, "start date") {
		code = "accounting_start_date"
	}
	return &integration.Error{Status: 422, Code: code, Message: msg}
}

func loadPostedResult(ctx context.Context, tx pgx.Tx, eventPublic, txnID string) (IntegrationResult, error) {
	var txnPublic string
	if txnID == "" {
		return IntegrationResult{}, integration.Validation("posted event has no transaction")
	}
	if err := tx.QueryRow(ctx, `SELECT public_id::text FROM transactions WHERE id=$1`, txnID).Scan(&txnPublic); err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{Status: "POSTED", EventPublicID: eventPublic, TransactionPublicID: txnPublic}, nil
}

func writeIntegrationAudit(ctx context.Context, tx pgx.Tx, entityID string, env integration.Envelope, txnPublic string) error {
	id, err := ids.UUIDv7()
	if err != nil {
		return err
	}
	pub, err := ids.ULID()
	if err != nil {
		return err
	}
	after, _ := json.Marshal(map[string]any{
		"event_type": env.EventType, "external_event_id": env.ExternalEventID, "transaction_public_id": txnPublic,
	})
	_, err = tx.Exec(ctx, `
INSERT INTO audit_events(id, public_id, actor_type, entity_id, action, resource_type, resource_public_id, outcome, source, after_data)
VALUES($1,$2,'INTEGRATION',$3,'integration.event.posted','transaction',$4,'SUCCESS','INTEGRATION',$5::jsonb)`,
		id, pub, entityID, txnPublic, string(after))
	return err
}

func (s *Store) GetIntegrationEvent(ctx context.Context, conn IntegrationConnection, externalID string) (IntegrationEventView, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT ev.public_id::text, ev.external_event_id, ev.event_type, ev.status,
       COALESCE(ev.accounting_date::text,''), COALESCE(t.public_id::text,''),
       COALESCE(ev.error_code,''), COALESCE(ev.error_message,''), ev.occurred_at, ev.payload
FROM integration_events ev
LEFT JOIN transactions t ON t.id=ev.transaction_id
WHERE ev.integration_connection_id=$1 AND ev.external_event_id=$2`, conn.ID, externalID)
	if err != nil {
		return IntegrationEventView{}, err
	}
	defer rows.Close()
	items, err := scanEvents(rows)
	if err != nil {
		return IntegrationEventView{}, err
	}
	if len(items) == 0 {
		return IntegrationEventView{}, &integration.Error{Status: 404, Code: "not_found", Message: "integration event not found"}
	}
	return items[0], nil
}

func scanEvents(rows pgx.Rows) ([]IntegrationEventView, error) {
	var out []IntegrationEventView
	for rows.Next() {
		var v IntegrationEventView
		var payload []byte
		if err := rows.Scan(&v.PublicID, &v.ExternalEventID, &v.EventType, &v.Status, &v.AccountingDate, &v.TransactionPublicID, &v.ErrorCode, &v.ErrorMessage, &v.OccurredAt, &payload); err != nil {
			return nil, err
		}
		v.Payload = payload
		out = append(out, v)
	}
	return out, rows.Err()
}
