package postgres

import "testing"

func TestListEntitiesReturnsHighestEffectiveRole(t *testing.T) {
	ctx,s:=integrationStore(t)
	user:=seedPlatformUser(t,s,"effective-role")

	entityID:=mustUUID(t)
	entityPublic:=mustULID(t)
	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO entities(
 id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,'Effective Role Entity','BUSINESS','MMK','Asia/Yangon',$4)`,
		entityID,entityPublic,"ROLE_"+entityPublic,user.ID);err!=nil{t.Fatal(err)}

	for _,roleID:=range []string{
		"00000000-0000-7000-8000-000000000005",
		"00000000-0000-7000-8000-000000000003",
		"00000000-0000-7000-8000-000000000001",
	}{
		if _,err:=s.Pool.Exec(ctx,`
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,$4,$2)`,mustUUID(t),user.ID,entityID,roleID);err!=nil{t.Fatal(err)}
	}

	entities,err:=s.ListEntities(ctx,user.ID);if err!=nil{t.Fatal(err)}
	var found *Entity
	for i:=range entities{
		if entities[i].PublicID==entityPublic{found=&entities[i];break}
	}
	if found==nil{t.Fatal("entity missing from accessible entity list")}
	if found.Role!="OWNER"{t.Fatalf("effective role=%q want OWNER",found.Role)}

	resolved,role,err:=s.ResolveEntityAccess(ctx,user.ID,entityPublic);if err!=nil{t.Fatal(err)}
	if role!="OWNER"||resolved.Role!="OWNER"{t.Fatalf("resolved role=%q entity role=%q",role,resolved.Role)}
}
