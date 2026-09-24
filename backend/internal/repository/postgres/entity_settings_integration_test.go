package postgres

import "testing"

func TestEntitySettingsUpdatePreservesAccountingIdentity(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,_,_:=seedServiceEntity(t,ctx,s,"ENTITY_SETTINGS")

	updated,err:=s.UpdateEntitySettings(ctx,user,entity,UpdateEntitySettingsInput{
		Name:"Renamed Entity",
		Timezone:"Asia/Singapore",
		FiscalMonth:4,
		FiscalDay:1,
	})
	if err!=nil{t.Fatal(err)}
	if updated.Name!="Renamed Entity"{t.Fatalf("name=%q",updated.Name)}
	if updated.Timezone!="Asia/Singapore"{t.Fatalf("timezone=%q",updated.Timezone)}
	if updated.FiscalMonth!=4||updated.FiscalDay!=1{t.Fatalf("fiscal=%d/%d",updated.FiscalMonth,updated.FiscalDay)}
	if updated.Code!=entity.Code||updated.Type!=entity.Type||updated.FunctionalCurrency!=entity.FunctionalCurrency{
		t.Fatalf("accounting identity changed: before=%+v after=%+v",entity,updated)
	}

	var auditCount int
	if err:=s.Pool.QueryRow(ctx,`
SELECT count(*) FROM audit_events
WHERE entity_id=$1 AND action='ENTITY_SETTINGS_UPDATE' AND resource_public_id=$2`,
		entity.ID,entity.PublicID).Scan(&auditCount);err!=nil{t.Fatal(err)}
	if auditCount!=1{t.Fatalf("entity settings audit count=%d want 1",auditCount)}
}

func TestEntitySettingsRejectInvalidTimezoneAndFiscalDate(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,_,_:=seedServiceEntity(t,ctx,s,"ENTITY_SETTINGS_INVALID")

	if _,err:=s.UpdateEntitySettings(ctx,user,entity,UpdateEntitySettingsInput{
		Name:"Entity",Timezone:"Not/A_Timezone",FiscalMonth:1,FiscalDay:1,
	});err==nil{
		t.Fatal("expected invalid timezone rejection")
	}
	if _,err:=s.UpdateEntitySettings(ctx,user,entity,UpdateEntitySettingsInput{
		Name:"Entity",Timezone:"Asia/Yangon",FiscalMonth:2,FiscalDay:30,
	});err==nil{
		t.Fatal("expected invalid fiscal date rejection")
	}
}
