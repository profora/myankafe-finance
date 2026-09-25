package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/config"
)

func TestMetricsHiddenInProductionWithoutToken(t *testing.T) {
	s := &Server{
		Config:  config.Config{AppEnv: "production"},
		Metrics: newHTTPMetrics(),
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	s.metrics(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rec.Code)
	}
}

func TestMetricsRequiresConfiguredBearerToken(t *testing.T) {
	s := &Server{
		Config: config.Config{
			AppEnv:             "production",
			MetricsBearerToken: "secret-token",
		},
		Metrics: newHTTPMetrics(),
	}
	s.Metrics.begin()
	s.Metrics.finish(http.MethodGet, http.StatusOK, 25*time.Millisecond)

	for _, tc := range []struct {
		name   string
		auth   string
		status int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong", auth: "Bearer wrong", status: http.StatusUnauthorized},
		{name: "correct", auth: "Bearer secret-token", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()

			s.metrics(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status == http.StatusOK {
				body := rec.Body.String()
				if !strings.Contains(body, "myankafe_finance_http_requests_total") {
					t.Fatalf("missing request counter: %s", body)
				}
				if !strings.Contains(body, "method=\"GET\",status_class=\"2xx\"") {
					t.Fatalf("missing GET 2xx series: %s", body)
				}
			}
		})
	}
}

func TestStatusClass(t *testing.T) {
	for status, want := range map[int]string{
		200: "2xx",
		302: "3xx",
		404: "4xx",
		503: "5xx",
		0:   "other",
	} {
		if got := statusClass(status); got != want {
			t.Fatalf("statusClass(%d)=%q want %q", status, got, want)
		}
	}
}
