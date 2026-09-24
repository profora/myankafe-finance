package postgres

import "testing"

func TestUnusedManualExchangeRateCanBeCorrectedAndDeleted(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,_,_:=seedServiceEntity(t,ctx,s,"FX_UNUSED")

	created,err:=s.CreateExchangeRate(ctx,user,entity,"2026-09-24","USD","MMK","4000","MANUAL","initial")
	if err!=nil{t.Fatal(err)}
	id,ok:=created["id"].(string);if !ok||id==""{t.Fatalf("rate id=%v",created["id"])}

	updated,err:=s.UpdateExchangeRate(ctx,user,entity,id,UpdateExchangeRateInput{
		RateDate:"2026-09-24",FromCurrency:"USD",ToCurrency:"MMK",Rate:"4100",SourceReference:"corrected",
	})
	if err!=nil{t.Fatal(err)}
	if updated["rate"]!="4100"{t.Fatalf("updated rate=%v",updated["rate"])}

	if err:=s.DeleteExchangeRate(ctx,user,entity,id);err!=nil{t.Fatal(err)}
	var count int
	if err:=s.Pool.QueryRow(ctx,`SELECT count(*) FROM exchange_rates WHERE public_id=$1`,id).Scan(&count);err!=nil{t.Fatal(err)}
	if count!=0{t.Fatalf("remaining rate rows=%d want 0",count)}
}

func TestUsedExchangeRateCannotBeMutatedOrDeleted(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,expenseAccount,_:=seedServiceEntity(t,ctx,s,"FX_USED")

	assetID:=mustUUID(t)
	assetPublic:=mustULID(t)
	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO accounts(id,public_id,entity_id,code,name,account_type,is_postable,created_by)
VALUES($1,$2,$3,$4,'USD Bank','ASSET',true,$5)`,
		assetID,assetPublic,entity.ID,"USD_"+assetPublic,user.ID);err!=nil{t.Fatal(err)}
	faID:=mustUUID(t)
	faPublic:=mustULID(t)
	if _,err:=s.Pool.Exec(ctx,`
INSERT INTO financial_accounts(id,public_id,entity_id,account_id,code,name,kind,currency_code,created_by)
VALUES($1,$2,$3,$4,$5,'USD Bank','BANK','USD',$6)`,
		faID,faPublic,entity.ID,assetID,"FA_"+faPublic,user.ID);err!=nil{t.Fatal(err)}

	created,err:=s.CreateExchangeRate(ctx,user,entity,"2026-09-24","USD","MMK","4000","MANUAL","posting rate")
	if err!=nil{t.Fatal(err)}
	rateID:=created["id"].(string)

	tx,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",Date:"2026-09-24",Description:"USD expense",
		FinancialAccountPublicID:faPublic,Currency:"USD",
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"1"}},
	})
	if err!=nil{t.Fatal(err)}
	if _,err:=s.PostTransaction(ctx,user,entity,tx.PublicID);err!=nil{t.Fatal(err)}

	rates,err:=s.ListExchangeRates(ctx,entity.ID);if err!=nil{t.Fatal(err)}
	var used bool
	for _,rate:=range rates{
		if rate["id"]==rateID{
			used,_=rate["used"].(bool)
			break
		}
	}
	if !used{t.Fatal("posted rate should be marked used")}

	if _,err:=s.UpdateExchangeRate(ctx,user,entity,rateID,UpdateExchangeRateInput{
		RateDate:"2026-09-24",FromCurrency:"USD",ToCurrency:"MMK",Rate:"4200",SourceReference:"illegal correction",
	});err==nil{
		t.Fatal("expected used exchange rate update to fail")
	}
	if err:=s.DeleteExchangeRate(ctx,user,entity,rateID);err==nil{
		t.Fatal("expected used exchange rate delete to fail")
	}
}
