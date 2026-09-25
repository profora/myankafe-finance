package config

import (
	"fmt"
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
		CORSOrigin:  getenv("CORS_ORIGIN", "http://localhost:3000"),

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
	}
	return c, nil
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
