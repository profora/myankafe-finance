package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) reverseTransaction(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canCorrectPostedAccounting(a.Role), "posted accounting correction requires OWNER or ACCOUNTANT") {
		return
	}
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	v, err := s.Store.ReverseTransaction(r.Context(), a.User, a.Entity, chi.URLParam(r, "tx"), in["reversal_date"], in["reason"])
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 201, v)
}
