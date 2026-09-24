package postgres

import "testing"

func TestInactiveContactPreservesHistoryButCannotBeReused(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,expenseAccount,financialAccount:=seedServiceEntity(t,ctx,s,"CONTACT_LIFECYCLE")

	created,err:=s.CreateContact(ctx,user,entity,"SUPPLIER","Supplier A","09-000","","")
	if err!=nil{t.Fatal(err)}
	contactID,ok:=created["id"].(string);if !ok||contactID==""{t.Fatalf("contact id=%v",created["id"])}

	tx,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",Date:"2026-09-24",Description:"Historical supplier expense",
		FinancialAccountPublicID:financialAccount,Currency:"MMK",ContactPublicID:contactID,
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"1000"}},
	})
	if err!=nil{t.Fatal(err)}

	if _,err:=s.UpdateContact(ctx,user,entity,contactID,UpdateContactInput{
		Type:"SUPPLIER",DisplayName:"Supplier A",Phone:"09-000",Active:false,
	});err!=nil{t.Fatal(err)}

	var retained string
	if err:=s.Pool.QueryRow(ctx,`
SELECT c.public_id::text
FROM transactions t
JOIN contacts c ON c.id=t.contact_id
WHERE t.public_id=$1`,tx.PublicID).Scan(&retained);err!=nil{t.Fatal(err)}
	if retained!=contactID{t.Fatalf("historical contact=%s want %s",retained,contactID)}

	if _,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",Date:"2026-09-25",Description:"Should reject inactive contact",
		FinancialAccountPublicID:financialAccount,Currency:"MMK",ContactPublicID:contactID,
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"500"}},
	});err==nil{
		t.Fatal("expected inactive contact to be rejected for new transaction")
	}
}

func TestInactiveFinancialAccountPreservesHistoryButCannotBeReused(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,expenseAccount,financialAccount:=seedServiceEntity(t,ctx,s,"FA_LIFECYCLE")

	tx,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",Date:"2026-09-24",Description:"Historical cash expense",
		FinancialAccountPublicID:financialAccount,Currency:"MMK",
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"2000"}},
	})
	if err!=nil{t.Fatal(err)}
	if _,err:=s.PostTransaction(ctx,user,entity,tx.PublicID);err!=nil{t.Fatal(err)}

	updated,err:=s.UpdateFinancialAccount(ctx,user,entity,financialAccount,UpdateFinancialAccountInput{
		Name:"Archived Cash",Institution:"",Reference:"drawer-1",Active:false,
	})
	if err!=nil{t.Fatal(err)}
	if updated.Active{t.Fatal("expected financial account inactive")}
	if updated.Name!="Archived Cash"{t.Fatalf("name=%q",updated.Name)}

	var historyCount int
	if err:=s.Pool.QueryRow(ctx,`
SELECT count(*)
FROM journal_lines jl
JOIN financial_accounts fa ON fa.id=jl.financial_account_id
WHERE fa.public_id=$1`,financialAccount).Scan(&historyCount);err!=nil{t.Fatal(err)}
	if historyCount==0{t.Fatal("expected historical journal line to retain financial account")}

	if _,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",Date:"2026-09-25",Description:"Should reject inactive financial account",
		FinancialAccountPublicID:financialAccount,Currency:"MMK",
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"500"}},
	});err==nil{
		t.Fatal("expected inactive financial account to be rejected for new transaction")
	}
}
