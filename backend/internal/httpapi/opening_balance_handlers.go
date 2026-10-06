package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) setAccountingStartDate(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "accounting start date requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	var body struct {
		AccountingStartDate *string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	a.Entity.Role = a.Role
	if err := s.Store.SetAccountingStartDate(r.Context(), a.User, a.Entity, body.AccountingStartDate); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"accounting_start_date": body.AccountingStartDate})
}

func (s *Server) getOpeningBalances(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	a.Entity.Role = a.Role
	v, err := s.Store.GetOpeningBalances(r.Context(), a.Entity)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (s *Server) previewOpeningBalances(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canEditOpeningBalances(a.Role), "opening balances require OWNER or ACCOUNTANT") {
		return
	}
	var body struct {
		Lines []postgres.OpeningBalanceLineInput
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	a.Entity.Role = a.Role
	v, err := s.Store.PreviewOpeningBalances(r.Context(), a.Entity, body.Lines)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (s *Server) saveOpeningBalances(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canEditOpeningBalances(a.Role), "opening balances require OWNER or ACCOUNTANT") {
		return
	}
	var body struct {
		Lines []postgres.OpeningBalanceLineInput
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	a.Entity.Role = a.Role
	v, err := s.Store.SaveOpeningBalances(r.Context(), a.User, a.Entity, body.Lines)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, v)
}
