package httpapi

import (
	"net/http"
	"strconv"
)

func (s *Server) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, err := s.Store.ListAuditEvents(r.Context(), a.Entity.ID, limit)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": v})
}
