package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type User struct{ ID, PublicID, Username, DisplayName string }
type Entity struct {
	ID, PublicID, Code, Name, Type, FunctionalCurrency, Timezone, Role string
	FiscalMonth, FiscalDay                                             int
}
type Account struct {
	PublicID, Code, Name, Type string
	Subtype                    *string
	Postable, Active           bool
}
type FinancialAccount struct {
	PublicID, Code, Name, Kind, Currency, AccountPublicID string
	Institution, Reference                                *string
	Active                                                bool
}

func (s *Store) ResolveUser(ctx context.Context, pub string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT id::text,public_id::text,username,display_name FROM users WHERE public_id=$1 AND status='ACTIVE'`, pub).Scan(&u.ID, &u.PublicID, &u.Username, &u.DisplayName)
	return u, err
}

func (s *Store) ListEntities(ctx context.Context, userID string) ([]Entity, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT e.id::text,e.public_id::text,e.code,e.name,e.entity_type,e.functional_currency_code,e.timezone,
       e.fiscal_year_start_month,e.fiscal_year_start_day,
       (
         SELECT r.code
         FROM user_entity_roles uer
         JOIN roles r ON r.id=uer.role_id
         WHERE uer.entity_id=e.id AND uer.user_id=$1 AND uer.revoked_at IS NULL
         ORDER BY CASE r.code WHEN 'OWNER' THEN 1 WHEN 'ADMIN' THEN 2 WHEN 'ACCOUNTANT' THEN 3 WHEN 'BOOKKEEPER' THEN 4 ELSE 5 END
         LIMIT 1
       ) effective_role
FROM entities e
WHERE e.active=true
  AND EXISTS (
    SELECT 1 FROM user_entity_roles x
    WHERE x.entity_id=e.id AND x.user_id=$1 AND x.revoked_at IS NULL
  )
ORDER BY e.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entity{}
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.PublicID, &e.Code, &e.Name, &e.Type, &e.FunctionalCurrency, &e.Timezone, &e.FiscalMonth, &e.FiscalDay, &e.Role); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ResolveEntityAccess(ctx context.Context, userID, pub string) (Entity, string, error) {
	var e Entity
	err := s.Pool.QueryRow(ctx, `SELECT e.id::text,e.public_id::text,e.code,e.name,e.entity_type,e.functional_currency_code,e.timezone,e.fiscal_year_start_month,e.fiscal_year_start_day,r.code
FROM entities e JOIN user_entity_roles uer ON uer.entity_id=e.id JOIN roles r ON r.id=uer.role_id
WHERE e.public_id=$1 AND uer.user_id=$2 AND uer.revoked_at IS NULL AND e.active=true
ORDER BY CASE r.code WHEN 'OWNER' THEN 1 WHEN 'ADMIN' THEN 2 WHEN 'ACCOUNTANT' THEN 3 WHEN 'BOOKKEEPER' THEN 4 ELSE 5 END LIMIT 1`, pub, userID).Scan(&e.ID, &e.PublicID, &e.Code, &e.Name, &e.Type, &e.FunctionalCurrency, &e.Timezone, &e.FiscalMonth, &e.FiscalDay, &e.Role)
	return e, e.Role, err
}

func (s *Store) ListAccounts(ctx context.Context, entityID string) ([]Account, error) {
	rows, err := s.Pool.Query(ctx, `SELECT public_id::text,code,name,account_type,account_subtype,is_postable,active FROM accounts WHERE entity_id=$1 ORDER BY code`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.PublicID, &a.Code, &a.Name, &a.Type, &a.Subtype, &a.Postable, &a.Active); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type CreateAccountInput struct {
	Code, Name, Type, Subtype, ParentPublicID string
	Postable                                  bool
}

func (s *Store) CreateAccount(ctx context.Context, user User, e Entity, in CreateAccountInput) (Account, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Type = strings.ToUpper(strings.TrimSpace(in.Type))
	in.Subtype = strings.TrimSpace(in.Subtype)
	if in.Code == "" || in.Name == "" {
		return Account{}, fmt.Errorf("code and name are required")
	}
	switch in.Type {
	case "ASSET", "LIABILITY", "EQUITY", "INCOME", "EXPENSE":
	default:
		return Account{}, fmt.Errorf("invalid account type")
	}

	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	var parent any
	if in.ParentPublicID != "" {
		var pid, parentType string
		var active bool
		if err := s.Pool.QueryRow(ctx, `SELECT id::text,account_type,active FROM accounts WHERE entity_id=$1 AND public_id=$2`, e.ID, in.ParentPublicID).Scan(&pid, &parentType, &active); err != nil {
			return Account{}, err
		}
		if !active {
			return Account{}, fmt.Errorf("parent account is inactive")
		}
		if parentType != in.Type {
			return Account{}, fmt.Errorf("parent account must have the same fundamental account type")
		}
		parent = pid
	}
	var a Account
	err := s.Pool.QueryRow(ctx, `INSERT INTO accounts(id,public_id,entity_id,code,name,parent_id,account_type,account_subtype,is_postable,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10)
RETURNING public_id::text,code,name,account_type,account_subtype,is_postable,active`, id, pub, e.ID, in.Code, in.Name, parent, in.Type, in.Subtype, in.Postable, user.ID).Scan(&a.PublicID, &a.Code, &a.Name, &a.Type, &a.Subtype, &a.Postable, &a.Active)
	if err == nil {
		_ = s.Audit(ctx, user, &e, "COA_CREATE", "ACCOUNT", &pub, "SUCCESS", map[string]any{"code": in.Code, "name": in.Name})
	}
	return a, err
}

func (s *Store) ListFinancialAccounts(ctx context.Context, entityID string) ([]FinancialAccount, error) {
	rows, err := s.Pool.Query(ctx, `SELECT fa.public_id::text,fa.code,fa.name,fa.kind,fa.currency_code,a.public_id::text,fa.institution_name,fa.account_reference,fa.active
FROM financial_accounts fa JOIN accounts a ON a.id=fa.account_id WHERE fa.entity_id=$1 ORDER BY fa.active DESC,fa.name`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FinancialAccount{}
	for rows.Next() {
		var f FinancialAccount
		if err := rows.Scan(&f.PublicID, &f.Code, &f.Name, &f.Kind, &f.Currency, &f.AccountPublicID, &f.Institution, &f.Reference, &f.Active); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

type CreateFinancialAccountInput struct{ Code, Name, Kind, Currency, AccountPublicID, Institution, Reference string }

func (s *Store) CreateFinancialAccount(ctx context.Context, user User, e Entity, in CreateFinancialAccountInput) (FinancialAccount, error) {
	var aid, typ string
	var postable bool
	if err := s.Pool.QueryRow(ctx, `SELECT id::text,account_type,is_postable FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true AND active=true`, e.ID, in.AccountPublicID).Scan(&aid, &typ, &postable); err != nil {
		return FinancialAccount{}, err
	}
	if !postable || (typ != "ASSET" && typ != "LIABILITY") {
		return FinancialAccount{}, fmt.Errorf("financial account must map to a postable ASSET or LIABILITY account")
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	var f FinancialAccount
	err := s.Pool.QueryRow(ctx, `INSERT INTO financial_accounts(id,public_id,entity_id,account_id,code,name,kind,currency_code,institution_name,account_reference,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),NULLIF($10,''),$11)
RETURNING public_id::text,code,name,kind,currency_code,$12,NULLIF(institution_name,''),NULLIF(account_reference,''),active`, id, pub, e.ID, aid, in.Code, in.Name, in.Kind, in.Currency, in.Institution, in.Reference, user.ID, in.AccountPublicID).Scan(&f.PublicID, &f.Code, &f.Name, &f.Kind, &f.Currency, &f.AccountPublicID, &f.Institution, &f.Reference, &f.Active)
	if err == nil {
		_ = s.Audit(ctx, user, &e, "FINANCIAL_ACCOUNT_CREATE", "FINANCIAL_ACCOUNT", &pub, "SUCCESS", map[string]any{"code": in.Code, "name": in.Name})
	}
	return f, err
}

func (s *Store) Audit(ctx context.Context, user User, e *Entity, action, resource string, pub *string, outcome string, after map[string]any) error {
	id, _ := ids.UUIDv7()
	apub, _ := ids.ULID()
	var eid any
	if e != nil {
		eid = e.ID
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO audit_events(id,public_id,actor_type,actor_user_id,entity_id,action,resource_type,resource_public_id,outcome,source,request_id,after_data)
VALUES($1,$2,'USER',$3,$4,$5,$6,$7,$8,'API',$9,$10)`, id, apub, user.ID, eid, action, resource, pub, outcome, fmt.Sprintf("req-%d", time.Now().UnixNano()), after)
	return err
}

func (s *Store) EnsureOpenDateTx(ctx context.Context, tx pgx.Tx, entityID, date string) error {
	var locked *time.Time
	err := tx.QueryRow(ctx, `INSERT INTO entity_accounting_controls(entity_id) VALUES($1) ON CONFLICT(entity_id) DO UPDATE SET entity_id=EXCLUDED.entity_id RETURNING transactions_locked_through_date`, entityID).Scan(&locked)
	if err != nil {
		return err
	}
	if locked != nil {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			return err
		}
		if !d.After(*locked) {
			return fmt.Errorf("accounting period is locked through %s", locked.Format("2006-01-02"))
		}
	}
	return nil
}
