package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func parseDate(v string, fallback time.Time) (time.Time, error) {
	if v == "" {
		return fallback, nil
	}
	return time.Parse("2006-01-02", v)
}

func fiscalYearStart(e postgres.Entity, now time.Time) time.Time {
	loc, err := time.LoadLocation(e.Timezone)
	if err != nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	start := time.Date(localNow.Year(), time.Month(e.FiscalMonth), e.FiscalDay, 0, 0, 0, 0, loc)
	if localNow.Before(start) {
		start = time.Date(localNow.Year()-1, time.Month(e.FiscalMonth), e.FiscalDay, 0, 0, 0, 0, loc)
	}
	return start
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := now.UTC()
	v, err := s.Store.Dashboard(r.Context(), a.Entity.ID, from, to)
	if err != nil { fail(w, http.StatusInternalServerError, err); return }
	_ = s.Store.Audit(r.Context(), a.User, &a.Entity, "DASHBOARD_VIEW", "REPORT", nil, "SUCCESS", map[string]any{"from":from.Format("2006-01-02"),"to":to.Format("2006-01-02")})
	write(w, http.StatusOK, v)
}

func (s *Server) profitLoss(w http.ResponseWriter, r *http.Request) {
	a:=getAccess(r)
	now:=time.Now().UTC()
	from,err:=parseDate(r.URL.Query().Get("from"),fiscalYearStart(a.Entity,now));if err!=nil{fail(w,400,err);return}
	to,err:=parseDate(r.URL.Query().Get("to"),now);if err!=nil{fail(w,400,err);return}
	v,err:=s.Store.ProfitLoss(r.Context(),a.Entity.ID,from,to);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","PROFIT_LOSS",nil,"SUCCESS",map[string]any{"from":from.Format("2006-01-02"),"to":to.Format("2006-01-02")})
	write(w,200,map[string]any{"items":v,"from":from.Format("2006-01-02"),"to":to.Format("2006-01-02")})
}

func (s *Server) trialBalance(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);through,err:=parseDate(r.URL.Query().Get("through"),time.Now().UTC());if err!=nil{fail(w,400,err);return}
	v,err:=s.Store.TrialBalance(r.Context(),a.Entity.ID,through);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"REPORT_VIEW","TRIAL_BALANCE",nil,"SUCCESS",map[string]any{"through":through.Format("2006-01-02")})
	write(w,200,map[string]any{"items":v,"through":through.Format("2006-01-02")})
}

func (s *Server) listExchangeRates(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);v,err:=s.Store.ListExchangeRates(r.Context(),a.Entity.ID);if err!=nil{fail(w,500,err);return}
	_=s.Store.Audit(r.Context(),a.User,&a.Entity,"EXCHANGE_RATE_LIST","EXCHANGE_RATE",nil,"SUCCESS",nil)
	write(w,200,map[string]any{"items":v})
}

func (s *Server) createExchangeRate(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);if !requireRole(w,canConfigureAccounting(a.Role),"exchange-rate configuration requires OWNER, ADMIN, or ACCOUNTANT"){return}
	var in map[string]string;if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.CreateExchangeRate(r.Context(),a.User,a.Entity,in["rate_date"],in["from_currency"],in["to_currency"],in["rate"],in["source"],in["source_reference"])
	if err!=nil{fail(w,400,err);return};write(w,201,v)
}

func (s *Server) manualJournal(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r);if a.Role!="OWNER"&&a.Role!="ACCOUNTANT"{fail(w,403,errors.New("manual journal requires OWNER or ACCOUNTANT"));return}
	var in postgres.ManualJournalInput;if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.PostManualJournal(r.Context(),a.User,a.Entity,in);if err!=nil{fail(w,400,err);return};write(w,201,v)
}


func (s *Server) updateExchangeRate(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canConfigureAccounting(a.Role),"exchange-rate configuration requires OWNER, ADMIN, or ACCOUNTANT"){return}
	var in postgres.UpdateExchangeRateInput
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.UpdateExchangeRate(r.Context(),a.User,a.Entity,chi.URLParam(r,"rate"),in)
	if err!=nil{fail(w,400,err);return}
	write(w,200,v)
}

func (s *Server) deleteExchangeRate(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if !requireRole(w,canConfigureAccounting(a.Role),"exchange-rate configuration requires OWNER, ADMIN, or ACCOUNTANT"){return}
	if err:=s.Store.DeleteExchangeRate(r.Context(),a.User,a.Entity,chi.URLParam(r,"rate"));err!=nil{
		fail(w,400,err);return
	}
	w.WriteHeader(http.StatusNoContent)
}
