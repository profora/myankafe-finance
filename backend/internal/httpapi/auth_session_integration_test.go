package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/auth"
	"github.com/profora/myankafe-finance/backend/internal/config"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func seedPasswordUser(t *testing.T, store *postgres.Store) (username, password, publicID string) {
	t.Helper()
	ctx := context.Background()
	internalID, err := ids.UUIDv7()
	if err != nil {
		t.Fatal(err)
	}
	publicID, err = ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	username = "login-" + strings.ToLower(publicID)
	password = "correct horse battery staple"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO users(id,public_id,username,display_name)
VALUES($1,$2,$3,'Login Test User')`, internalID, publicID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO user_credentials(user_id,password_hash)
VALUES($1,$2)`, internalID, hash); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return username, password, publicID
}

func passwordRouter(store *postgres.Store) http.Handler {
	return New(store, config.Config{
		AppEnv:             "development",
		AuthMode:           "password",
		CORSOrigin:         "http://localhost:3000",
		AuthCookieName:     "finance_test_session",
		AuthSessionTTL:     time.Hour,
		AttachmentMaxBytes: 20 << 20,
	})
}

func jsonRequest(t *testing.T, h http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPasswordSessionLifecycle(t *testing.T) {
	store := testHTTPStore(t)
	username, password, publicID := seedPasswordUser(t, store)
	router := passwordRouter(store)

	login := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "finance_test_session" || cookies[0].Value == "" {
		t.Fatalf("unexpected login cookies: %+v", cookies)
	}
	if !cookies[0].HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}

	me := jsonRequest(t, router, http.MethodGet, "/api/v1/auth/me", nil, cookies[0])
	if me.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
	if !strings.Contains(me.Body.String(), publicID) {
		t.Fatalf("me response missing public id: %s", me.Body.String())
	}

	logout := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/logout", map[string]any{}, cookies[0])
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}

	reused := jsonRequest(t, router, http.MethodGet, "/api/v1/auth/me", nil, cookies[0])
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status=%d body=%s", reused.Code, reused.Body.String())
	}
}

func TestPasswordLoginRejectsWrongPassword(t *testing.T) {
	store := testHTTPStore(t)
	username, _, _ := seedPasswordUser(t, store)
	router := passwordRouter(store)

	login := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": "this-is-not-the-right-password",
	})
	if login.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-password status=%d body=%s", login.Code, login.Body.String())
	}
}

func TestPasswordChangeRotatesSessionsAndCredential(t *testing.T) {
	store := testHTTPStore(t)
	username, password, _ := seedPasswordUser(t, store)
	router := passwordRouter(store)

	login := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	oldCookie := login.Result().Cookies()[0]
	newPassword := "updated correct horse battery"

	change := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"current_password": password,
		"new_password":     newPassword,
	}, oldCookie)
	if change.Code != http.StatusOK {
		t.Fatalf("change-password status=%d body=%s", change.Code, change.Body.String())
	}
	if strings.Contains(change.Body.String(), "session_token") {
		t.Fatalf("cookie clients must not receive a bearer token: %s", change.Body.String())
	}
	replacementCookies := change.Result().Cookies()
	if len(replacementCookies) != 1 || replacementCookies[0].Value == "" {
		t.Fatalf("missing replacement cookie: %+v", replacementCookies)
	}
	replacement := replacementCookies[0]

	oldSession := jsonRequest(t, router, http.MethodGet, "/api/v1/auth/me", nil, oldCookie)
	if oldSession.Code != http.StatusUnauthorized {
		t.Fatalf("old session status=%d body=%s", oldSession.Code, oldSession.Body.String())
	}
	newSession := jsonRequest(t, router, http.MethodGet, "/api/v1/auth/me", nil, replacement)
	if newSession.Code != http.StatusOK {
		t.Fatalf("replacement session status=%d body=%s", newSession.Code, newSession.Body.String())
	}

	oldPasswordLogin := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if oldPasswordLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status=%d body=%s", oldPasswordLogin.Code, oldPasswordLogin.Body.String())
	}

	newPasswordLogin := jsonRequest(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": newPassword,
	})
	if newPasswordLogin.Code != http.StatusOK {
		t.Fatalf("new password login status=%d body=%s", newPasswordLogin.Code, newPasswordLogin.Body.String())
	}
}

func TestBearerPasswordChangeReturnsReplacementToken(t *testing.T) {
	store := testHTTPStore(t)
	username, password, _ := seedPasswordUser(t, store)
	router := passwordRouter(store)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"`+username+`","password":"`+password+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-Session-Transport", "bearer")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("bearer login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	var loginBody map[string]any
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	token, _ := loginBody["session_token"].(string)
	if token == "" {
		t.Fatalf("bearer login missing session_token: %s", loginRec.Body.String())
	}
	if len(loginRec.Result().Cookies()) == 0 {
		t.Fatal("bearer login should still set the web session cookie")
	}

	newPassword := "replacement horse battery staple"
	body := `{"current_password":"` + password + `","new_password":"` + newPassword + `"}`
	changeReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(body))
	changeReq.Header.Set("Content-Type", "application/json")
	changeReq.Header.Set("X-Session-Transport", "bearer")
	changeReq.Header.Set("Authorization", "Bearer "+token)
	changeRec := httptest.NewRecorder()
	router.ServeHTTP(changeRec, changeReq)
	if changeRec.Code != http.StatusOK {
		t.Fatalf("bearer change status=%d body=%s", changeRec.Code, changeRec.Body.String())
	}
	var changed map[string]any
	if err := json.Unmarshal(changeRec.Body.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	next, _ := changed["session_token"].(string)
	if next == "" || next == token {
		t.Fatalf("expected a new bearer token, got %q", next)
	}

	oldMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	oldMe.Header.Set("Authorization", "Bearer "+token)
	oldRec := httptest.NewRecorder()
	router.ServeHTTP(oldRec, oldMe)
	if oldRec.Code != http.StatusUnauthorized {
		t.Fatalf("old bearer should be revoked, status=%d", oldRec.Code)
	}
	newMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	newMe.Header.Set("Authorization", "Bearer "+next)
	newRec := httptest.NewRecorder()
	router.ServeHTTP(newRec, newMe)
	if newRec.Code != http.StatusOK {
		t.Fatalf("replacement bearer status=%d body=%s", newRec.Code, newRec.Body.String())
	}
}
