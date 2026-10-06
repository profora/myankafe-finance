package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "audit access requires OWNER, ADMIN, or ACCOUNTANT") {
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	outcome := q.Get("outcome")
	switch outcome {
	case "", "SUCCESS", "FAILED", "DENIED":
	default:
		fail(w, http.StatusBadRequest, errors.New("invalid audit outcome filter"))
		return
	}
	for _, key := range []string{"from", "to"} {
		if value := q.Get(key); value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				fail(w, http.StatusBadRequest, errors.New("invalid "+key+" date"))
				return
			}
		}
	}

	v, err := s.Store.ListAuditEvents(r.Context(), a.Entity.ID, postgres.AuditEventFilter{
		Search:  q.Get("q"),
		Action:  q.Get("action"),
		Outcome: outcome,
		From:    q.Get("from"),
		To:      q.Get("to"),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, v)
}
