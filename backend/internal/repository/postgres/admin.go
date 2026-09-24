package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s *Store) CreateUser(ctx context.Context, actor User, in CreateUserInput,passwordHash string)(map[string]any,error){
	if strings.TrimSpace(in.Username)==""||strings.TrimSpace(in.DisplayName)==""{return nil,fmt.Errorf("username and display name are required")}
	if strings.TrimSpace(passwordHash)==""{return nil,fmt.Errorf("password hash is required")}
	tx,err:=s.Pool.Begin(ctx);if err!=nil{return nil,err};defer tx.Rollback(ctx)
	id,_:=ids.UUIDv7();pub,_:=ids.ULID()
	if _,err=tx.Exec(ctx,`INSERT INTO users(id,public_id,username,display_name,email) VALUES($1,$2,$3,$4,NULLIF($5,''))`,id,pub,in.Username,in.DisplayName,in.Email);err!=nil{return nil,err}
	if _,err=tx.Exec(ctx,`INSERT INTO user_credentials(user_id,password_hash) VALUES($1,$2)`,id,passwordHash);err!=nil{return nil,err}
	if err=tx.Commit(ctx);err!=nil{return nil,err}
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


func (s *Store) SetUserStatus(ctx context.Context,actor User,targetUserPublicID,status string) error {
	status=strings.ToUpper(strings.TrimSpace(status))
	if status!="ACTIVE"&&status!="DISABLED"{return fmt.Errorf("status must be ACTIVE or DISABLED")}

	tx,err:=s.Pool.Begin(ctx);if err!=nil{return err}
	defer tx.Rollback(ctx)

	var targetID,currentStatus,displayName string
	if err:=tx.QueryRow(ctx,`
SELECT id::text,status,display_name
FROM users
WHERE public_id=$1
FOR UPDATE`,targetUserPublicID).Scan(&targetID,&currentStatus,&displayName);err!=nil{return err}

	if targetID==actor.ID&&status=="DISABLED"{
		return fmt.Errorf("you cannot disable your own user account")
	}
	if currentStatus==status{return nil}

	if status=="DISABLED"{
		var soleOwnerEntity string
		err:=tx.QueryRow(ctx,`
SELECT e.name
FROM user_entity_roles uer
JOIN roles r ON r.id=uer.role_id
JOIN entities e ON e.id=uer.entity_id
WHERE uer.user_id=$1
  AND uer.revoked_at IS NULL
  AND r.code='OWNER'
  AND e.active=true
  AND NOT EXISTS (
    SELECT 1
    FROM user_entity_roles other
    JOIN roles other_role ON other_role.id=other.role_id
    JOIN users other_user ON other_user.id=other.user_id
    WHERE other.entity_id=uer.entity_id
      AND other.revoked_at IS NULL
      AND other_role.code='OWNER'
      AND other_user.status='ACTIVE'
      AND other_user.id<>$1
  )
LIMIT 1`,targetID).Scan(&soleOwnerEntity)
		if err==nil{return fmt.Errorf("cannot disable %s because they are the only active OWNER of %s",displayName,soleOwnerEntity)}
		if err!=pgx.ErrNoRows{return err}
	}

	if _,err:=tx.Exec(ctx,`
UPDATE users
SET status=$2,updated_at=now()
WHERE id=$1`,targetID,status);err!=nil{return err}

	if status=="DISABLED"{
		if _,err:=tx.Exec(ctx,`
UPDATE user_sessions
SET revoked_at=COALESCE(revoked_at,now())
WHERE user_id=$1 AND revoked_at IS NULL`,targetID);err!=nil{return err}
	}

	auditID,err:=ids.UUIDv7();if err!=nil{return err}
	auditPublicID,err:=ids.ULID();if err!=nil{return err}
	if _,err:=tx.Exec(ctx,`
INSERT INTO audit_events(
  id,public_id,actor_type,actor_user_id,action,resource_type,resource_public_id,
  outcome,source,request_id,after_data
) VALUES(
  $1,$2,'USER',$3,'USER_STATUS_CHANGE','USER',$4,
  'SUCCESS','API',$5,jsonb_build_object('status',$6)
)`,auditID,auditPublicID,actor.ID,targetUserPublicID,fmt.Sprintf("req-%d",time.Now().UnixNano()),status);err!=nil{return err}

	return tx.Commit(ctx)
}
