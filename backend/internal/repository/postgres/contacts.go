package postgres

import (
	"context"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func (s *Store) ListContacts(ctx context.Context, entityID string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT c.public_id::text,c.contact_type,COALESCE(ct.name,c.contact_type),c.display_name,c.phone,c.email,c.notes,c.active
FROM contacts c
LEFT JOIN contact_types ct ON ct.entity_id=c.entity_id AND ct.code=c.contact_type
WHERE c.entity_id=$1
ORDER BY c.display_name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, typ, typeName, name string
		var phone, email, notes *string
		var active bool
		if err := rows.Scan(&id, &typ, &typeName, &name, &phone, &email, &notes, &active); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "contact_type": typ, "contact_type_name": typeName, "display_name": name, "phone": phone, "email": email, "notes": notes, "active": active})
	}
	return out, rows.Err()
}

func (s *Store) CreateContact(ctx context.Context, user User, e Entity, typ, name, phone, email, notes string) (map[string]any, error) {
	if typ == "" {
		typ = "OTHER"
	}
	var err error
	typ, err = NormalizeContactTypeCode(typ)
	if err != nil {
		return nil, err
	}
	if err := s.usableContactType(ctx, e.ID, typ, false); err != nil {
		return nil, err
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	_, err = s.Pool.Exec(ctx, `INSERT INTO contacts(id,public_id,entity_id,contact_type,display_name,phone,email,notes,created_by) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9)`, id, pub, e.ID, typ, name, phone, email, notes, user.ID)
	if err != nil {
		return nil, err
	}
	_ = s.Audit(ctx, user, &e, "CONTACT_CREATE", "CONTACT", &pub, "SUCCESS", map[string]any{"name": name, "contact_type": typ})
	return map[string]any{"id": pub, "contact_type": typ, "display_name": name, "phone": phone, "email": email, "notes": notes, "active": true}, nil
}
