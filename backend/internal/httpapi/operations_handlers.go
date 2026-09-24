package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) listContacts(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	v,err:=s.Store.ListContacts(r.Context(),a.Entity.ID)
	if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":v})
}

func (s *Server) createContact(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canOperateLedger(a.Role),"ledger operation access required"){return}
	var in map[string]string
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.CreateContact(r.Context(),a.User,a.Entity,in["contact_type"],in["display_name"],in["phone"],in["email"],in["notes"])
	if err!=nil{fail(w,400,err);return}
	write(w,201,v)
}

func (s *Server) createTransfer(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canOperateLedger(a.Role),"ledger operation access required"){return}
	var in postgres.TransferInput
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.PostTransfer(r.Context(),a.User,a.Entity,in)
	if err!=nil{fail(w,400,err);return}
	write(w,201,v)
}
