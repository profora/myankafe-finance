package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/profora/myankafe-finance/backend/internal/config"
)

func TestReadinessReportsDatabaseHealthy(t *testing.T) {
	store := testHTTPStore(t)
	s := &Server{Store: store, Config: config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	s.readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got == "" {
		t.Fatal("expected readiness response body")
	}
}
