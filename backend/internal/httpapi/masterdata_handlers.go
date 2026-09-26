package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) listCurrencies(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	activeOnly := r.URL.Query().Get("active") == "1" || strings.EqualFold(r.URL.Query().Get("active"), "true")
	if !activeOnly {
		isOwner, err := s.ownerAnywhere(r, u)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		if !isOwner {
			fail(w, http.StatusForbidden, errors.New("OWNER access required"))
			return
		}
	}
	items, err := s.Store.ListCurrencies(r.Context(), activeOnly)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) createCurrency(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !isOwner {
		fail(w, http.StatusForbidden, errors.New("OWNER access required"))
		return
	}
	var in postgres.CurrencyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.Store.CreateCurrency(r.Context(), u, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, item)
}

func (s *Server) updateCurrency(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !isOwner {
		fail(w, http.StatusForbidden, errors.New("OWNER access required"))
		return
	}
	var in postgres.CurrencyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.Store.UpdateCurrency(r.Context(), u, chi.URLParam(r, "code"), in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, item)
}

func (s *Server) deleteCurrency(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !isOwner {
		fail(w, http.StatusForbidden, errors.New("OWNER access required"))
		return
	}
	if err := s.Store.DeleteCurrency(r.Context(), u, chi.URLParam(r, "code")); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listContactTypes(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	items, err := s.Store.ListContactTypes(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) createContactType(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "contact type configuration requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	var in postgres.ContactTypeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.Store.CreateContactType(r.Context(), a.User, a.Entity, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, item)
}

func (s *Server) updateContactType(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "contact type configuration requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	var in postgres.ContactTypeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.Store.UpdateContactType(r.Context(), a.User, a.Entity, chi.URLParam(r, "code"), in.Name, in.Active)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, item)
}

func (s *Server) deleteContactType(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "contact type configuration requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	if err := s.Store.DeleteContactType(r.Context(), a.User, a.Entity, chi.URLParam(r, "code")); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAuditActions(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "audit access requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	items, err := s.Store.ListAuditActions(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": items})
}
