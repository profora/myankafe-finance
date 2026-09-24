package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type TransactionAttachment struct {
	PublicID         string    `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	DisplayOrder     int       `json:"display_order"`
	CreatedAt        time.Time `json:"created_at"`
}

type NewAttachment struct {
	PublicID         string
	StorageKey       string
	OriginalFilename string
	MimeType         string
	SizeBytes        int64
	SHA256Hex        string
}

type AttachmentObject struct {
	PublicID         string
	StorageKey       string
	OriginalFilename string
	MimeType         string
	SizeBytes        int64
}

func (s *Store) ListTransactionAttachments(ctx context.Context,entityID,transactionPublicID string)([]TransactionAttachment,error){
	rows,err:=s.Pool.Query(ctx,`
SELECT ta.public_id::text,ta.original_filename,ta.mime_type,ta.size_bytes,ta.display_order,ta.created_at
FROM transaction_attachments ta
JOIN transactions t ON t.id=ta.transaction_id
WHERE t.entity_id=$1 AND t.public_id=$2 AND ta.deleted_at IS NULL
ORDER BY ta.display_order,ta.created_at,ta.public_id`,entityID,transactionPublicID)
	if err!=nil{return nil,err}
	defer rows.Close()
	out:=[]TransactionAttachment{}
	for rows.Next(){
		var a TransactionAttachment
		if err:=rows.Scan(&a.PublicID,&a.OriginalFilename,&a.MimeType,&a.SizeBytes,&a.DisplayOrder,&a.CreatedAt);err!=nil{return nil,err}
		out=append(out,a)
	}
	return out,rows.Err()
}

func (s *Store) RegisterTransactionAttachments(ctx context.Context,user User,e Entity,transactionPublicID string,input []NewAttachment)([]TransactionAttachment,error){
	if len(input)==0{return nil,fmt.Errorf("at least one attachment is required")}
	tx,err:=s.Pool.Begin(ctx);if err!=nil{return nil,err}
	defer tx.Rollback(ctx)

	var transactionID string
	if err:=tx.QueryRow(ctx,`
SELECT id::text FROM transactions
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`,e.ID,transactionPublicID).Scan(&transactionID);err!=nil{return nil,err}

	var existing int
	if err:=tx.QueryRow(ctx,`
SELECT count(*) FROM transaction_attachments
WHERE transaction_id=$1 AND deleted_at IS NULL`,transactionID).Scan(&existing);err!=nil{return nil,err}
	if existing+len(input)>50{return nil,fmt.Errorf("a transaction can have at most 50 active attachments")}

	var nextOrder int
	if err:=tx.QueryRow(ctx,`
SELECT COALESCE(max(display_order),-1)+1
FROM transaction_attachments
WHERE transaction_id=$1 AND deleted_at IS NULL`,transactionID).Scan(&nextOrder);err!=nil{return nil,err}

	out:=make([]TransactionAttachment,0,len(input))
	for i,in:=range input{
		internalID,err:=ids.UUIDv7();if err!=nil{return nil,err}
		var created time.Time
		order:=nextOrder+i
		_,err=tx.Exec(ctx,`
INSERT INTO transaction_attachments(
 id,public_id,transaction_id,storage_key,original_filename,mime_type,size_bytes,
 uploaded_by,display_order,sha256_hex
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			internalID,in.PublicID,transactionID,in.StorageKey,in.OriginalFilename,in.MimeType,in.SizeBytes,user.ID,order,in.SHA256Hex)
		if err!=nil{return nil,err}
		if err:=tx.QueryRow(ctx,`SELECT created_at FROM transaction_attachments WHERE id=$1`,internalID).Scan(&created);err!=nil{return nil,err}
		out=append(out,TransactionAttachment{
			PublicID:in.PublicID,OriginalFilename:in.OriginalFilename,MimeType:in.MimeType,
			SizeBytes:in.SizeBytes,DisplayOrder:order,CreatedAt:created,
		})
	}
	if err:=insertAuditTx(ctx,tx,user,e,"TRANSACTION_ATTACHMENTS_UPLOAD","TRANSACTION",transactionPublicID,map[string]any{"count":len(out)});err!=nil{return nil,err}
	if err:=tx.Commit(ctx);err!=nil{return nil,err}
	return out,nil
}

func (s *Store) TransactionAttachmentObject(ctx context.Context,entityID,transactionPublicID,attachmentPublicID string)(AttachmentObject,error){
	var out AttachmentObject
	err:=s.Pool.QueryRow(ctx,`
SELECT ta.public_id::text,ta.storage_key,ta.original_filename,ta.mime_type,ta.size_bytes
FROM transaction_attachments ta
JOIN transactions t ON t.id=ta.transaction_id
WHERE t.entity_id=$1 AND t.public_id=$2 AND ta.public_id=$3 AND ta.deleted_at IS NULL`,
		entityID,transactionPublicID,attachmentPublicID).
		Scan(&out.PublicID,&out.StorageKey,&out.OriginalFilename,&out.MimeType,&out.SizeBytes)
	return out,err
}
