package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) listInterEntityMappings(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	v,err:=s.Store.ListInterEntityMappings(r.Context(),a.Entity.ID)
	if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":v})
}

func (s *Server) upsertInterEntityMapping(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canConfigureAccounting(a.Role),"inter-entity mapping configuration requires OWNER, ADMIN, or ACCOUNTANT"){return}
	counterparty,_,err:=s.Store.ResolveEntityAccess(r.Context(),a.User.ID,chi.URLParam(r,"counterparty"))
	if err!=nil{fail(w,403,err);return}
	var in map[string]string
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.UpsertInterEntityMapping(r.Context(),a.User,a.Entity,counterparty,in["due_from_account_id"],in["due_to_account_id"])
	if err!=nil{fail(w,400,err);return}
	write(w,200,v)
}

func (s *Server) createInterEntityExpense(w http.ResponseWriter,r *http.Request){
	u,err:=s.principal(r);if err!=nil{fail(w,401,err);return}
	var body struct{
		InitiatingEntityID string
		CounterpartyEntityID string
		Date string
		InitiatingFinancialAccountPublicID string
		InitiatingAmount string
		CounterpartyAmount string
		CounterpartyExpenseAccountPublicID string
		Description string
	}
	if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{fail(w,400,err);return}
	initEntity,initRole,err:=s.Store.ResolveEntityAccess(r.Context(),u.ID,body.InitiatingEntityID);if err!=nil{fail(w,403,err);return}
	cpEntity,cpRole,err:=s.Store.ResolveEntityAccess(r.Context(),u.ID,body.CounterpartyEntityID);if err!=nil{fail(w,403,err);return}
	if !canOperateLedger(initRole)||!canOperateLedger(cpRole){fail(w,403,errors.New("ledger operation access required on both entities"));return}
	v,err:=s.Store.PostInterEntityExpense(r.Context(),u,initEntity,cpEntity,postgres.InterEntityExpenseInput{
		Date:body.Date,
		InitiatingFinancialAccountPublicID:body.InitiatingFinancialAccountPublicID,
		InitiatingAmount:body.InitiatingAmount,
		CounterpartyAmount:body.CounterpartyAmount,
		CounterpartyExpenseAccountPublicID:body.CounterpartyExpenseAccountPublicID,
		Description:body.Description,
	})
	if err!=nil{fail(w,400,err);return}
	write(w,201,v)
}
