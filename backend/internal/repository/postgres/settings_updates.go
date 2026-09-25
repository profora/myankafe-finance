package postgres

import (
	"context"
	"fmt"
	"strings"
)

type UpdateContactInput struct {
	Type        string
	DisplayName string
	Phone       string
	Email       string
	Notes       string
	Active      bool
}

func (s *Store) UpdateContact(ctx context.Context, user User, e Entity, publicID string, in UpdateContactInput) (map[string]any, error) {
	in.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	switch in.Type {
	case "OTHER", "SUPPLIER", "CUSTOMER", "EMPLOYEE", "OWNER":
	default:
		return nil, fmt.Errorf("invalid contact type")
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" {
		return nil, fmt.Errorf("display name is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
SELECT id::text
FROM contacts
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`, e.ID, publicID).Scan(&id); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE contacts
SET contact_type=$3,
    display_name=$4,
    phone=NULLIF($5,''),
    email=NULLIF($6,''),
    notes=NULLIF($7,''),
    active=$8,
    updated_at=now()
WHERE id=$1 AND entity_id=$2`,
		id, e.ID, in.Type, in.DisplayName, strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email), strings.TrimSpace(in.Notes), in.Active); err != nil {
		return nil, err
	}

	if err := insertAuditTx(ctx, tx, user, e, "CONTACT_UPDATE", "CONTACT", publicID, map[string]any{
		"contact_type": in.Type,
		"display_name": in.DisplayName,
		"active":       in.Active,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"id":           publicID,
		"contact_type": in.Type,
		"display_name": in.DisplayName,
		"phone":        in.Phone,
		"email":        in.Email,
		"notes":        in.Notes,
		"active":       in.Active,
	}, nil
}

type UpdateFinancialAccountInput struct {
	Name        string
	Institution string
	Reference   string
	Active      bool
}

func (s *Store) UpdateFinancialAccount(ctx context.Context, user User, e Entity, publicID string, in UpdateFinancialAccountInput) (FinancialAccount, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return FinancialAccount{}, fmt.Errorf("name is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FinancialAccount{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
SELECT id::text
FROM financial_accounts
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`, e.ID, publicID).Scan(&id); err != nil {
		return FinancialAccount{}, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE financial_accounts
SET name=$3,
    institution_name=NULLIF($4,''),
    account_reference=NULLIF($5,''),
    active=$6,
    updated_at=now()
WHERE id=$1 AND entity_id=$2`,
		id, e.ID, in.Name, strings.TrimSpace(in.Institution), strings.TrimSpace(in.Reference), in.Active); err != nil {
		return FinancialAccount{}, err
	}

	var out FinancialAccount
	if err := tx.QueryRow(ctx, `
SELECT fa.public_id::text,fa.code,fa.name,fa.kind,fa.currency_code,
       a.public_id::text,fa.institution_name,fa.account_reference,fa.active
FROM financial_accounts fa
JOIN accounts a ON a.id=fa.account_id
WHERE fa.id=$1`, id).
		Scan(&out.PublicID, &out.Code, &out.Name, &out.Kind, &out.Currency, &out.AccountPublicID, &out.Institution, &out.Reference, &out.Active); err != nil {
		return FinancialAccount{}, err
	}

	if err := insertAuditTx(ctx, tx, user, e, "FINANCIAL_ACCOUNT_UPDATE", "FINANCIAL_ACCOUNT", publicID, map[string]any{
		"name":   out.Name,
		"active": out.Active,
	}); err != nil {
		return FinancialAccount{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return FinancialAccount{}, err
	}
	return out, nil
}

type UpdateAccountInput struct {
	Name    string
	Subtype string
	Active  bool
}

func (s *Store) UpdateAccount(ctx context.Context, user User, e Entity, publicID string, in UpdateAccountInput) (Account, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return Account{}, fmt.Errorf("name is required")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	var currentActive bool
	if err := tx.QueryRow(ctx, `
SELECT id::text,active
FROM accounts
WHERE entity_id=$1 AND public_id=$2
FOR UPDATE`, e.ID, publicID).Scan(&id, &currentActive); err != nil {
		return Account{}, err
	}

	if currentActive && !in.Active {
		var activeDescendants int
		if err := tx.QueryRow(ctx, `
WITH RECURSIVE descendants AS (
  SELECT id,parent_id,active
  FROM accounts
  WHERE entity_id=$1 AND parent_id=$2
  UNION ALL
  SELECT a.id,a.parent_id,a.active
  FROM accounts a
  JOIN descendants d ON a.parent_id=d.id
  WHERE a.entity_id=$1
)
SELECT count(*) FROM descendants WHERE active=true`, e.ID, id).Scan(&activeDescendants); err != nil {
			return Account{}, err
		}
		if activeDescendants > 0 {
			return Account{}, fmt.Errorf("deactivate active child accounts first")
		}

		var activeFinancial int
		if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM financial_accounts
WHERE entity_id=$1 AND account_id=$2 AND active=true`, e.ID, id).Scan(&activeFinancial); err != nil {
			return Account{}, err
		}
		if activeFinancial > 0 {
			return Account{}, fmt.Errorf("deactivate linked financial accounts first")
		}
	}

	if _, err := tx.Exec(ctx, `
UPDATE accounts
SET name=$3,
    account_subtype=NULLIF($4,''),
    active=$5,
    updated_at=now()
WHERE id=$1 AND entity_id=$2`,
		id, e.ID, in.Name, strings.TrimSpace(in.Subtype), in.Active); err != nil {
		return Account{}, err
	}

	var out Account
	if err := tx.QueryRow(ctx, `
SELECT public_id::text,code,name,account_type,account_subtype,is_postable,active
FROM accounts WHERE id=$1`, id).
		Scan(&out.PublicID, &out.Code, &out.Name, &out.Type, &out.Subtype, &out.Postable, &out.Active); err != nil {
		return Account{}, err
	}

	if err := insertAuditTx(ctx, tx, user, e, "COA_UPDATE", "ACCOUNT", publicID, map[string]any{
		"name":    out.Name,
		"subtype": out.Subtype,
		"active":  out.Active,
	}); err != nil {
		return Account{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, err
	}
	return out, nil
}
