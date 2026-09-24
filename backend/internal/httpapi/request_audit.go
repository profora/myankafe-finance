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

func (s *Server) auditRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

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

		outcome := "SUCCESS"
		if rec.status >= 400 && rec.status < 500 {
			outcome = "DENIED"
		}
		if rec.status >= 500 {
			outcome = "FAILED"
		}

		_ = s.Store.Audit(
			r.Context(),
			u,
			entityPtr,
			"HTTP_REQUEST",
			"API",
			nil,
			outcome,
			map[string]any{
				"method": r.Method,
				"path": r.URL.Path,
				"status": rec.status,
			},
		)
	})
}
