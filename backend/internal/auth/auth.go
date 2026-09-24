package auth

import (
	"context"
	"net/http"
	"strings"
)

type Principal struct {
	PublicID  string
	SessionID string
}

type key struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, key{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(key{}).(Principal)
	return p, ok
}

// Middleware is retained for explicit development/test token mode.
// Password/session authentication is wired by the HTTP server because it
// requires database-backed session resolution.
func Middleware(mode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mode != "dev" {
				http.Error(w, "development authentication disabled", http.StatusServiceUnavailable)
				return
			}
			h := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(h, "Bearer dev:") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			id := strings.TrimSpace(strings.TrimPrefix(h, "Bearer dev:"))
			if len(id) != 26 {
				http.Error(w, "invalid development principal", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), Principal{PublicID: id})))
		})
	}
}
