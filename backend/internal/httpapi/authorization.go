package httpapi

import (
	"errors"
	"net/http"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func canConfigureAccounting(role string) bool {
	switch role {
	case "OWNER", "ADMIN", "ACCOUNTANT":
		return true
	default:
		return false
	}
}

func canManageInterEntitySetup(role string) bool {
	return role == "OWNER" || role == "ADMIN"
}

func canOperateLedger(role string) bool {
	switch role {
	case "OWNER", "ADMIN", "ACCOUNTANT", "BOOKKEEPER":
		return true
	default:
		return false
	}
}

func canCorrectPostedAccounting(role string) bool {
	return role == "OWNER" || role == "ACCOUNTANT"
}

func canEditOpeningBalances(role string) bool {
	return role == "OWNER" || role == "ACCOUNTANT"
}

func (s *Server) ownerAnywhere(r *http.Request, user postgres.User) (bool, error) {
	if user.PlatformOwner {
		return true, nil
	}
	entities, err := s.Store.ListEntities(r.Context(), user.ID)
	if err != nil {
		return false, err
	}
	for _, e := range entities {
		_, role, err := s.Store.ResolveEntityAccess(r.Context(), user.ID, e.PublicID)
		if err == nil && role == "OWNER" {
			return true, nil
		}
	}
	return false, nil
}

func requireRole(w http.ResponseWriter, ok bool, message string) bool {
	if ok {
		return true
	}
	fail(w, http.StatusForbidden, errors.New(message))
	return false
}
