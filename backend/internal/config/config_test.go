package config

import (
	"strings"
	"testing"
)

func setMinimumProductionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://finance:secret@db.example.com:5432/finance?sslmode=require")
	t.Setenv("AUTH_MODE", "password")
	t.Setenv("AUTH_COOKIE_SECURE", "true")
	t.Setenv("CORS_ORIGIN", "https://finance.example.com")
	t.Setenv("R2_ENDPOINT", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("METRICS_BEARER_TOKEN", "")
}

func TestProductionRequiresExactHTTPSOrigin(t *testing.T) {
	cases := []string{
		"http://finance.example.com",
		"https://finance.example.com/",
		"https://finance.example.com/path",
		"https://finance.example.com?x=1",
		"*",
	}
	for _, origin := range cases {
		t.Run(origin, func(t *testing.T) {
			setMinimumProductionEnv(t)
			t.Setenv("CORS_ORIGIN", origin)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CORS_ORIGIN") {
				t.Fatalf("Load() error=%v, want CORS_ORIGIN validation error", err)
			}
		})
	}
}

func TestProductionAcceptsHTTPSOriginWithPort(t *testing.T) {
	setMinimumProductionEnv(t)
	t.Setenv("CORS_ORIGIN", "https://finance.example.com:8443")
	if _, err := Load(); err != nil {
		t.Fatalf("Load() error=%v", err)
	}
}

func TestProductionRejectsInsecureR2Endpoint(t *testing.T) {
	setMinimumProductionEnv(t)
	t.Setenv("R2_ENDPOINT", "http://r2.example.com/bucket")
	t.Setenv("R2_ACCESS_KEY_ID", "access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "R2_ENDPOINT") {
		t.Fatalf("Load() error=%v, want R2 endpoint validation error", err)
	}
}

func TestProductionRejectsWeakMetricsToken(t *testing.T) {
	setMinimumProductionEnv(t)
	t.Setenv("METRICS_BEARER_TOKEN", "short-token")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "METRICS_BEARER_TOKEN") {
		t.Fatalf("Load() error=%v, want metrics token validation error", err)
	}
}

func TestProductionAcceptsStrongMetricsToken(t *testing.T) {
	setMinimumProductionEnv(t)
	t.Setenv("METRICS_BEARER_TOKEN", strings.Repeat("a", 32))
	if _, err := Load(); err != nil {
		t.Fatalf("Load() error=%v", err)
	}
}
