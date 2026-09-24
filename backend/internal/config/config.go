package config

import (
  "fmt"
  "os"
)

type Config struct {
  AppEnv, HTTPAddr, DatabaseURL, AuthMode, CORSOrigin string
}

func Load() (Config, error) {
  c := Config{
    AppEnv: getenv("APP_ENV", "development"),
    HTTPAddr: getenv("HTTP_ADDR", ":8080"),
    DatabaseURL: os.Getenv("DATABASE_URL"),
    AuthMode: getenv("AUTH_MODE", "dev"),
    CORSOrigin: getenv("CORS_ORIGIN", "http://localhost:3000"),
  }
  if c.DatabaseURL == "" {
    return c, fmt.Errorf("DATABASE_URL is required")
  }
  if c.AppEnv == "production" && c.AuthMode == "dev" {
    return c, fmt.Errorf("AUTH_MODE=dev is forbidden in production")
  }
  return c, nil
}

func getenv(k, d string) string {
  if v := os.Getenv(k); v != "" { return v }
  return d
}
