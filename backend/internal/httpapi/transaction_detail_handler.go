package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) transactionDetail(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	v, err := s.Store.TransactionDetail(r.Context(), a.Entity.ID, chi.URLParam(r, "tx"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	_ = s.Store.Audit(r.Context(), a.User, &a.Entity, "TRANSACTION_VIEW", "TRANSACTION", nil, "SUCCESS", map[string]any{"transaction_id": chi.URLParam(r, "tx")})
	write(w, http.StatusOK, v)
}
