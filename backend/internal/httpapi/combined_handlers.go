package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) combinedDashboard(w http.ResponseWriter,r *http.Request){
	u,err:=s.principal(r);if err!=nil{fail(w,401,err);return}
	now:=time.Now().UTC()
	from,err:=parseDate(r.URL.Query().Get("from"),time.Date(now.Year(),now.Month(),1,0,0,0,0,time.UTC));if err!=nil{fail(w,400,err);return}
	to,err:=parseDate(r.URL.Query().Get("to"),now);if err!=nil{fail(w,400,err);return}
	currency:=r.URL.Query().Get("currency");if currency==""{currency="MMK"}
	v,err:=s.Store.CombinedDashboard(r.Context(),u.ID,currency,from,to);if err!=nil{fail(w,400,err);return}
	_=s.Store.Audit(r.Context(),u,nil,"COMBINED_DASHBOARD_VIEW","REPORT",nil,"SUCCESS",map[string]any{"currency":currency,"from":from.Format("2006-01-02"),"to":to.Format("2006-01-02")})
	write(w,200,v)
}
