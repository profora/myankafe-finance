package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/profora/myankafe-finance/backend/internal/config"
)

func TestOriginGuardRejectsUnexpectedMutationOrigin(t *testing.T) {
	s := &Server{Config: config.Config{CORSOrigin: "https://finance.example.com"}}
	called := false
	h := s.originGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/api/v1/test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if called {
		t.Fatal("downstream handler must not run")
	}
}

func TestOriginGuardAllowsConfiguredOriginAndNativeNoOrigin(t *testing.T) {
	s := &Server{Config: config.Config{CORSOrigin: "https://finance.example.com"}}
	for _, origin := range []string{"https://finance.example.com", ""} {
		t.Run(origin, func(t *testing.T) {
			called := false
			h := s.originGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodDelete, "https://api.example.com/api/v1/test", nil)
			if origin != "" {
				req.Header.Set("Origin", origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status=%d want 204", rec.Code)
			}
			if !called {
				t.Fatal("downstream handler should run")
			}
		})
	}
}
