package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/auth"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (s *Server) auditDeniedAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status != http.StatusForbidden {
			return
		}

		p, ok := auth.From(r.Context())
		if !ok {
			return
		}
		u, err := s.Store.ResolveUser(r.Context(), p.PublicID)
		if err != nil {
			return
		}

		var entityPtr *postgres.Entity
		entityPublicID := chi.URLParam(r, "entity")
		if entityPublicID != "" {
			if e, _, err := s.Store.ResolveEntityAccess(r.Context(), u.ID, entityPublicID); err == nil {
				entityPtr = &e
			}
		}
		route := ""
		if rc := chi.RouteContext(r.Context()); rc != nil {
			route = rc.RoutePattern()
		}
		_ = s.Store.Audit(r.Context(), u, entityPtr, "ACCESS_DENIED", "API", nil, "DENIED", map[string]any{
			"method": r.Method,
			"route":  route,
		})
	})
}
