package postgres

import (
	"context"
	"time"
)

type AuditEventFilter struct {
	Search  string
	Action  string
	Outcome string
	From    string
	To      string
	Limit   int
	Offset  int
}

type AuditEventList struct {
	Items   []map[string]any `json:"items"`
	Count   int              `json:"count"`
	HasMore bool             `json:"has_more"`
}

func (s *Store) ListAuditEvents(ctx context.Context, entityID string, f AuditEventFilter) (AuditEventList, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	rows, err := s.Pool.Query(ctx, `
WITH filtered AS (
  SELECT ae.public_id,ae.occurred_at,ae.action,ae.resource_type,ae.resource_public_id,
         ae.outcome,ae.source,ae.reason,ae.before_data,ae.after_data,ae.metadata,
         ae.ip_address,ae.user_agent,ae.request_id,
         u.public_id user_public_id,u.display_name,
         COUNT(*) OVER() total_count
  FROM audit_events ae
  LEFT JOIN users u ON u.id=ae.actor_user_id
  WHERE ae.entity_id=$1
    AND ($2='' OR ae.outcome=$2)
    AND ($3='' OR ae.action ILIKE '%'||$3||'%')
    AND ($4='' OR ae.occurred_at>=NULLIF($4,'')::date)
    AND ($5='' OR ae.occurred_at<(NULLIF($5,'')::date + INTERVAL '1 day'))
    AND (
      $6='' OR
      ae.action ILIKE '%'||$6||'%' OR
      COALESCE(ae.resource_type,'') ILIKE '%'||$6||'%' OR
      COALESCE(ae.resource_public_id::text,'') ILIKE '%'||$6||'%' OR
      COALESCE(ae.reason,'') ILIKE '%'||$6||'%' OR
      COALESCE(ae.request_id,'') ILIKE '%'||$6||'%' OR
      COALESCE(u.display_name,'') ILIKE '%'||$6||'%' OR
      COALESCE(u.public_id::text,'') ILIKE '%'||$6||'%'
    )
)
SELECT public_id::text,occurred_at,action,resource_type,resource_public_id::text,
       outcome,source,reason,before_data,after_data,metadata,
       host(ip_address)::text,user_agent,request_id,
       user_public_id::text,display_name,total_count
FROM filtered
ORDER BY occurred_at DESC,public_id DESC
LIMIT $7 OFFSET $8`,
		entityID,f.Outcome,f.Action,f.From,f.To,f.Search,f.Limit,f.Offset)
	if err != nil {
		return AuditEventList{}, err
	}
	defer rows.Close()

	out:=AuditEventList{Items:[]map[string]any{}}
	for rows.Next() {
		var id,action,outcome,source string
		var resourceType,resourceID,reason,ip,userAgent,requestID,userID,userName *string
		var occurred time.Time
		var before,after,metadata any
		var totalCount int
		if err:=rows.Scan(
			&id,&occurred,&action,&resourceType,&resourceID,
			&outcome,&source,&reason,&before,&after,&metadata,
			&ip,&userAgent,&requestID,&userID,&userName,&totalCount,
		);err!=nil{return AuditEventList{},err}
		out.Count=totalCount
		out.Items=append(out.Items,map[string]any{
			"id":id,
			"occurred_at":occurred,
			"action":action,
			"resource_type":resourceType,
			"resource_id":resourceID,
			"outcome":outcome,
			"source":source,
			"reason":reason,
			"before":before,
			"after":after,
			"metadata":metadata,
			"ip_address":ip,
			"user_agent":userAgent,
			"request_id":requestID,
			"actor":map[string]any{"id":userID,"display_name":userName},
		})
	}
	if err:=rows.Err();err!=nil{return AuditEventList{},err}
	out.HasMore=f.Offset+len(out.Items)<out.Count
	return out,nil
}
