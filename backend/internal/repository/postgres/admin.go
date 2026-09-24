package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type CreateEntityInput struct {
	Code               string
	Name               string
	EntityType         string
	FunctionalCurrency string
	Timezone           string
	FiscalMonth        int
	FiscalDay          int
}

func (s *Store) CreateEntity(ctx context.Context, user User, in CreateEntityInput) (Entity, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	if in.Code == "" || strings.TrimSpace(in.Name) == "" {
		return Entity{}, fmt.Errorf("code and name are required")
	}
	if in.EntityType == "" { in.EntityType = "BUSINESS" }
	if in.FunctionalCurrency == "" { in.FunctionalCurrency = "MMK" }
	if in.Timezone == "" { in.Timezone = "Asia/Yangon" }
	if in.FiscalMonth == 0 { in.FiscalMonth = 1 }
	if in.FiscalDay == 0 { in.FiscalDay = 1 }

	tx, err := s.Pool.Begin(ctx)
	if err != nil { return Entity{}, err }
	defer tx.Rollback(ctx)

	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	_, err = tx.Exec(ctx, `INSERT INTO entities(
id,public_id,code,name,entity_type,functional_currency_code,timezone,
fiscal_year_start_month,fiscal_year_start_day,created_by
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id,pub,in.Code,in.Name,in.EntityType,in.FunctionalCurrency,in.Timezone,in.FiscalMonth,in.FiscalDay,user.ID)
	if err != nil { return Entity{}, err }

	linkID, _ := ids.UUIDv7()
	_, err = tx.Exec(ctx, `INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$2)`, linkID,user.ID,id)
	if err != nil { return Entity{}, err }

	e := Entity{ID:id,PublicID:pub,Code:in.Code,Name:in.Name,Type:in.EntityType,FunctionalCurrency:in.FunctionalCurrency,Timezone:in.Timezone,FiscalMonth:in.FiscalMonth,FiscalDay:in.FiscalDay}
	if err:=insertAuditTx(ctx,tx,user,e,"ENTITY_CREATE","ENTITY",pub,map[string]any{"code":in.Code,"name":in.Name});err!=nil{return Entity{},err}
	if err:=tx.Commit(ctx);err!=nil{return Entity{},err}
	return e,nil
}

type CreateUserInput struct {
	Username, DisplayName, Email string
}

func (s *Store) ListUsers(ctx context.Context) ([]map[string]any,error) {
	rows,err:=s.Pool.Query(ctx,`SELECT public_id::text,username,display_name,email,status,created_at FROM users ORDER BY display_name,username`)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=[]map[string]any{}
	for rows.Next(){
		var id,username,name,status string
		var email *string
		var created any
		if err:=rows.Scan(&id,&username,&name,&email,&status,&created);err!=nil{return nil,err}
		out=append(out,map[string]any{"id":id,"username":username,"display_name":name,"email":email,"status":status,"created_at":created})
	}
	return out,rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, actor User, in CreateUserInput)(map[string]any,error){
	if strings.TrimSpace(in.Username)==""||strings.TrimSpace(in.DisplayName)==""{return nil,fmt.Errorf("username and display name are required")}
	id,_:=ids.UUIDv7();pub,_:=ids.ULID()
	_,err:=s.Pool.Exec(ctx,`INSERT INTO users(id,public_id,username,display_name,email) VALUES($1,$2,$3,$4,NULLIF($5,''))`,id,pub,in.Username,in.DisplayName,in.Email)
	if err!=nil{return nil,err}
	_=s.Audit(ctx,actor,nil,"USER_CREATE","USER",&pub,"SUCCESS",map[string]any{"username":in.Username,"display_name":in.DisplayName})
	return map[string]any{"id":pub,"username":in.Username,"display_name":in.DisplayName,"email":in.Email,"status":"ACTIVE"},nil
}

func (s *Store) SetUserEntityRole(ctx context.Context,actor User,targetUserPub string,e Entity,roleCode string)(map[string]any,error){
	var targetID,roleID string
	if err:=s.Pool.QueryRow(ctx,`SELECT id::text FROM users WHERE public_id=$1 AND status='ACTIVE'`,targetUserPub).Scan(&targetID);err!=nil{return nil,err}
	if err:=s.Pool.QueryRow(ctx,`SELECT id::text FROM roles WHERE code=$1`,roleCode).Scan(&roleID);err!=nil{return nil,err}

	tx,err:=s.Pool.Begin(ctx);if err!=nil{return nil,err};defer tx.Rollback(ctx)
	if _,err=tx.Exec(ctx,`UPDATE user_entity_roles SET revoked_at=now() WHERE user_id=$1 AND entity_id=$2 AND revoked_at IS NULL`,targetID,e.ID);err!=nil{return nil,err}
	linkID,_:=ids.UUIDv7()
	if _,err=tx.Exec(ctx,`INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by) VALUES($1,$2,$3,$4,$5)`,linkID,targetID,e.ID,roleID,actor.ID);err!=nil{return nil,err}
	if err=insertAuditTx(ctx,tx,actor,e,"USER_ROLE_CHANGE","USER",targetUserPub,map[string]any{"role":roleCode});err!=nil{return nil,err}
	if err=tx.Commit(ctx);err!=nil{return nil,err}
	return map[string]any{"user_id":targetUserPub,"entity_id":e.PublicID,"role":roleCode},nil
}

func (s *Store) ListEntityUsers(ctx context.Context,e Entity)([]map[string]any,error){
	rows,err:=s.Pool.Query(ctx,`SELECT u.public_id::text,u.username,u.display_name,u.email,r.code
FROM user_entity_roles uer
JOIN users u ON u.id=uer.user_id
JOIN roles r ON r.id=uer.role_id
WHERE uer.entity_id=$1 AND uer.revoked_at IS NULL
ORDER BY u.display_name,r.code`,e.ID)
	if err!=nil{return nil,err};defer rows.Close()
	out:=[]map[string]any{}
	for rows.Next(){var id,username,name,role string;var email *string;if err:=rows.Scan(&id,&username,&name,&email,&role);err!=nil{return nil,err};out=append(out,map[string]any{"id":id,"username":username,"display_name":name,"email":email,"role":role})}
	return out,rows.Err()
}
