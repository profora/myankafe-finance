package postgres

import (
	"context"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

const ownerRoleID = "00000000-0000-7000-8000-000000000001"

func auditRequestID(ctx context.Context) any {
	id := strings.TrimSpace(middleware.GetReqID(ctx))
	if id == "" {
		return nil
	}
	return id
}

func auditPayload(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if sensitiveAuditKey(key) {
			continue
		}
		out[key] = value
	}
	return out
}

func sensitiveAuditKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "password_hash", "current_password", "new_password",
		"authorization", "cookie", "cookies", "user_agent", "headers",
		"body", "request_body", "response_body", "session_token", "raw_request", "query":
		return true
	default:
		return false
	}
}
