package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
)

func (s *Store) ListExchangeRates(ctx context.Context,entityID string)([]map[string]any,error){
	rows,err:=s.Pool.Query(ctx,`SELECT public_id::text,rate_date::text,from_currency_code,to_currency_code,rate::text,source,source_reference,created_at FROM exchange_rates WHERE entity_id=$1 ORDER BY rate_date DESC,created_at DESC LIMIT 200`,entityID)
	if err!=nil{return nil,err};defer rows.Close();out:=[]map[string]any{}
	for rows.Next(){var id,date,from,to,rate,source string;var ref *string;var created time.Time;if err:=rows.Scan(&id,&date,&from,&to,&rate,&source,&ref,&created);err!=nil{return nil,err};out=append(out,map[string]any{"id":id,"rate_date":date,"from_currency":from,"to_currency":to,"rate":rate,"source":source,"source_reference":ref,"created_at":created})}
	return out,rows.Err()
}

func (s *Store) CreateExchangeRate(ctx context.Context,user User,e Entity,date,from,to,rate,source,reference string)(map[string]any,error){
	if _,err:=time.Parse("2006-01-02",date);err!=nil{return nil,fmt.Errorf("invalid rate date")}
	if len(from)!=3||len(to)!=3||from==to{return nil,fmt.Errorf("invalid currency pair")}
	if source==""{source="MANUAL"}
	id,_:=ids.UUIDv7();pub,_:=ids.ULID()
	_,err:=s.Pool.Exec(ctx,`INSERT INTO exchange_rates(id,public_id,entity_id,rate_date,from_currency_code,to_currency_code,rate,source,source_reference,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10)`,id,pub,e.ID,date,from,to,rate,source,reference,user.ID)
	if err!=nil{return nil,err}
	_=s.Audit(ctx,user,&e,"EXCHANGE_RATE_CREATE","EXCHANGE_RATE",&pub,"SUCCESS",map[string]any{"rate_date":date,"from":from,"to":to,"rate":rate})
	return map[string]any{"id":pub,"rate_date":date,"from_currency":from,"to_currency":to,"rate":rate,"source":source},nil
}
