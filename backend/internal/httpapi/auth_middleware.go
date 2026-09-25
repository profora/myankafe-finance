package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/auth"
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Config.AuthMode == "dev" {
			h := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(h, "Bearer dev:") {
				fail(w, http.StatusUnauthorized, errors.New("authentication required"))
				return
			}
			id := strings.TrimSpace(strings.TrimPrefix(h, "Bearer dev:"))
			if len(id) != 26 {
				fail(w, http.StatusUnauthorized, errors.New("invalid development principal"))
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{PublicID: id})))
			return
		}

		raw, ok := auth.SessionTokenFromRequest(r, s.Config.AuthCookieName)
		if !ok {
			s.clearSessionCookie(w)
			fail(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		hash, err := auth.HashSessionToken(raw)
		if err != nil {
			s.clearSessionCookie(w)
			fail(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		user, session, err := s.Store.ResolveSession(r.Context(), hash)
		if err != nil {
			s.clearSessionCookie(w)
			fail(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		if time.Since(session.LastSeenAt) >= 5*time.Minute {
			_ = s.Store.TouchSession(r.Context(), session.ID)
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{
			PublicID:        user.PublicID,
			SessionID:       session.ID,
			SessionPublicID: session.PublicID,
		})))
	})
}
