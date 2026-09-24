package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/auth"
	"github.com/profora/myankafe-finance/backend/internal/config"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

type Server struct {
	Store  *postgres.Store
	Config config.Config
}

func New(store *postgres.Store, cfg config.Config) http.Handler {
	s := &Server{Store: store, Config: cfg}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(cors(cfg.CORSOrigin))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusOK, map[string]any{"ok": true})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.AuthMode))
		r.Use(s.auditRequests)

		r.Get("/entities", s.listEntities)
		r.Post("/entities", s.createEntity)
		r.Get("/users", s.listUsers)
		r.Post("/users", s.createUser)
		r.Post("/inter-entity-transactions", s.createInterEntityExpense)
		r.Get("/dashboard/combined", s.combinedDashboard)

		r.Route("/entities/{entity}", func(r chi.Router) {
			r.Use(s.entityAccess)

			r.Get("/dashboard", s.dashboard)
			r.Get("/reports/profit-loss", s.profitLoss)
			r.Get("/reports/trial-balance", s.trialBalance)
			r.Get("/reports/balance-sheet", s.balanceSheet)
			r.Get("/reports/general-ledger", s.generalLedger)
			r.Get("/reports/account-ledger", s.accountLedger)
			r.Get("/reports/cash-movement", s.cashMovement)
			r.Get("/reports/inter-entity-balances", s.interEntityBalances)

			r.Get("/accounts", s.listAccounts)
			r.Post("/accounts", s.createAccount)

			r.Get("/financial-accounts", s.listFinancialAccounts)
			r.Post("/financial-accounts", s.createFinancialAccount)

			r.Get("/contacts", s.listContacts)
			r.Post("/contacts", s.createContact)

			r.Get("/exchange-rates", s.listExchangeRates)
			r.Post("/exchange-rates", s.createExchangeRate)

			r.Get("/transactions", s.listTransactions)
			r.Post("/transactions", s.createTransaction)
			r.Post("/transactions/{tx}/post", s.postTransaction)
			r.Post("/transactions/{tx}/reverse", s.reverseTransaction)

			r.Post("/transfers", s.createTransfer)
			r.Post("/manual-journals", s.manualJournal)

			r.Get("/inter-entity-mappings", s.listInterEntityMappings)
			r.Put("/inter-entity-mappings/{counterparty}", s.upsertInterEntityMapping)

			r.Get("/users", s.listEntityUsers)
			r.Put("/users/role", s.setUserRole)

			r.Get("/audit-events", s.listAuditEvents)

			r.Get("/accounting-lock", s.getLock)
			r.Post("/accounting-lock", s.lock)
			r.Post("/accounting-lock/unlock", s.unlock)
		})
	})

	return r
}

type accessKey struct{}

type access struct {
	User   postgres.User
	Entity postgres.Entity
	Role   string
}

func (s *Server) principal(r *http.Request) (postgres.User, error) {
	p, _ := auth.From(r.Context())
	return s.Store.ResolveUser(r.Context(), p.PublicID)
}

func (s *Server) entityAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.principal(r)
		if err != nil {
			fail(w, http.StatusUnauthorized, err)
			return
		}
		e, role, err := s.Store.ResolveEntityAccess(r.Context(), u.ID, chi.URLParam(r, "entity"))
		if err != nil {
			fail(w, http.StatusForbidden, err)
			return
		}
		ctx := context.WithValue(r.Context(), accessKey{}, access{User: u, Entity: e, Role: role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getAccess(r *http.Request) access {
	return r.Context().Value(accessKey{}).(access)
}

func (s *Server) listEntities(w http.ResponseWriter, r *http.Request) {
	u, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	v, err := s.Store.ListEntities(r.Context(), u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": v})
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	v, err := s.Store.ListAccounts(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": v})
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role == "VIEWER" {
		fail(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}
	var in postgres.CreateAccountInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.Store.CreateAccount(r.Context(), a.User, a.Entity, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, v)
}

func (s *Server) listFinancialAccounts(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	v, err := s.Store.ListFinancialAccounts(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": v})
}

func (s *Server) createFinancialAccount(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role == "VIEWER" {
		fail(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}
	var in postgres.CreateFinancialAccountInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.Store.CreateFinancialAccount(r.Context(), a.User, a.Entity, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, v)
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	v, err := s.Store.ListTransactions(r.Context(), a.Entity.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"items": v})
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role == "VIEWER" {
		fail(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}
	var in postgres.CreateTransactionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	v, err := s.Store.CreateTransaction(r.Context(), a.User, a.Entity, in)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, v)
}

func (s *Server) postTransaction(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role == "VIEWER" {
		fail(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}
	v, err := s.Store.PostTransaction(r.Context(), a.User, a.Entity, chi.URLParam(r, "tx"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (s *Server) getLock(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	v, err := s.Store.GetLock(r.Context(), a.Entity.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			write(w, http.StatusOK, map[string]any{"locked_through": nil})
			return
		}
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, v)
}

func (s *Server) lock(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	if a.Role != "OWNER" && a.Role != "ACCOUNTANT" {
		fail(w, http.StatusForbidden, errors.New("only OWNER or ACCOUNTANT can lock"))
		return
	}
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.Lock(r.Context(), a.User, a.Entity, in["locked_through"], in["reason"]); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) unlock(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	var in map[string]string
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.Unlock(r.Context(), a.User, a.Entity, a.Role, in["reason"]); err != nil {
		fail(w, http.StatusForbidden, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"ok": true})
}

func cors(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, err error) {
	write(w, status, map[string]any{"error": err.Error()})
}
