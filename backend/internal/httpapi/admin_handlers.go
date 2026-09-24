package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) createEntity(w http.ResponseWriter,r *http.Request){
	u,err:=s.principal(r);if err!=nil{fail(w,401,err);return}
	isOwner,err:=s.ownerAnywhere(r,u);if err!=nil{fail(w,500,err);return}
	if !isOwner{fail(w,403,errors.New("OWNER access required"));return}
	var in postgres.CreateEntityInput
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.CreateEntity(r.Context(),u,in);if err!=nil{fail(w,400,err);return}
	write(w,201,v)
}

func (s *Server) listUsers(w http.ResponseWriter,r *http.Request){
	u,err:=s.principal(r);if err!=nil{fail(w,401,err);return}
	isOwner,err:=s.ownerAnywhere(r,u);if err!=nil{fail(w,500,err);return}
	if !isOwner{fail(w,403,errors.New("OWNER access required"));return}
	v,err:=s.Store.ListUsers(r.Context());if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":v})
}

func (s *Server) createUser(w http.ResponseWriter,r *http.Request){
	u,err:=s.principal(r);if err!=nil{fail(w,401,err);return}
	isOwner,err:=s.ownerAnywhere(r,u);if err!=nil{fail(w,500,err);return}
	if !isOwner{fail(w,403,errors.New("OWNER access required"));return}
	var in postgres.CreateUserInput
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	v,err:=s.Store.CreateUser(r.Context(),u,in);if err!=nil{fail(w,400,err);return}
	write(w,201,v)
}

func (s *Server) listEntityUsers(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if a.Role!="OWNER"&&a.Role!="ADMIN"{fail(w,403,errors.New("OWNER or ADMIN required"));return}
	v,err:=s.Store.ListEntityUsers(r.Context(),a.Entity);if err!=nil{fail(w,500,err);return}
	write(w,200,map[string]any{"items":v})
}

func (s *Server) setUserRole(w http.ResponseWriter,r *http.Request){
	a:=getAccess(r)
	if a.Role!="OWNER"{fail(w,403,errors.New("OWNER required"));return}
	var in map[string]string
	if err:=json.NewDecoder(r.Body).Decode(&in);err!=nil{fail(w,400,err);return}
	role:=in["role"]
	switch role{case "OWNER","ADMIN","ACCOUNTANT","BOOKKEEPER","VIEWER":default:fail(w,400,errors.New("invalid role"));return}
	v,err:=s.Store.SetUserEntityRole(r.Context(),a.User,in["user_id"],a.Entity,role);if err!=nil{fail(w,400,err);return}
	write(w,200,v)
}
