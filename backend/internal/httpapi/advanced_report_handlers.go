package httpapi

import (
	"net/http"
	"strconv"
	"time"
)

func reportRange(r *http.Request)(time.Time,time.Time,error){
	now:=time.Now().UTC()
	from,err:=parseDate(r.URL.Query().Get("from"),time.Date(now.Year(),1,1,0,0,0,0,time.UTC));if err!=nil{return time.Time{},time.Time{},err}
	to,err:=parseDate(r.URL.Query().Get("to"),now);if err!=nil{return time.Time{},time.Time{},err}
	return from,to,nil
}

func (s *Server) balanceSheet(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);through,err:=parseDate(r.URL.Query().Get("through"),time.Now().UTC());if err!=nil{fail(w,400,err);return}
	v,err:=s.Store.BalanceSheet(r.Context(),a.Entity.ID,through);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","BALANCE_SHEET",nil,"SUCCESS",map[string]any{"through":through.Format("2006-01-02")})
	write(w,200,v)
}

func (s *Server) generalLedger(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);from,to,err:=reportRange(r);if err!=nil{fail(w,400,err);return};limit,_:=strconv.Atoi(r.URL.Query().Get("limit"))
	v,err:=s.Store.GeneralLedger(r.Context(),a.Entity.ID,from,to,limit);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","GENERAL_LEDGER",nil,"SUCCESS",map[string]any{"from":from.Format("2006-01-02"),"to":to.Format("2006-01-02")})
	write(w,200,map[string]any{"items":v})
}

func (s *Server) accountLedger(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);from,to,err:=reportRange(r);if err!=nil{fail(w,400,err);return};limit,_:=strconv.Atoi(r.URL.Query().Get("limit"))
	account:=r.URL.Query().Get("account_id");if account==""{fail(w,400,errString("account_id is required"));return}
	v,err:=s.Store.AccountLedger(r.Context(),a.Entity.ID,account,from,to,limit);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","ACCOUNT_LEDGER",nil,"SUCCESS",map[string]any{"account_id":account})
	write(w,200,map[string]any{"items":v})
}

func (s *Server) cashMovement(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);from,to,err:=reportRange(r);if err!=nil{fail(w,400,err);return}
	v,err:=s.Store.CashMovement(r.Context(),a.Entity.ID,from,to);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","CASH_MOVEMENT",nil,"SUCCESS",nil)
	write(w,200,map[string]any{"items":v})
}

func (s *Server) interEntityBalances(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);through,err:=parseDate(r.URL.Query().Get("through"),time.Now().UTC());if err!=nil{fail(w,400,err);return}
	v,err:=s.Store.InterEntityBalances(r.Context(),a.Entity.ID,through);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","INTER_ENTITY_BALANCES",nil,"SUCCESS",nil)
	write(w,200,map[string]any{"items":v})
}

type stringError string
func (e stringError) Error()string{return string(e)}
func errString(v string)error{return stringError(v)}
