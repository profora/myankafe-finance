package postgres

import (
	"testing"
	"time"
)

func seedPlatformUser(t *testing.T, s *Store, suffix string) User {
	t.Helper()
	ctx := t.Context()
	id := mustUUID(t)
	pub := mustULID(t)
	username := "status-" + suffix + "-" + lowerULID(pub)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO users(id,public_id,username,display_name)
VALUES($1,$2,$3,$4)`, id, pub, username, "Status "+suffix); err != nil {
		t.Fatal(err)
	}
	return User{ID:id,PublicID:pub,Username:username,DisplayName:"Status "+suffix}
}

func lowerULID(v string) string {
	out:=make([]byte,len(v))
	for i:=range v {
		c:=v[i]
		if c>='A'&&c<='Z' { c += 'a'-'A' }
		out[i]=c
	}
	return string(out)
}

func TestDisableUserRevokesSessionsAndReactivatePreservesIdentity(t *testing.T){
	ctx,s:=integrationStore(t)
	actor:=seedPlatformUser(t,s,"actor")
	target:=seedPlatformUser(t,s,"target")

	tokenHash:=make([]byte,32);tokenHash[0]=77
	if _,err:=s.CreateSession(ctx,target.ID,tokenHash,"Test Browser","127.0.0.1",time.Now().UTC().Add(time.Hour));err!=nil{
		t.Fatal(err)
	}

	if err:=s.SetUserStatus(ctx,actor,target.PublicID,"DISABLED");err!=nil{t.Fatal(err)}

	var status string
	var activeSessions int
	if err:=s.Pool.QueryRow(ctx,`
SELECT status,
       (SELECT count(*) FROM user_sessions WHERE user_id=u.id AND revoked_at IS NULL AND expires_at>now())
FROM users u WHERE id=$1`,target.ID).Scan(&status,&activeSessions);err!=nil{t.Fatal(err)}
	if status!="DISABLED"{t.Fatalf("status=%s want DISABLED",status)}
	if activeSessions!=0{t.Fatalf("active sessions=%d want 0",activeSessions)}

	if err:=s.SetUserStatus(ctx,actor,target.PublicID,"ACTIVE");err!=nil{t.Fatal(err)}
	if err:=s.Pool.QueryRow(ctx,`SELECT status FROM users WHERE id=$1`,target.ID).Scan(&status);err!=nil{t.Fatal(err)}
	if status!="ACTIVE"{t.Fatalf("status=%s want ACTIVE",status)}

	var auditCount int
	if err:=s.Pool.QueryRow(ctx,`
SELECT count(*) FROM audit_events
WHERE actor_user_id=$1 AND resource_public_id=$2 AND action='USER_STATUS_CHANGE' AND outcome='SUCCESS'`,
		actor.ID,target.PublicID).Scan(&auditCount);err!=nil{t.Fatal(err)}
	if auditCount!=2{t.Fatalf("status audit count=%d want 2",auditCount)}
}

func TestUserCannotDisableSelf(t *testing.T){
	ctx,s:=integrationStore(t)
	actor:=seedPlatformUser(t,s,"self")
	if err:=s.SetUserStatus(ctx,actor,actor.PublicID,"DISABLED");err==nil{
		t.Fatal("expected self-disable to be rejected")
	}
	var status string
	if err:=s.Pool.QueryRow(ctx,`SELECT status FROM users WHERE id=$1`,actor.ID).Scan(&status);err!=nil{t.Fatal(err)}
	if status!="ACTIVE"{t.Fatalf("self-disable mutated status=%s",status)}
}

func TestCannotDisableOnlyActiveOwner(t *testing.T){
	ctx,s:=integrationStore(t)
	actor:=seedPlatformUser(t,s,"admin")
	target:=seedPlatformUser(t,s,"sole-owner")

	entityID:=mustUUID(t)
	entityPublic:=mustULID(t)
	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO entities(
 id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,'Sole Owner Entity','BUSINESS','MMK','Asia/Yangon',$4)`,
		entityID,entityPublic,"SOLE_"+entityPublic,actor.ID);err!=nil{t.Fatal(err)}

	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$4)`,
		mustUUID(t),target.ID,entityID,actor.ID);err!=nil{t.Fatal(err)}

	if err:=s.SetUserStatus(ctx,actor,target.PublicID,"DISABLED");err==nil{
		t.Fatal("expected sole active OWNER disable to be rejected")
	}

	otherOwner:=seedPlatformUser(t,s,"other-owner")
	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$4)`,
		mustUUID(t),otherOwner.ID,entityID,actor.ID);err!=nil{t.Fatal(err)}

	if err:=s.SetUserStatus(ctx,actor,target.PublicID,"DISABLED");err!=nil{
		t.Fatalf("disable with another active owner: %v",err)
	}
}
