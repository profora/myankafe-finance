package postgres

import (
	"strings"
	"testing"
)

func TestTransactionAttachmentMetadataLifecycle(t *testing.T) {
	ctx, s := integrationStore(t)
	user, entity, expenseAccount, financialAccount := seedServiceEntity(t, ctx, s, "ATTACHMENTS")

	tx, err := s.CreateTransaction(ctx, user, entity, CreateTransactionInput{
		Type:                     "EXPENSE",
		Date:                     "2026-09-24",
		Description:              "Attachment metadata test",
		FinancialAccountPublicID: financialAccount,
		Currency:                 "MMK",
		Splits: []SplitInput{{
			AccountPublicID: expenseAccount,
			Amount:          "1200",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	firstID := mustULID(t)
	secondID := mustULID(t)
	created, err := s.RegisterTransactionAttachments(ctx, user, entity, tx.PublicID, []NewAttachment{
		{
			PublicID:         firstID,
			StorageKey:       "transactions/" + entity.PublicID + "/" + tx.PublicID + "/" + firstID + "/receipt.jpg",
			OriginalFilename: "receipt.jpg",
			MimeType:         "image/jpeg",
			SizeBytes:        128,
			SHA256Hex:        strings.Repeat("a", 64),
		},
		{
			PublicID:         secondID,
			StorageKey:       "transactions/" + entity.PublicID + "/" + tx.PublicID + "/" + secondID + "/invoice.pdf",
			OriginalFilename: "invoice.pdf",
			MimeType:         "application/pdf",
			SizeBytes:        256,
			SHA256Hex:        strings.Repeat("b", 64),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created[0].DisplayOrder != 0 || created[1].DisplayOrder != 1 {
		t.Fatalf("unexpected created attachments: %+v", created)
	}

	items, err := s.ListTransactionAttachments(ctx, entity.ID, tx.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].PublicID != firstID || items[1].PublicID != secondID {
		t.Fatalf("unexpected attachment order: %+v", items)
	}

	obj, err := s.TransactionAttachmentObject(ctx, entity.ID, tx.PublicID, secondID)
	if err != nil {
		t.Fatal(err)
	}
	if obj.OriginalFilename != "invoice.pdf" || obj.MimeType != "application/pdf" || obj.SizeBytes != 256 {
		t.Fatalf("unexpected attachment object: %+v", obj)
	}

	_, otherEntity, _, _ := seedServiceEntity(t, ctx, s, "ATTACHMENTS_OTHER")
	if _, err := s.TransactionAttachmentObject(ctx, otherEntity.ID, tx.PublicID, secondID); err == nil {
		t.Fatal("expected cross-entity attachment access to fail")
	}

	var auditCount int
	if err := s.Pool.QueryRow(ctx, `
SELECT count(*)
FROM audit_events
WHERE entity_id=$1
  AND resource_public_id=$2
  AND action='TRANSACTION_ATTACHMENTS_UPLOAD'
  AND outcome='SUCCESS'`, entity.ID, tx.PublicID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("attachment upload audit count=%d want 1", auditCount)
	}
}
