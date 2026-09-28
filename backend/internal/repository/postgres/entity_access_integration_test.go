package postgres

import "testing"

func TestEntityAccessRemovalProtectsLastActiveOwner(t *testing.T) {
	ctx, s := integrationStore(t)
	actor := seedPlatformUser(t, s, "access-actor")
	target := seedPlatformUser(t, s, "access-target")

	entityID := mustUUID(t)
	entityPublic := mustULID(t)
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO entities(
 id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by
) VALUES($1,$2,$3,'Access Test Entity','BUSINESS','MMK','Asia/Yangon',$4)`,
		entityID, entityPublic, "ACCESS_"+entityPublic, actor.ID); err != nil {
		t.Fatal(err)
	}
	entity := Entity{ID: entityID, PublicID: entityPublic, Code: "ACCESS_" + entityPublic, Name: "Access Test Entity", Type: "BUSINESS", FunctionalCurrency: "MMK", Timezone: "Asia/Yangon", FiscalMonth: 1, FiscalDay: 1}

	if _, err := s.Pool.Exec(ctx, `
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$4)`,
		mustUUID(t), target.ID, entityID, actor.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.RevokeUserEntityAccess(ctx, actor, target.PublicID, entity); err == nil {
		t.Fatal("expected last active OWNER access removal to fail")
	}

	otherOwner := seedPlatformUser(t, s, "access-other")
	if _, err := s.Pool.Exec(ctx, `
INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by)
VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$4)`,
		mustUUID(t), otherOwner.ID, entityID, actor.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.RevokeUserEntityAccess(ctx, actor, target.PublicID, entity); err != nil {
		t.Fatal(err)
	}

	var activeRoles int
	if err := s.Pool.QueryRow(ctx, `
SELECT count(*) FROM user_entity_roles
WHERE user_id=$1 AND entity_id=$2 AND revoked_at IS NULL`, target.ID, entityID).Scan(&activeRoles); err != nil {
		t.Fatal(err)
	}
	if activeRoles != 0 {
		t.Fatalf("active role rows=%d want 0", activeRoles)
	}

	var auditCount int
	if err := s.Pool.QueryRow(ctx, `
SELECT count(*) FROM audit_events
WHERE entity_id=$1 AND action='USER_ENTITY_ACCESS_REVOKE' AND resource_public_id=$2`,
		entityID, target.PublicID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("access revoke audit count=%d want 1", auditCount)
	}
}
