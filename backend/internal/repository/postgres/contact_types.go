package postgres

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type ContactType struct {
	PublicID string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

type ContactTypeInput struct {
	Code   string
	Name   string
	Active bool
}

func NormalizeContactTypeCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if b.Len() > 0 && !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" || len(out) > 40 {
		return "", fmt.Errorf("contact type code is required")
	}
	return out, nil
}

func (s *Store) ListContactTypes(ctx context.Context, entityID string) ([]ContactType, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT public_id::text, code, name, active
FROM contact_types
WHERE entity_id=$1
ORDER BY name, code`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ContactType{}
	for rows.Next() {
		var item ContactType
		if err := rows.Scan(&item.PublicID, &item.Code, &item.Name, &item.Active); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) usableContactType(ctx context.Context, entityID, code string, allowInactive bool) error {
	var active bool
	err := s.Pool.QueryRow(ctx, `SELECT active FROM contact_types WHERE entity_id=$1 AND code=$2`, entityID, code).Scan(&active)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("unknown contact type")
		}
		return err
	}
	if !active && !allowInactive {
		return fmt.Errorf("contact type is inactive")
	}
	return nil
}

func (s *Store) CreateContactType(ctx context.Context, user User, e Entity, in ContactTypeInput) (ContactType, error) {
	code, err := NormalizeContactTypeCode(in.Code)
	if err != nil {
		return ContactType{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ContactType{}, fmt.Errorf("contact type name is required")
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	_, err = s.Pool.Exec(ctx, `
INSERT INTO contact_types(id, public_id, entity_id, code, name, active, created_by)
VALUES($1,$2,$3,$4,$5,$6,$7)`, id, pub, e.ID, code, name, in.Active, user.ID)
	if err != nil {
		return ContactType{}, err
	}
	if err := s.Audit(ctx, user, &e, "CONTACT_TYPE_CREATE", "CONTACT_TYPE", &pub, "SUCCESS", map[string]any{
		"code": code, "name": name, "active": in.Active,
	}); err != nil {
		return ContactType{}, err
	}
	return ContactType{PublicID: pub, Code: code, Name: name, Active: in.Active}, nil
}

func (s *Store) UpdateContactType(ctx context.Context, user User, e Entity, code string, name string, active bool) (ContactType, error) {
	code, err := NormalizeContactTypeCode(code)
	if err != nil {
		return ContactType{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ContactType{}, fmt.Errorf("contact type name is required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ContactType{}, err
	}
	defer tx.Rollback(ctx)

	var id, pub string
	var wasActive bool
	err = tx.QueryRow(ctx, `
SELECT id::text, public_id::text, active
FROM contact_types
WHERE entity_id=$1 AND code=$2
FOR UPDATE`, e.ID, code).Scan(&id, &pub, &wasActive)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ContactType{}, fmt.Errorf("unknown contact type")
		}
		return ContactType{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE contact_types
SET name=$3, active=$4, updated_at=now()
WHERE id=$1 AND entity_id=$2`, id, e.ID, name, active); err != nil {
		return ContactType{}, err
	}
	action := "CONTACT_TYPE_UPDATE"
	if wasActive != active {
		if active {
			action = "CONTACT_TYPE_ACTIVATE"
		} else {
			action = "CONTACT_TYPE_DEACTIVATE"
		}
	}
	if err := insertAuditTx(ctx, tx, user, e, action, "CONTACT_TYPE", pub, map[string]any{
		"code": code, "name": name, "active": active,
	}); err != nil {
		return ContactType{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ContactType{}, err
	}
	return ContactType{PublicID: pub, Code: code, Name: name, Active: active}, nil
}

func (s *Store) DeleteContactType(ctx context.Context, user User, e Entity, code string) error {
	code, err := NormalizeContactTypeCode(code)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id, pub string
	err = tx.QueryRow(ctx, `
SELECT id::text, public_id::text
FROM contact_types
WHERE entity_id=$1 AND code=$2
FOR UPDATE`, e.ID, code).Scan(&id, &pub)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("unknown contact type")
		}
		return err
	}
	var used int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM contacts WHERE entity_id=$1 AND contact_type=$2`, e.ID, code).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("contact type is in use; deactivate it instead")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM contact_types WHERE id=$1 AND entity_id=$2`, id, e.ID); err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, user, e, "CONTACT_TYPE_DELETE", "CONTACT_TYPE", pub, map[string]any{"code": code}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
