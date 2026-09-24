package postgres

import "testing"

func TestAttachmentReorderAndSoftDelete(t *testing.T){
	ctx,s:=integrationStore(t)
	user,entity,expenseAccount,financialAccount:=seedServiceEntity(t,ctx,s,"ATTACHMENTS")

	tx,err:=s.CreateTransaction(ctx,user,entity,CreateTransactionInput{
		Type:"EXPENSE",
		Date:"2026-09-24",
		Description:"Attachment management test",
		FinancialAccountPublicID:financialAccount,
		Currency:"MMK",
		Splits:[]SplitInput{{AccountPublicID:expenseAccount,Amount:"1000"}},
	})
	if err!=nil{t.Fatal(err)}

	a:=mustULID(t)
	b:=mustULID(t)
	items,err:=s.RegisterTransactionAttachments(ctx,user,entity,tx.PublicID,[]NewAttachment{
		{PublicID:a,StorageKey:"test/"+a,OriginalFilename:"a.pdf",MimeType:"application/pdf",SizeBytes:10,SHA256Hex:"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{PublicID:b,StorageKey:"test/"+b,OriginalFilename:"b.pdf",MimeType:"application/pdf",SizeBytes:20,SHA256Hex:"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	})
	if err!=nil{t.Fatal(err)}
	if len(items)!=2||items[0].PublicID!=a||items[1].PublicID!=b{t.Fatalf("initial items=%+v",items)}

	if err:=s.ReorderTransactionAttachments(ctx,user,entity,tx.PublicID,[]string{b,a});err!=nil{t.Fatal(err)}
	items,err=s.ListTransactionAttachments(ctx,entity.ID,tx.PublicID);if err!=nil{t.Fatal(err)}
	if len(items)!=2||items[0].PublicID!=b||items[1].PublicID!=a{t.Fatalf("reordered items=%+v",items)}

	deleted,err:=s.SoftDeleteTransactionAttachment(ctx,user,entity,tx.PublicID,b);if err!=nil{t.Fatal(err)}
	if deleted.PublicID!=b{t.Fatalf("deleted=%+v",deleted)}
	items,err=s.ListTransactionAttachments(ctx,entity.ID,tx.PublicID);if err!=nil{t.Fatal(err)}
	if len(items)!=1||items[0].PublicID!=a{t.Fatalf("remaining items=%+v",items)}

	var deletedAt any
	if err:=s.Pool.QueryRow(ctx,`SELECT deleted_at FROM transaction_attachments WHERE public_id=$1`,b).Scan(&deletedAt);err!=nil{t.Fatal(err)}
	if deletedAt==nil{t.Fatal("soft-deleted attachment must retain deleted_at")}
}
