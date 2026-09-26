package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/profora/myankafe-finance/backend/internal/config"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

var roleIDs = map[string]string{
	"OWNER":      "00000000-0000-7000-8000-000000000001",
	"ADMIN":      "00000000-0000-7000-8000-000000000002",
	"ACCOUNTANT": "00000000-0000-7000-8000-000000000003",
	"BOOKKEEPER": "00000000-0000-7000-8000-000000000004",
	"VIEWER":     "00000000-0000-7000-8000-000000000005",
}

func seedHTTPRole(t *testing.T, store *postgres.Store, role string) (string, string) {
	t.Helper()
	ctx := context.Background()

	userID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	userPublic, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
INSERT INTO users(id,public_id,username,display_name)
VALUES($1,$2,$3,$4)`,
		userID, userPublic, strings.ToLower("auth-"+strings.ToLower(role)+"-"+strings.ToLower(userPublic)), role+" Test User"); err != nil {
		t.Fatal(err)
	}

	entityID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	entityPublic, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
INSERT INTO entities(
  id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,$4,'BUSINESS','MMK','Asia/Yangon',$5)`,
		entityID, entityPublic, "AUTH_"+role+"_"+entityPublic, role+" Test Entity", userID); err != nil {
		t.Fatal(err)
	}

	linkID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,$4,$2)`, linkID, userID, entityID, roleIDs[role]); err != nil {
		t.Fatal(err)
	}

	return userPublic, entityPublic
}

func testRouter(store *postgres.Store) http.Handler {
	return New(store, config.Config{
		AppEnv:     "development",
		AuthMode:   "dev",
		CORSOrigin: "http://localhost:3000",
	})
}

func performAuthorizedJSON(
	t *testing.T,
	h http.Handler,
	method, path, principal string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer dev:"+principal)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBookkeeperCannotConfigureChartOfAccounts(t *testing.T) {
	store := testHTTPStore(t)
	principal, entity := seedHTTPRole(t, store, "BOOKKEEPER")
	router := testRouter(store)

	rec := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/entities/"+entity+"/accounts", principal, map[string]any{
		"Code": "6000", "Name": "Restricted Config", "Type": "EXPENSE", "Postable": true,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("BOOKKEEPER configure account status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountantCanConfigureChartOfAccounts(t *testing.T) {
	store := testHTTPStore(t)
	principal, entity := seedHTTPRole(t, store, "ACCOUNTANT")
	router := testRouter(store)

	rec := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/entities/"+entity+"/accounts", principal, map[string]any{
		"Code": "6000", "Name": "Allowed Config", "Type": "EXPENSE", "Postable": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("ACCOUNTANT configure account status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestViewerCannotCreateLedgerTransaction(t *testing.T) {
	store := testHTTPStore(t)
	principal, entity := seedHTTPRole(t, store, "VIEWER")
	router := testRouter(store)

	rec := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/entities/"+entity+"/transactions", principal, map[string]any{})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("VIEWER create transaction status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminCannotReversePostedAccounting(t *testing.T) {
	store := testHTTPStore(t)
	principal, entity := seedHTTPRole(t, store, "ADMIN")
	router := testRouter(store)
	transactionID, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}

	rec := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/entities/"+entity+"/transactions/"+transactionID+"/reverse", principal, map[string]any{
		"reversal_date": "2026-09-24",
		"reason":        "authorization check",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ADMIN reverse status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuditVisibilityRequiresAccountingConfigurationRole(t *testing.T) {
	store := testHTTPStore(t)
	bookkeeper, bookkeeperEntity := seedHTTPRole(t, store, "BOOKKEEPER")
	accountant, accountantEntity := seedHTTPRole(t, store, "ACCOUNTANT")
	router := testRouter(store)

	denied := performAuthorizedJSON(t, router, http.MethodGet, "/api/v1/entities/"+bookkeeperEntity+"/audit-events", bookkeeper, nil)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("BOOKKEEPER audit status=%d body=%s", denied.Code, denied.Body.String())
	}

	allowed := performAuthorizedJSON(t, router, http.MethodGet, "/api/v1/entities/"+accountantEntity+"/audit-events", accountant, nil)
	if allowed.Code != http.StatusOK {
		t.Fatalf("ACCOUNTANT audit status=%d body=%s", allowed.Code, allowed.Body.String())
	}
}

func TestInterEntitySetupRejectsOperationalRoles(t *testing.T) {
	store := testHTTPStore(t)
	router := testRouter(store)
	for _, role := range []string{"ACCOUNTANT", "BOOKKEEPER", "VIEWER"} {
		principal, entity := seedHTTPRole(t, store, role)
		setup := performAuthorizedJSON(t, router, http.MethodGet, "/api/v1/entities/"+entity+"/inter-entity-setup", principal, nil)
		if setup.Code != http.StatusForbidden {
			t.Fatalf("%s setup status=%d body=%s", role, setup.Code, setup.Body.String())
		}
		save := performAuthorizedJSON(t, router, http.MethodPut, "/api/v1/entities/"+entity+"/inter-entity-pairs/"+entity, principal, map[string]any{})
		if save.Code != http.StatusForbidden {
			t.Fatalf("%s save status=%d body=%s", role, save.Code, save.Body.String())
		}
	}

	owner, entity := seedHTTPRole(t, store, "OWNER")
	allowed := performAuthorizedJSON(t, router, http.MethodGet, "/api/v1/entities/"+entity+"/inter-entity-setup", owner, nil)
	if allowed.Code != http.StatusOK {
		t.Fatalf("OWNER setup status=%d body=%s", allowed.Code, allowed.Body.String())
	}
	admin, adminEntity := seedHTTPRole(t, store, "ADMIN")
	adminAllowed := performAuthorizedJSON(t, router, http.MethodGet, "/api/v1/entities/"+adminEntity+"/inter-entity-setup", admin, nil)
	if adminAllowed.Code != http.StatusOK {
		t.Fatalf("ADMIN setup status=%d body=%s", adminAllowed.Code, adminAllowed.Body.String())
	}
}

func TestOwnerAndAdminCanSaveInterEntityPair(t *testing.T) {
	store := testHTTPStore(t)
	router := testRouter(store)
	for _, role := range []string{"OWNER", "ADMIN"} {
		principal, payer := seedHTTPRole(t, store, role)
		_, counterparty := seedHTTPRole(t, store, "OWNER")
		grantPublicRole(t, store, principal, counterparty, role)
		payerFrom := createHTTPAccount(t, router, principal, payer, "DF", "Due from", "ASSET")
		payerTo := createHTTPAccount(t, router, principal, payer, "DT", "Due to", "LIABILITY")
		cpFrom := createHTTPAccount(t, router, principal, counterparty, "CDF", "Counterparty due from", "ASSET")
		cpTo := createHTTPAccount(t, router, principal, counterparty, "CDT", "Counterparty due to", "LIABILITY")
		body := map[string]any{
			"PayerDueFromAccountID": payerFrom, "PayerDueToAccountID": payerTo,
			"CounterpartyDueFromAccountID": cpFrom, "CounterpartyDueToAccountID": cpTo,
		}
		created := performAuthorizedJSON(t, router, http.MethodPut, "/api/v1/entities/"+payer+"/inter-entity-pairs/"+counterparty, principal, body)
		if created.Code != http.StatusOK {
			t.Fatalf("%s create status=%d body=%s", role, created.Code, created.Body.String())
		}
		updated := performAuthorizedJSON(t, router, http.MethodPut, "/api/v1/entities/"+payer+"/inter-entity-pairs/"+counterparty, principal, body)
		if updated.Code != http.StatusOK {
			t.Fatalf("%s update status=%d body=%s", role, updated.Code, updated.Body.String())
		}
	}
}

func createHTTPAccount(t *testing.T, router http.Handler, principal, entity, code, name, accountType string) string {
	t.Helper()
	rec := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/entities/"+entity+"/accounts", principal, map[string]any{
		"Code": code + entity[:6], "Name": name, "Type": accountType, "Postable": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create account status=%d body=%s", rec.Code, rec.Body.String())
	}
	var account postgres.Account
	if err := json.Unmarshal(rec.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}
	return account.PublicID
}

func TestDailyInterEntityPostingRequiresBothEntities(t *testing.T) {
	store := testHTTPStore(t)
	router := testRouter(store)
	bookkeeper, payer := seedHTTPRole(t, store, "BOOKKEEPER")
	_, counterparty := seedHTTPRole(t, store, "OWNER")
	denied := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/inter-entity-transactions", bookkeeper, map[string]any{
		"InitiatingEntityID": payer, "CounterpartyEntityID": counterparty, "Date": "2026-09-26",
		"InitiatingAmount": "10", "CounterpartyAmount": "10",
	})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("payer-only post status=%d body=%s", denied.Code, denied.Body.String())
	}

	viewer, viewerEntity := seedHTTPRole(t, store, "VIEWER")
	_, viewerCounterparty := seedHTTPRole(t, store, "VIEWER")
	grantPublicRole(t, store, viewer, viewerCounterparty, "VIEWER")
	viewerPost := performAuthorizedJSON(t, router, http.MethodPost, "/api/v1/inter-entity-transactions", viewer, map[string]any{
		"InitiatingEntityID": viewerEntity, "CounterpartyEntityID": viewerCounterparty, "Date": "2026-09-26",
		"InitiatingAmount": "10", "CounterpartyAmount": "10",
	})
	if viewerPost.Code != http.StatusForbidden {
		t.Fatalf("VIEWER post status=%d body=%s", viewerPost.Code, viewerPost.Body.String())
	}
}

func grantPublicRole(t *testing.T, store *postgres.Store, userPublic, entityPublic, role string) {
	t.Helper()
	ctx := context.Background()
	var userID, entityID string
	if err := store.Pool.QueryRow(ctx, `SELECT id::text FROM users WHERE public_id=$1`, userPublic).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT id::text FROM entities WHERE public_id=$1`, entityPublic).Scan(&entityID); err != nil {
		t.Fatal(err)
	}
	linkID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,$4,$2)`, linkID, userID, entityID, roleIDs[role]); err != nil {
		t.Fatal(err)
	}
}
