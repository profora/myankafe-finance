package httpapi

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) login(w http.ResponseWriter,r *http.Request){
	if s.Config.AuthMode!="password"{
		fail(w,http.StatusServiceUnavailable,errors.New("password authentication is not enabled"))
		return
	}
	var in loginRequest
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	username,usernameErr:=auth.NormalizeUsername(in.Username)
	ip:=clientIP(r)
	now:=time.Now().UTC()
	usernameKey:="username:"+username
	if usernameErr!=nil{usernameKey="username:invalid"}
	if !s.LoginLimiter.Allow("ip:"+ip,20,now)||!s.LoginLimiter.Allow(usernameKey,10,now){
		_ = s.Store.AuditAuth(r.Context(),nil,"AUTH_LOGIN_RATE_LIMITED","DENIED",map[string]any{"username":username,"ip":ip})
		fail(w,http.StatusTooManyRequests,errors.New("too many login attempts; try again later"))
		return
	}

	passwordHash:=s.DummyPasswordHash
	var userID *string
	var credentialUserID string
	cred,err:=s.Store.CredentialByUsername(r.Context(),username)
	found:=err==nil
	if found{
		passwordHash=cred.PasswordHash
		credentialUserID=cred.User.ID
		userID=&credentialUserID
	}else if !errors.Is(err,pgx.ErrNoRows){
		fail(w,500,errors.New("could not process login"))
		return
	}

	match,needsRehash,verifyErr:=auth.VerifyPassword(in.Password,passwordHash)
	if verifyErr!=nil{
		fail(w,500,errors.New("could not process login"))
		return
	}
	if usernameErr!=nil||!found||!match{
		_ = s.Store.AuditAuth(r.Context(),userID,"AUTH_LOGIN_FAILED","DENIED",map[string]any{"username":username,"ip":ip})
		fail(w,http.StatusUnauthorized,errors.New("invalid username or password"))
		return
	}

	if needsRehash{
		if hash,err:=auth.HashPassword(in.Password);err==nil{
			_ = s.Store.UpsertPasswordHash(r.Context(),cred.User.ID,hash)
		}
	}

	rawToken,tokenHash,err:=auth.NewSessionToken()
	if err!=nil{fail(w,500,errors.New("could not create session"));return}
	expires:=now.Add(s.Config.AuthSessionTTL)
	session,err:=s.Store.CreateSession(r.Context(),cred.User.ID,tokenHash,r.UserAgent(),ip,expires)
	if err!=nil{fail(w,500,errors.New("could not create session"));return}

	_ = s.Store.AuditAuth(r.Context(),&cred.User.ID,"AUTH_LOGIN","SUCCESS",map[string]any{"session_id":session.PublicID,"ip":ip})
	s.LoginLimiter.Reset(usernameKey)
	s.LoginLimiter.Reset("ip:"+ip)
	s.setSessionCookie(w,rawToken,session.ExpiresAt)
	w.Header().Set("Cache-Control","no-store")
	response:=map[string]any{
		"user":map[string]any{
			"public_id":cred.User.PublicID,
			"username":cred.User.Username,
			"display_name":cred.User.DisplayName,
		},
		"session_expires_at":session.ExpiresAt,
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Session-Transport")),"bearer"){
		response["session_token"]=rawToken
	}
	write(w,200,response)
}

func (s *Server) logout(w http.ResponseWriter,r *http.Request){
	if raw,ok:=auth.SessionTokenFromRequest(r,s.Config.AuthCookieName);ok{
		if hash,err:=auth.HashSessionToken(raw);err==nil{
			_ = s.Store.RevokeSession(r.Context(),hash)
		}
	}
	if p,ok:=auth.From(r.Context());ok{
		if u,err:=s.Store.ResolveUser(r.Context(),p.PublicID);err==nil{
			_ = s.Store.AuditAuth(r.Context(),&u.ID,"AUTH_LOGOUT","SUCCESS",map[string]any{"session_id":p.SessionPublicID})
		}
	}
	s.clearSessionCookie(w)
	w.Header().Set("Cache-Control","no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.From(r.Context())
	if !ok{fail(w,401,errors.New("authentication required"));return}
	u,err:=s.Store.ResolveUser(r.Context(),p.PublicID)
	if err!=nil{fail(w,401,errors.New("authentication required"));return}
	w.Header().Set("Cache-Control","no-store")
	write(w,200,map[string]any{"user":map[string]any{
		"public_id":u.PublicID,
		"username":u.Username,
		"display_name":u.DisplayName,
	}})
}

func (s *Server) changePassword(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.From(r.Context());if !ok{fail(w,401,errors.New("authentication required"));return}
	u,err:=s.Store.ResolveUser(r.Context(),p.PublicID);if err!=nil{fail(w,401,errors.New("authentication required"));return}
	var in changePasswordRequest
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	if err:=auth.ValidatePassword(in.NewPassword);err!=nil{fail(w,422,err);return}
	currentHash,err:=s.Store.CredentialByUserID(r.Context(),u.ID);if err!=nil{fail(w,500,errors.New("could not load credentials"));return}
	match,_,err:=auth.VerifyPassword(in.CurrentPassword,currentHash)
	if err!=nil||!match{fail(w,401,errors.New("current password is incorrect"));return}
	newHash,err:=auth.HashPassword(in.NewPassword);if err!=nil{fail(w,422,err);return}
	rawToken,tokenHash,err:=auth.NewSessionToken();if err!=nil{fail(w,500,err);return}
	session,err:=s.Store.ChangePasswordAndReplaceSessions(
		r.Context(),u,newHash,tokenHash,r.UserAgent(),clientIP(r),time.Now().UTC().Add(s.Config.AuthSessionTTL),
	)
	if err!=nil{fail(w,500,errors.New("could not change password"));return}
	s.setSessionCookie(w,rawToken,session.ExpiresAt)
	write(w,200,map[string]any{"changed":true,"session_expires_at":session.ExpiresAt})
}

func (s *Server) setSessionCookie(w http.ResponseWriter,token string,expires time.Time){
	maxAge:=int(time.Until(expires).Seconds());if maxAge<1{maxAge=1}
	http.SetCookie(w,&http.Cookie{
		Name:s.Config.AuthCookieName,Value:token,Path:"/",Expires:expires,MaxAge:maxAge,
		HttpOnly:true,Secure:s.Config.AuthCookieSecure,SameSite:http.SameSiteStrictMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter){
	http.SetCookie(w,&http.Cookie{
		Name:s.Config.AuthCookieName,Value:"",Path:"/",Expires:time.Unix(1,0).UTC(),MaxAge:-1,
		HttpOnly:true,Secure:s.Config.AuthCookieSecure,SameSite:http.SameSiteStrictMode,
	})
}

func clientIP(r *http.Request) string {
	v:=strings.TrimSpace(r.RemoteAddr)
	if host,_,err:=net.SplitHostPort(v);err==nil{return host}
	return v
}


func (s *Server) listSessions(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.From(r.Context());if !ok{fail(w,401,errors.New("authentication required"));return}
	u,err:=s.Store.ResolveUser(r.Context(),p.PublicID);if err!=nil{fail(w,401,errors.New("authentication required"));return}
	items,err:=s.Store.ListActiveSessions(r.Context(),u.ID);if err!=nil{fail(w,500,err);return}
	out:=make([]map[string]any,0,len(items))
	for _,item:=range items{
		out=append(out,map[string]any{
			"id":item.PublicID,
			"user_agent":item.UserAgent,
			"ip_address":item.IPAddress,
			"expires_at":item.ExpiresAt,
			"last_seen_at":item.LastSeenAt,
			"created_at":item.CreatedAt,
			"current":item.InternalID==p.SessionID,
		})
	}
	write(w,200,map[string]any{"items":out})
}

func (s *Server) revokeSession(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.From(r.Context());if !ok{fail(w,401,errors.New("authentication required"));return}
	u,err:=s.Store.ResolveUser(r.Context(),p.PublicID);if err!=nil{fail(w,401,errors.New("authentication required"));return}
	internalID,err:=s.Store.RevokeSessionByPublicID(r.Context(),u.ID,chi.URLParam(r,"session"))
	if err!=nil{fail(w,404,errors.New("session not found"));return}
	_ = s.Store.AuditAuth(r.Context(),&u.ID,"AUTH_SESSION_REVOKED","SUCCESS",map[string]any{"session_id":chi.URLParam(r,"session")})
	if internalID==p.SessionID{
		s.clearSessionCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) revokeOtherSessions(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.From(r.Context());if !ok{fail(w,401,errors.New("authentication required"));return}
	u,err:=s.Store.ResolveUser(r.Context(),p.PublicID);if err!=nil{fail(w,401,errors.New("authentication required"));return}
	if p.SessionID==""{fail(w,400,errors.New("current session cannot be identified in development auth mode"));return}
	count,err:=s.Store.RevokeOtherSessions(r.Context(),u.ID,p.SessionID);if err!=nil{fail(w,500,err);return}
	_ = s.Store.AuditAuth(r.Context(),&u.ID,"AUTH_OTHER_SESSIONS_REVOKED","SUCCESS",map[string]any{"count":count})
	write(w,200,map[string]any{"revoked":count})
}
