package postgres

import (
	"context"
	"time"
)

func (s *Store) ListAuditEvents(ctx context.Context, entityID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.Pool.Query(ctx, `
SELECT ae.public_id::text,ae.occurred_at,ae.action,ae.resource_type,ae.resource_public_id::text,
       ae.outcome,ae.source,ae.reason,ae.after_data,
       u.public_id::text,u.display_name
FROM audit_events ae
LEFT JOIN users u ON u.id=ae.actor_user_id
WHERE ae.entity_id=$1
ORDER BY ae.occurred_at DESC
LIMIT $2`, entityID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id,action,outcome,source string
		var resourceType,resourceID,reason,userID,userName *string
		var occurred time.Time
		var after any
		if err:=rows.Scan(&id,&occurred,&action,&resourceType,&resourceID,&outcome,&source,&reason,&after,&userID,&userName);err!=nil{return nil,err}
		out=append(out,map[string]any{
			"id":id,"occurred_at":occurred,"action":action,"resource_type":resourceType,"resource_id":resourceID,
			"outcome":outcome,"source":source,"reason":reason,"after":after,
			"actor":map[string]any{"id":userID,"display_name":userName},
		})
	}
	return out,rows.Err()
}
