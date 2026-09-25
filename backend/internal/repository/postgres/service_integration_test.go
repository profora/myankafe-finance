package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func integrationStore(t *testing.T) (context.Context, *Store) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return ctx, &Store{Pool: pool}
}

func seedServiceEntity(t *testing.T, ctx context.Context, s *Store, suffix string) (User, Entity, string, string) {
	t.Helper()

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	userID := mustUUID(t)
	userPublic := mustULID(t)
	username := strings.ToLower("svc-" + strings.ToLower(suffix) + "-" + strings.ToLower(userPublic))
	if _, err := tx.Exec(ctx, `
INSERT INTO users(id,public_id,username,display_name)
VALUES($1,$2,$3,$4)`, userID, userPublic, username, "Service Test User"); err != nil {
		t.Fatal(err)
	}

	entityID := mustUUID(t)
	entityPublic := mustULID(t)
	entityCode := "SVC_" + suffix + "_" + entityPublic
	if _, err := tx.Exec(ctx, `
INSERT INTO entities(
  id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,$4,'BUSINESS','MMK','Asia/Yangon',$5)`,
		entityID, entityPublic, entityCode, "Service Test "+suffix, userID); err != nil {
		t.Fatal(err)
	}

	expenseID := mustUUID(t)
	expensePublic := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,'EXPENSE',true,$6)`,
		expenseID, expensePublic, entityID, "EXP_"+expensePublic, "Test Expense", userID); err != nil {
		t.Fatal(err)
	}

	cashID := mustUUID(t)
	cashPublic := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,'ASSET',true,$6)`,
		cashID, cashPublic, entityID, "CASH_"+cashPublic, "Test Cash", userID); err != nil {
		t.Fatal(err)
	}

	faID := mustUUID(t)
	faPublic := mustULID(t)
	if _, err := tx.Exec(ctx, `
INSERT INTO financial_accounts(
  id,public_id,entity_id,account_id,code,name,kind,currency_code,created_by
) VALUES($1,$2,$3,$4,$5,$6,'CASH','MMK',$7)`,
		faID, faPublic, entityID, cashID, "FA_"+faPublic, "Test Cash Account", userID); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	return User{
			ID: userID, PublicID: userPublic, Username: username, DisplayName: "Service Test User",
		}, Entity{
			ID: entityID, PublicID: entityPublic, Code: entityCode, Name: "Service Test " + suffix,
			Type: "BUSINESS", FunctionalCurrency: "MMK", Timezone: "Asia/Yangon", FiscalMonth: 1, FiscalDay: 1,
		}, expensePublic, faPublic
}

func TestIdempotencyClaimCompleteAndReplay(t *testing.T) {
	ctx, s := integrationStore(t)
	scope := "test:" + mustULID(t)
	key := "key-" + mustULID(t)
	requestHash := "hash-a"

	claimed, existing, err := s.ClaimIdempotency(ctx, scope, key, requestHash)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatalf("first claim should succeed: %+v", existing)
	}

	if err := s.CompleteIdempotency(ctx, scope, key, 201, `{"ok":true}`); err != nil {
		t.Fatal(err)
	}

	claimed, existing, err = s.ClaimIdempotency(ctx, scope, key, requestHash)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("completed idempotency key must not be claimed twice")
	}
	if existing.RequestHash != requestHash {
		t.Fatalf("request hash=%q want %q", existing.RequestHash, requestHash)
	}
	if existing.ResponseStatus == nil || *existing.ResponseStatus != 201 {
		t.Fatalf("response status=%v want 201", existing.ResponseStatus)
	}
	if existing.ResponseBody == nil || *existing.ResponseBody != `{"ok": true}` {
		t.Fatalf("response body=%v", existing.ResponseBody)
	}
}

func TestIdempotencyConcurrentClaimHasSingleWinner(t *testing.T) {
	ctx, s := integrationStore(t)
	scope := "concurrent:" + mustULID(t)
	key := "key-" + mustULID(t)
	requestHash := "same-request"

	var winners atomic.Int32
	var failures atomic.Int32
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, _, err := s.ClaimIdempotency(ctx, scope, key, requestHash)
			if err != nil {
				failures.Add(1)
				return
			}
			if claimed {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()

	if failures.Load() != 0 {
		t.Fatalf("unexpected claim failures: %d", failures.Load())
	}
	if winners.Load() != 1 {
		t.Fatalf("idempotency concurrent winners=%d want 1", winners.Load())
	}
}

func TestPostedTransactionReversalCreatesOppositeJournal(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "REVERSAL")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-24",
		Description:              "Service reversal test",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{
			AccountPublicID: expenseAccount,
			Amount:          "12500",
			Description:     "Test expense",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
		t.Fatal(err)
	}

	result, err := s.ReverseTransaction(ctx, user, entity, draft.PublicID, "2026-09-25", "correction test")
	if err != nil {
		t.Fatal(err)
	}

	reversalPublic, ok := result["reversal_transaction_id"].(string)
	if !ok || reversalPublic == "" {
		t.Fatalf("missing reversal public id: %+v", result)
	}

	var originalStatus, reversalStatus, originalJournalStatus, reversalJournalStatus string
	var originalDebit, originalCredit, reversalDebit, reversalCredit string
	err = s.Pool.QueryRow(ctx, `
SELECT
  ot.status,
  rt.status,
  oje.status,
  rje.status,
  (SELECT COALESCE(SUM(debit_amount),0)::text FROM journal_lines WHERE journal_entry_id=oje.id),
  (SELECT COALESCE(SUM(credit_amount),0)::text FROM journal_lines WHERE journal_entry_id=oje.id),
  (SELECT COALESCE(SUM(debit_amount),0)::text FROM journal_lines WHERE journal_entry_id=rje.id),
  (SELECT COALESCE(SUM(credit_amount),0)::text FROM journal_lines WHERE journal_entry_id=rje.id)
FROM transactions ot
JOIN transactions rt ON rt.original_transaction_id=ot.id
JOIN journal_entries oje ON oje.transaction_id=ot.id
JOIN journal_entries rje ON rje.transaction_id=rt.id
WHERE ot.entity_id=$1 AND ot.public_id=$2 AND rt.public_id=$3`,
		entity.ID, draft.PublicID, reversalPublic).
		Scan(
			&originalStatus, &reversalStatus,
			&originalJournalStatus, &reversalJournalStatus,
			&originalDebit, &originalCredit, &reversalDebit, &reversalCredit,
		)
	if err != nil {
		t.Fatal(err)
	}

	if originalStatus != "VOIDED" || reversalStatus != "POSTED" {
		t.Fatalf("transaction states original=%s reversal=%s", originalStatus, reversalStatus)
	}
	if originalJournalStatus != "REVERSED" || reversalJournalStatus != "POSTED" {
		t.Fatalf("journal states original=%s reversal=%s", originalJournalStatus, reversalJournalStatus)
	}
	if originalDebit != originalCredit || reversalDebit != reversalCredit {
		t.Fatalf("journals not balanced original=%s/%s reversal=%s/%s",
			originalDebit, originalCredit, reversalDebit, reversalCredit)
	}

	var net string
	if err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(SUM(jl.debit_amount-jl.credit_amount),0)::text
FROM journal_lines jl
JOIN journal_entries je ON je.id=jl.journal_entry_id
JOIN transactions t ON t.id=je.transaction_id
WHERE t.entity_id=$1 AND (t.public_id=$2 OR t.public_id=$3)`,
		entity.ID, draft.PublicID, reversalPublic).Scan(&net); err != nil {
		t.Fatal(err)
	}
	if net != "0.000000" && net != "0" {
		t.Fatalf("original plus reversal net=%s want 0", net)
	}
}

func TestReversalInsideLockedPeriodIsRejectedWithoutMutation(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "LOCKED_REVERSAL")

	draft, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-24",
		Description:              "Locked reversal test",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits:                   []SplitInput{{AccountPublicID: expenseAccount, Amount: "5000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostTransaction(ctx, user, entity, draft.PublicID); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(ctx, user, entity, "2026-09-30", "month close"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.ReverseTransaction(ctx, user, entity, draft.PublicID, "2026-09-30", "should fail"); err == nil {
		t.Fatal("expected reversal inside locked period to fail")
	}

	var status string
	var reversalCount int
	if err := s.Pool.QueryRow(ctx, `
SELECT status,
       (SELECT count(*) FROM transactions r WHERE r.original_transaction_id=t.id)
FROM transactions t
WHERE t.entity_id=$1 AND t.public_id=$2`, entity.ID, draft.PublicID).
		Scan(&status, &reversalCount); err != nil {
		t.Fatal(err)
	}
	if status != "POSTED" || reversalCount != 0 {
		t.Fatalf("locked reversal mutated source: status=%s reversals=%d", status, reversalCount)
	}
}

func TestIdempotencyReleaseAllowsRetry(t *testing.T) {
	ctx, s := integrationStore(t)
	scope := "release:" + mustULID(t)
	key := "key-" + mustULID(t)

	claimed, _, err := s.ClaimIdempotency(ctx, scope, key, "hash")
	if err != nil || !claimed {
		t.Fatalf("initial claim: claimed=%v err=%v", claimed, err)
	}
	if err := s.ReleaseIdempotency(ctx, scope, key); err != nil {
		t.Fatal(err)
	}
	claimed, _, err = s.ClaimIdempotency(ctx, scope, key, "hash")
	if err != nil || !claimed {
		t.Fatalf("retry claim: claimed=%v err=%v", claimed, err)
	}
}

func seedTypedAccountCommitted(t *testing.T, ctx context.Context, s *Store, entity Entity, user User, accountType, label string) (string, string) {
	t.Helper()
	id := mustUUID(t)
	publicID := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,$6,true,$7)`,
		id, publicID, entity.ID, label+"_"+publicID, label, accountType, user.ID); err != nil {
		t.Fatal(err)
	}
	return id, publicID
}

func seedInterEntityMappings(
	t *testing.T,
	ctx context.Context,
	s *Store,
	user User,
	left, right Entity,
) (leftDueFromID, leftDueToID, rightDueFromID, rightDueToID string) {
	t.Helper()

	leftDueFromID, _ = seedTypedAccountCommitted(t, ctx, s, left, user, "ASSET", "DUE_FROM")
	leftDueToID, _ = seedTypedAccountCommitted(t, ctx, s, left, user, "LIABILITY", "DUE_TO")
	rightDueFromID, _ = seedTypedAccountCommitted(t, ctx, s, right, user, "ASSET", "DUE_FROM")
	rightDueToID, _ = seedTypedAccountCommitted(t, ctx, s, right, user, "LIABILITY", "DUE_TO")

	if _, err := s.Pool.Exec(ctx, `
INSERT INTO inter_entity_account_mappings(
  id,entity_id,counterparty_entity_id,due_from_account_id,due_to_account_id,created_by
) VALUES
($1,$2,$3,$4,$5,$6),
($7,$3,$2,$8,$9,$6)`,
		mustUUID(t), left.ID, right.ID, leftDueFromID, leftDueToID, user.ID,
		mustUUID(t), rightDueFromID, rightDueToID); err != nil {
		t.Fatal(err)
	}

	return
}

func TestInterEntityPostingCreatesBalancedPair(t *testing.T) {
	ctx, s := integrationStore(t)
	user, left, _, leftFinancial := seedServiceEntity(t, ctx, s, "INTER_LEFT")
	_, right, rightExpense, _ := seedServiceEntity(t, ctx, s, "INTER_RIGHT")

	leftDueFrom, _, _, rightDueTo := seedInterEntityMappings(t, ctx, s, user, left, right)

	result, err := s.PostInterEntityExpense(ctx, user, left, right, InterEntityExpenseInput{
		Date:                               "2026-09-24",
		InitiatingFinancialAccountPublicID: leftFinancial,
		InitiatingAmount:                   "10000",
		CounterpartyAmount:                 "10000",
		CounterpartyExpenseAccountPublicID: rightExpense,
		Description:                        "Atomic success " + mustULID(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["id"] == nil {
		t.Fatalf("missing inter-entity public id: %+v", result)
	}

	var dueFromBalance, dueToBalance string
	if err := s.Pool.QueryRow(ctx, `
SELECT
  COALESCE((SELECT SUM(jl.debit_amount-jl.credit_amount)
            FROM journal_lines jl JOIN journal_entries je ON je.id=jl.journal_entry_id
            WHERE jl.account_id=$1 AND je.status='POSTED'),0)::text,
  COALESCE((SELECT SUM(jl.credit_amount-jl.debit_amount)
            FROM journal_lines jl JOIN journal_entries je ON je.id=jl.journal_entry_id
            WHERE jl.account_id=$2 AND je.status='POSTED'),0)::text`,
		leftDueFrom, rightDueTo).Scan(&dueFromBalance, &dueToBalance); err != nil {
		t.Fatal(err)
	}
	if dueFromBalance != "10000.000000" || dueToBalance != "10000.000000" {
		t.Fatalf("inter-entity balances due-from=%s due-to=%s", dueFromBalance, dueToBalance)
	}
}

func TestInterEntityPostingRollsBackAfterLateDatabaseFailure(t *testing.T) {
	ctx, s := integrationStore(t)
	user, left, _, leftFinancial := seedServiceEntity(t, ctx, s, "ROLLBACK_LEFT")
	_, right, rightExpense, _ := seedServiceEntity(t, ctx, s, "ROLLBACK_RIGHT")
	_, _, _, rightDueTo := seedInterEntityMappings(t, ctx, s, user, left, right)

	const functionName = "test_force_interentity_failure"
	const triggerName = "trg_test_force_interentity_failure"
	_, _ = s.Pool.Exec(ctx, "DROP TRIGGER IF EXISTS "+triggerName+" ON journal_lines")
	_, _ = s.Pool.Exec(ctx, "DROP FUNCTION IF EXISTS "+functionName+"()")
	t.Cleanup(func() {
		_, _ = s.Pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS "+triggerName+" ON journal_lines")
		_, _ = s.Pool.Exec(context.Background(), "DROP FUNCTION IF EXISTS "+functionName+"()")
	})

	functionSQL := fmt.Sprintf(`
CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $test$
BEGIN
  IF NEW.entity_id = '%s'::uuid AND NEW.account_id = '%s'::uuid THEN
    RAISE EXCEPTION 'forced late inter-entity test failure';
  END IF;
  RETURN NEW;
END;
$test$`, functionName, right.ID, rightDueTo)
	if _, err := s.Pool.Exec(ctx, functionSQL); err != nil {
		t.Fatal(err)
	}

	triggerSQL := fmt.Sprintf(`
CREATE TRIGGER %s
BEFORE INSERT ON journal_lines
FOR EACH ROW EXECUTE FUNCTION %s()`, triggerName, functionName)
	if _, err := s.Pool.Exec(ctx, triggerSQL); err != nil {
		t.Fatal(err)
	}

	description := "Atomic rollback " + mustULID(t)
	if _, err := s.PostInterEntityExpense(ctx, user, left, right, InterEntityExpenseInput{
		Date:                               "2026-09-24",
		InitiatingFinancialAccountPublicID: leftFinancial,
		InitiatingAmount:                   "15000",
		CounterpartyAmount:                 "15000",
		CounterpartyExpenseAccountPublicID: rightExpense,
		Description:                        description,
	}); err == nil {
		t.Fatal("expected forced late database failure")
	}

	var transactions, pairs, journals, lines int
	if err := s.Pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM transactions WHERE description=$1),
  (SELECT count(*) FROM inter_entity_transactions WHERE description=$1),
  (SELECT count(*) FROM journal_entries WHERE description=$1),
  (SELECT count(*) FROM journal_lines WHERE description=$1)`, description).
		Scan(&transactions, &pairs, &journals, &lines); err != nil {
		t.Fatal(err)
	}
	if transactions != 0 || pairs != 0 || journals != 0 || lines != 0 {
		t.Fatalf("partial inter-entity data survived rollback: transactions=%d pairs=%d journals=%d lines=%d",
			transactions, pairs, journals, lines)
	}
}

func TestServiceFixtureUniqueness(t *testing.T) {
	// Small guard against accidentally changing test ID helpers to static values.
	a, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal(fmt.Errorf("generated duplicate ULID %s", a))
	}
}
