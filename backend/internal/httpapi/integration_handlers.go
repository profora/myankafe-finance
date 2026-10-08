package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/integration"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

type integrationConnKey struct{}

func (s *Server) requireIntegrationAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		system := strings.TrimSpace(r.Header.Get(integration.HeaderSystem))
		if system != integration.SystemMyanKafePlatform {
			integrationFail(w, &integration.Error{Status: 401, Code: "unknown_system", Message: "unknown integration system"})
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			integrationFail(w, integration.Validation("could not read request body"))
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		conn, currency, lookupErr := s.Store.ActiveIntegrationConnection(r.Context(), system)
		secret := s.Config.IntegrationSecretMyanKafePlatform
		if secret == "" || conn.SecretReference == "" {
			var ie *integration.Error
			if errors.As(lookupErr, &ie) && ie.Code == "unknown_system" {
				integrationFail(w, ie)
				return
			}
			integrationFail(w, &integration.Error{Status: 401, Code: "secret_not_configured", Message: "integration secret is not configured"})
			return
		}
		if err := integration.VerifySignature(secret, r.Method, r.URL.Path, r.Header.Get(integration.HeaderTimestamp), body, r.Header.Get(integration.HeaderSignature)); err != nil {
			integrationFail(w, &integration.Error{Status: 401, Code: "bad_signature", Message: "integration signature was rejected"})
			return
		}
		if err := integration.VerifyTimestamp(r.Header.Get(integration.HeaderTimestamp), time.Now()); err != nil {
			code := err.Error()
			integrationFail(w, &integration.Error{Status: 401, Code: code, Message: "integration timestamp was rejected"})
			return
		}
		if lookupErr != nil {
			var ie *integration.Error
			if errors.As(lookupErr, &ie) {
				integrationFail(w, ie)
				return
			}
			fail(w, http.StatusInternalServerError, lookupErr)
			return
		}
		eventID := strings.TrimSpace(r.Header.Get(integration.HeaderEventID))
		switch {
		case r.URL.Path == "/api/v1/integrations/readiness":
			if eventID != integration.ReadinessEventID {
				integrationFail(w, &integration.Error{Status: 401, Code: "event_id_mismatch", Message: "event id does not match the request"})
				return
			}
		case r.Method == http.MethodGet:
			if eventID == "" || eventID != chi.URLParam(r, "externalEventID") {
				integrationFail(w, &integration.Error{Status: 401, Code: "event_id_mismatch", Message: "event id does not match the request"})
				return
			}
		default:
			env, err := integration.ParseEnvelope(body)
			if err != nil {
				integrationFail(w, err)
				return
			}
			if eventID == "" || eventID != env.ExternalEventID {
				integrationFail(w, &integration.Error{Status: 401, Code: "event_id_mismatch", Message: "event id does not match the request"})
				return
			}
		}
		ctx := context.WithValue(r.Context(), integrationConnKey{}, integrationPrincipal{conn: conn, currency: currency, body: body})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type integrationPrincipal struct {
	conn     postgres.IntegrationConnection
	currency string
	body     []byte
}

func integrationPrincipalFrom(r *http.Request) integrationPrincipal {
	v, _ := r.Context().Value(integrationConnKey{}).(integrationPrincipal)
	return v
}

func (s *Server) previewIntegrationEvent(w http.ResponseWriter, r *http.Request) {
	p := integrationPrincipalFrom(r)
	result, err := s.Store.PreviewIntegrationEvent(r.Context(), p.conn, p.currency, p.body)
	if err != nil {
		integrationFail(w, err)
		return
	}
	write(w, http.StatusOK, integrationResponse(result))
}

func (s *Server) postIntegrationEvent(w http.ResponseWriter, r *http.Request) {
	p := integrationPrincipalFrom(r)
	result, err := s.Store.PostIntegrationEvent(r.Context(), p.conn, p.currency, p.body)
	if err != nil {
		integrationFail(w, err)
		return
	}
	write(w, http.StatusOK, integrationResponse(result))
}

func (s *Server) getMachineIntegrationEvent(w http.ResponseWriter, r *http.Request) {
	p := integrationPrincipalFrom(r)
	ev, err := s.Store.GetIntegrationEvent(r.Context(), p.conn, chi.URLParam(r, "externalEventID"))
	if err != nil {
		integrationFail(w, err)
		return
	}
	write(w, http.StatusOK, ev)
}

func (s *Server) machineIntegrationReadiness(w http.ResponseWriter, r *http.Request) {
	p := integrationPrincipalFrom(r)
	ready, err := s.Store.IntegrationReadiness(r.Context(), p.conn.EntityID, s.Config.IntegrationSecretMyanKafePlatform != "")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, ready)
}

func (s *Server) listIntegrationConnections(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	items, err := s.Store.ListIntegrationConnections(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []postgres.IntegrationConnection{}
	}
	write(w, http.StatusOK, map[string]any{"items": items, "secret_configured": s.Config.IntegrationSecretMyanKafePlatform != ""})
}

func (s *Server) createIntegrationConnection(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canManageInterEntitySetup(a.Role), "activating an integration requires OWNER or ADMIN") {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		fail(w, http.StatusBadRequest, err)
		return
	}
	conn, err := s.Store.CreateIntegrationConnection(r.Context(), a.User, a.Entity, body.Name)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, conn)
}

func (s *Server) updateIntegrationConnection(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canManageInterEntitySetup(a.Role), "activating an integration requires OWNER or ADMIN") {
		return
	}
	var body struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	conn, err := s.Store.UpdateIntegrationConnection(r.Context(), a.User, a.Entity, chi.URLParam(r, "system"), body.Name, body.Active)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, conn)
}

func (s *Server) listIntegrationMappings(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	items, err := s.Store.ListIntegrationMappings(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []postgres.IntegrationMappingView{}
	}
	write(w, http.StatusOK, map[string]any{"items": items, "suggested_keys": integration.StructuralKeys()})
}

func (s *Server) putIntegrationMapping(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if !requireRole(w, canConfigureAccounting(a.Role), "integration mappings require OWNER, ADMIN, or ACCOUNTANT") {
		return
	}
	var body postgres.IntegrationMappingInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	body.SourceKey = chi.URLParam(r, "sourceKey")
	if err := s.Store.PutIntegrationMapping(r.Context(), a.User, a.Entity, body); err != nil {
		integrationFail(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"source_key": body.SourceKey, "saved": true})
}

func (s *Server) listIntegrationEvents(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	items, err := s.Store.ListIntegrationEvents(r.Context(), a.Entity.ID, 50)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []postgres.IntegrationEventView{}
	}
	write(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) entityIntegrationReadiness(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	ready, err := s.Store.IntegrationReadiness(r.Context(), a.Entity.ID, s.Config.IntegrationSecretMyanKafePlatform != "")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, ready)
}

func integrationResponse(result postgres.IntegrationResult) map[string]any {
	lines := make([]map[string]any, 0, len(result.Lines))
	for _, line := range result.Lines {
		lines = append(lines, map[string]any{
			"source_key": line.SourceKey, "memo": line.Memo, "debit_mmk": line.Debit, "credit_mmk": line.Credit,
		})
	}
	return map[string]any{
		"status": result.Status, "event_public_id": result.EventPublicID,
		"transaction_public_id": result.TransactionPublicID, "replayed": result.Replayed, "lines": lines,
	}
}

func integrationFail(w http.ResponseWriter, err error) {
	var ie *integration.Error
	if errors.As(err, &ie) {
		write(w, ie.Status, map[string]any{"error": ie.Message, "code": ie.Code})
		return
	}
	fail(w, http.StatusBadRequest, err)
}
