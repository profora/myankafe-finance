package httpapi

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.Store.Pool.Ping(ctx); err != nil {
		write(w, http.StatusServiceUnavailable, map[string]any{
			"ready":                         false,
			"database":                      "unavailable",
			"attachment_storage_configured": s.AttachmentStore != nil && s.AttachmentStore.Configured(),
		})
		return
	}

	write(w, http.StatusOK, map[string]any{
		"ready":                         true,
		"database":                      "ok",
		"attachment_storage_configured": s.AttachmentStore != nil && s.AttachmentStore.Configured(),
	})
}
