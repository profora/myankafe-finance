package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/profora/myankafe-finance/backend/internal/auth"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func testHTTPStore(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &postgres.Store{Pool: pool}
}

func performIdempotentRequest(t *testing.T, h http.Handler, principal, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test?mode=write", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer dev:"+principal)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHTTPIdempotencyReplaysAndRejectsMismatchedReuse(t *testing.T) {
	store := testHTTPStore(t)
	server := &Server{Store: store}

	principal, err := ids.ULID()
	if err != nil {
		t.Fatal(err)
	}
	key := "http-test-" + principal

	var executions atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := executions.Add(1)
		write(w, http.StatusCreated, map[string]any{"execution": n})
	})

	handler := auth.Middleware("dev")(server.idempotency(next))

	first := performIdempotentRequest(t, handler, principal, key, `{"amount":100}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	if executions.Load() != 1 {
		t.Fatalf("downstream executions=%d want 1", executions.Load())
	}

	replay := performIdempotentRequest(t, handler, principal, key, `{"amount":100}`)
	if replay.Code != http.StatusCreated {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	if replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("missing replay header: %v", replay.Header())
	}
	var firstJSON, replayJSON map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayJSON, firstJSON) {
		t.Fatalf("replayed JSON=%v first=%v", replayJSON, firstJSON)
	}
	if executions.Load() != 1 {
		t.Fatalf("replay re-executed downstream: %d", executions.Load())
	}

	mismatch := performIdempotentRequest(t, handler, principal, key, `{"amount":101}`)
	if mismatch.Code != http.StatusConflict {
		t.Fatalf("mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
	if executions.Load() != 1 {
		t.Fatalf("mismatched reuse executed downstream: %d", executions.Load())
	}
}
