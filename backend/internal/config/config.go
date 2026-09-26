package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	HTTPAddr    string
	DatabaseURL string
	AuthMode    string
	CORSOrigin  string

	AuthCookieName   string
	AuthSessionTTL   time.Duration
	AuthCookieSecure bool

	R2Endpoint  string
	R2Bucket    string
	R2Region    string
	R2AccessKey string
	R2SecretKey string

	AttachmentMaxBytes int64
	MetricsBearerToken string
}

func Load() (Config, error) {
	sessionHours, err := strconv.Atoi(getenv("AUTH_SESSION_HOURS", "168"))
	if err != nil || sessionHours < 1 || sessionHours > 24*90 {
		return Config{}, fmt.Errorf("AUTH_SESSION_HOURS must be between 1 and 2160")
	}
	maxMB, err := strconv.Atoi(getenv("ATTACHMENT_MAX_MB", "20"))
	if err != nil || maxMB < 1 || maxMB > 100 {
		return Config{}, fmt.Errorf("ATTACHMENT_MAX_MB must be between 1 and 100")
	}

	c := Config{
		AppEnv:      getenv("APP_ENV", "development"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AuthMode:    getenv("AUTH_MODE", "password"),
		CORSOrigin:  strings.TrimSpace(getenv("CORS_ORIGIN", "http://localhost:3000")),

		AuthCookieName:   getenv("AUTH_COOKIE_NAME", "myankafe_finance_session"),
		AuthSessionTTL:   time.Duration(sessionHours) * time.Hour,
		AuthCookieSecure: envBool("AUTH_COOKIE_SECURE", false),

		R2Endpoint:  strings.TrimRight(strings.TrimSpace(os.Getenv("R2_ENDPOINT")), "/"),
		R2Bucket:    strings.Trim(strings.TrimSpace(os.Getenv("R2_BUCKET")), "/"),
		R2Region:    getenv("R2_REGION", "auto"),
		R2AccessKey: strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),

		AttachmentMaxBytes: int64(maxMB) << 20,
		MetricsBearerToken: strings.TrimSpace(os.Getenv("METRICS_BEARER_TOKEN")),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.AuthMode != "password" && c.AuthMode != "dev" {
		return c, fmt.Errorf("AUTH_MODE must be password or dev")
	}
	if c.AppEnv == "production" {
		if c.AuthMode != "password" {
			return c, fmt.Errorf("AUTH_MODE=password is required in production")
		}
		if !c.AuthCookieSecure {
			return c, fmt.Errorf("AUTH_COOKIE_SECURE=true is required in production")
		}
		if err := validateProductionOrigin(c.CORSOrigin); err != nil {
			return c, err
		}
		if c.R2Endpoint != "" && !strings.HasPrefix(strings.ToLower(c.R2Endpoint), "https://") {
			return c, fmt.Errorf("R2_ENDPOINT must use https in production")
		}
		if c.MetricsBearerToken != "" && len(c.MetricsBearerToken) < 32 {
			return c, fmt.Errorf("METRICS_BEARER_TOKEN must be at least 32 characters in production")
		}
	}
	return c, nil
}

func validateProductionOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("CORS_ORIGIN must be one exact https origin in production")
	}
	return nil
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envBool(k string, d bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return d
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return d
	}
}
