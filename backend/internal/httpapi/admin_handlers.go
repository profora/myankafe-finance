package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/auth"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) createEntity(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, 401, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if !isOwner && !u.PlatformOwner {
		fail(w, 403, errors.New("OWNER access required"))
		return
	}
	var in postgres.CreateEntityInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	v, err := s.Store.CreateEntity(r.Context(), u, in)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 201, v)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, 401, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if !isOwner {
		fail(w, 403, errors.New("OWNER access required"))
		return
	}
	v, err := s.Store.ListUsers(r.Context())
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, map[string]any{"items": v})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, 401, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if !isOwner {
		fail(w, 403, errors.New("OWNER access required"))
		return
	}
	var in struct {
		Username    string
		DisplayName string
		Email       string
		Password    string
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	username, err := auth.NormalizeUsername(in.Username)
	if err != nil {
		fail(w, 422, err)
		return
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		fail(w, 422, err)
		return
	}
	passwordHash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(w, 500, err)
		return
	}
	v, err := s.Store.CreateUser(r.Context(), u, postgres.CreateUserInput{
		Username: username, DisplayName: in.DisplayName, Email: in.Email,
	}, passwordHash)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 201, v)
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, 401, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if !isOwner {
		fail(w, 403, errors.New("OWNER access required"))
		return
	}
	var in struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	if err := auth.ValidatePassword(in.NewPassword); err != nil {
		fail(w, 422, err)
		return
	}
	hash, err := auth.HashPassword(in.NewPassword)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if err := s.Store.ResetUserPassword(r.Context(), u, chi.URLParam(r, "user"), hash); err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]any{"reset": true, "sessions_revoked": true})
}

func (s *Server) listEntityUsers(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role != "OWNER" && a.Role != "ADMIN" {
		fail(w, 403, errors.New("OWNER or ADMIN required"))
		return
	}
	v, err := s.Store.ListEntityUsers(r.Context(), a.Entity)
	if err != nil {
		fail(w, 500, err)
		return
	}
	write(w, 200, map[string]any{"items": v})
}

func (s *Server) setUserRole(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role != "OWNER" {
		fail(w, 403, errors.New("OWNER required"))
		return
	}
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	role := in["role"]
	switch role {
	case "OWNER", "ADMIN", "ACCOUNTANT", "BOOKKEEPER", "VIEWER":
	default:
		fail(w, 400, errors.New("invalid role"))
		return
	}
	v, err := s.Store.SetUserEntityRole(r.Context(), a.User, in["user_id"], a.Entity, role)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) setUserStatus(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, 401, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, u)
	if err != nil {
		fail(w, 500, err)
		return
	}
	if !isOwner {
		fail(w, 403, errors.New("OWNER access required"))
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	if err := s.Store.SetUserStatus(r.Context(), u, chi.URLParam(r, "user"), in.Status); err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, map[string]any{"user_id": chi.URLParam(r, "user"), "status": in.Status})
}

func (s *Server) updateEntitySettings(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role != "OWNER" && a.Role != "ADMIN" {
		fail(w, 403, errors.New("OWNER or ADMIN required"))
		return
	}
	var in postgres.UpdateEntitySettingsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, 400, err)
		return
	}
	v, err := s.Store.UpdateEntitySettings(r.Context(), a.User, a.Entity, in)
	if err != nil {
		fail(w, 400, err)
		return
	}
	write(w, 200, v)
}

func (s *Server) revokeUserEntityAccess(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role != "OWNER" {
		fail(w, 403, errors.New("OWNER required"))
		return
	}
	if err := s.Store.RevokeUserEntityAccess(r.Context(), a.User, chi.URLParam(r, "user"), a.Entity); err != nil {
		fail(w, 400, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
