package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

type LockState struct {
	LockedThrough *time.Time `json:"locked_through"`
	Reason        *string    `json:"reason"`
}

func (s *Store) GetLock(ctx context.Context, entityID string) (LockState, error) {
	var x LockState
	err := s.Pool.QueryRow(ctx, `SELECT transactions_locked_through_date,lock_reason FROM entity_accounting_controls WHERE entity_id=$1`, entityID).Scan(&x.LockedThrough, &x.Reason)
	if err != nil {
		return LockState{}, err
	}
	return x, nil
}

func (s *Store) Lock(ctx context.Context, user User, e Entity, date, reason string) error {
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid lock date")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var prev *time.Time
	err = tx.QueryRow(ctx, `INSERT INTO entity_accounting_controls(entity_id) VALUES($1) ON CONFLICT(entity_id) DO UPDATE SET entity_id=EXCLUDED.entity_id RETURNING transactions_locked_through_date`, e.ID).Scan(&prev)
	if err != nil {
		return err
	}
	if prev != nil && d.Before(*prev) {
		return fmt.Errorf("new lock cannot move backwards; owner must unlock first")
	}
	if _, err = tx.Exec(ctx, `UPDATE entity_accounting_controls SET transactions_locked_through_date=$2,locked_by=$3,locked_at=now(),lock_reason=$4,updated_at=now() WHERE entity_id=$1`, e.ID, date, user.ID, reason); err != nil {
		return err
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	if _, err = tx.Exec(ctx, `INSERT INTO accounting_lock_events(id,public_id,entity_id,action,previous_lock_date,new_lock_date,reason,actor_user_id) VALUES($1,$2,$3,'LOCK',$4,$5,$6,$7)`, id, pub, e.ID, prev, date, reason, user.ID); err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, user, e, "PERIOD_LOCK", "ENTITY", e.PublicID, map[string]any{"locked_through": date, "reason": reason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Unlock(ctx context.Context, user User, e Entity, role, reason string) error {
	if role != "OWNER" {
		return fmt.Errorf("only OWNER can unlock a period")
	}
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var prev *time.Time
	if err = tx.QueryRow(ctx, `SELECT transactions_locked_through_date FROM entity_accounting_controls WHERE entity_id=$1 FOR UPDATE`, e.ID).Scan(&prev); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE entity_accounting_controls SET transactions_locked_through_date=NULL,locked_by=NULL,locked_at=NULL,lock_reason=NULL,updated_at=now() WHERE entity_id=$1`, e.ID); err != nil {
		return err
	}
	id, _ := ids.UUIDv7()
	pub, _ := ids.ULID()
	if _, err = tx.Exec(ctx, `INSERT INTO accounting_lock_events(id,public_id,entity_id,action,previous_lock_date,new_lock_date,reason,actor_user_id) VALUES($1,$2,$3,'UNLOCK',$4,NULL,$5,$6)`, id, pub, e.ID, prev, reason, user.ID); err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, user, e, "PERIOD_UNLOCK", "ENTITY", e.PublicID, map[string]any{"previous_lock": prev, "reason": reason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
