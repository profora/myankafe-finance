package postgres

import (
  "context"
  "fmt"
  "math/big"
  "time"

  "github.com/jackc/pgx/v5"
  "github.com/profora/myankafe-finance/backend/internal/accounting"
  "github.com/profora/myankafe-finance/backend/internal/ids"
)

type SplitInput struct{ AccountPublicID,Amount,Description string }
type CreateTransactionInput struct{ Type,Date,Description,FinancialAccountPublicID,Currency,ContactPublicID string; Splits []SplitInput }
type Transaction struct{ PublicID,Type,Status,Date,Description,Currency,Total string }

type TransactionFilter struct {
  Search string
  Type string
  Status string
  From string
  To string
  Limit int
}

type TransactionCurrencyTotals struct {
  Currency string `json:"currency"`
  Income string `json:"income"`
  Expense string `json:"expense"`
  Net string `json:"net"`
  Count int `json:"count"`
}

type TransactionListResult struct {
  Items []Transaction `json:"items"`
  Totals []TransactionCurrencyTotals `json:"totals"`
}

func (s *Store) ListTransactions(ctx context.Context,entityID string,filter TransactionFilter)(TransactionListResult,error){
  if filter.Limit<=0||filter.Limit>1000{filter.Limit=500}
  rows,err:=s.Pool.Query(ctx,`
SELECT t.public_id::text,t.transaction_type,t.status,t.transaction_date::text,t.description,t.currency_code,t.total_amount::text
FROM transactions t
LEFT JOIN contacts c ON c.id=t.contact_id
LEFT JOIN financial_accounts fa ON fa.id=t.primary_financial_account_id
WHERE t.entity_id=$1
  AND ($2='' OR t.description ILIKE '%'||$2||'%' OR t.public_id::text ILIKE '%'||$2||'%' OR COALESCE(t.external_reference,'') ILIKE '%'||$2||'%' OR COALESCE(c.display_name,'') ILIKE '%'||$2||'%' OR COALESCE(fa.name,'') ILIKE '%'||$2||'%')
  AND ($3='' OR t.transaction_type=$3)
  AND ($4='' OR t.status=$4)
  AND (NULLIF($5,'')::date IS NULL OR t.transaction_date>=NULLIF($5,'')::date)
  AND (NULLIF($6,'')::date IS NULL OR t.transaction_date<=NULLIF($6,'')::date)
ORDER BY t.transaction_date DESC,t.created_at DESC
LIMIT $7`,
    entityID,filter.Search,filter.Type,filter.Status,filter.From,filter.To,filter.Limit)
  if err!=nil{return TransactionListResult{},err}
  defer rows.Close()
  out:=[]Transaction{}
  for rows.Next(){
    var t Transaction
    if err:=rows.Scan(&t.PublicID,&t.Type,&t.Status,&t.Date,&t.Description,&t.Currency,&t.Total);err!=nil{return TransactionListResult{},err}
    out=append(out,t)
  }
  if err:=rows.Err();err!=nil{return TransactionListResult{},err}

  totalRows,err:=s.Pool.Query(ctx,`
SELECT t.currency_code,
       COALESCE(SUM(CASE WHEN t.status='POSTED' AND t.transaction_type='INCOME' THEN t.total_amount ELSE 0 END),0)::text income,
       COALESCE(SUM(CASE WHEN t.status='POSTED' AND t.transaction_type='EXPENSE' THEN t.total_amount ELSE 0 END),0)::text expense,
       COALESCE(SUM(CASE
         WHEN t.status='POSTED' AND t.transaction_type='INCOME' THEN t.total_amount
         WHEN t.status='POSTED' AND t.transaction_type='EXPENSE' THEN -t.total_amount
         ELSE 0
       END),0)::text net,
       count(*)::int
FROM transactions t
LEFT JOIN contacts c ON c.id=t.contact_id
LEFT JOIN financial_accounts fa ON fa.id=t.primary_financial_account_id
WHERE t.entity_id=$1
  AND ($2='' OR t.description ILIKE '%'||$2||'%' OR t.public_id::text ILIKE '%'||$2||'%' OR COALESCE(t.external_reference,'') ILIKE '%'||$2||'%' OR COALESCE(c.display_name,'') ILIKE '%'||$2||'%' OR COALESCE(fa.name,'') ILIKE '%'||$2||'%')
  AND ($3='' OR t.transaction_type=$3)
  AND ($4='' OR t.status=$4)
  AND (NULLIF($5,'')::date IS NULL OR t.transaction_date>=NULLIF($5,'')::date)
  AND (NULLIF($6,'')::date IS NULL OR t.transaction_date<=NULLIF($6,'')::date)
GROUP BY t.currency_code
ORDER BY t.currency_code`,
    entityID,filter.Search,filter.Type,filter.Status,filter.From,filter.To)
  if err!=nil{return TransactionListResult{},err}
  defer totalRows.Close()
  totals:=[]TransactionCurrencyTotals{}
  for totalRows.Next(){
    var v TransactionCurrencyTotals
    if err:=totalRows.Scan(&v.Currency,&v.Income,&v.Expense,&v.Net,&v.Count);err!=nil{return TransactionListResult{},err}
    totals=append(totals,v)
  }
  return TransactionListResult{Items:out,Totals:totals},totalRows.Err()
}

func (s *Store) CreateTransaction(ctx context.Context,user User,e Entity,in CreateTransactionInput)(Transaction,error){
  if in.Type!="INCOME" && in.Type!="EXPENSE"{return Transaction{},fmt.Errorf("only INCOME or EXPENSE accepted by simple entry endpoint")}
  if _,err:=time.Parse("2006-01-02",in.Date);err!=nil{return Transaction{},fmt.Errorf("invalid date")}
  if len(in.Splits)==0{return Transaction{},fmt.Errorf("at least one split is required")}
  tx,err:=s.Pool.Begin(ctx);if err!=nil{return Transaction{},err};defer tx.Rollback(ctx)
  var faID,faCurrency string
  if err:=tx.QueryRow(ctx,`SELECT id::text,currency_code FROM financial_accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`,e.ID,in.FinancialAccountPublicID).Scan(&faID,&faCurrency);err!=nil{return Transaction{},err}
  if in.Currency==""{in.Currency=faCurrency};if in.Currency!=faCurrency{return Transaction{},fmt.Errorf("transaction currency must match selected financial account")}
  var contactID any
  if in.ContactPublicID!=""{
    var cid string
    if err:=tx.QueryRow(ctx,`SELECT id::text FROM contacts WHERE entity_id=$1 AND public_id=$2 AND active=true`,e.ID,in.ContactPublicID).Scan(&cid);err!=nil{return Transaction{},err}
    contactID=cid
  }
  total:=new(big.Rat)
  type resolved struct{ id,amount,desc string }
  rr:=make([]resolved,0,len(in.Splits))
  for _,sp:=range in.Splits{
    a,err:=accounting.ParseAmount(sp.Amount);if err!=nil||a.Sign()<=0{return Transaction{},fmt.Errorf("invalid split amount")}
    total.Add(total,a)
    var aid string;var post bool;var typ string
    if err:=tx.QueryRow(ctx,`SELECT id::text,is_postable,account_type FROM accounts WHERE entity_id=$1 AND public_id=$2 AND active=true`,e.ID,sp.AccountPublicID).Scan(&aid,&post,&typ);err!=nil{return Transaction{},err}
    if !post{return Transaction{},fmt.Errorf("split account is not postable")}
    if in.Type=="EXPENSE" && typ!="EXPENSE"{return Transaction{},fmt.Errorf("expense splits must use EXPENSE accounts")}
    if in.Type=="INCOME" && typ!="INCOME"{return Transaction{},fmt.Errorf("income splits must use INCOME accounts")}
    rr=append(rr,resolved{aid,sp.Amount,sp.Description})
  }
  id,_:=ids.UUIDv7();pub,_:=ids.ULID()
  if _,err:=tx.Exec(ctx,`INSERT INTO transactions(id,public_id,entity_id,transaction_type,status,transaction_date,description,contact_id,primary_financial_account_id,currency_code,total_amount,created_by)
VALUES($1,$2,$3,$4,'DRAFT',$5,$6,$7,$8,$9,$10,$11)`,id,pub,e.ID,in.Type,in.Date,in.Description,contactID,faID,in.Currency,total.FloatString(6),user.ID);err!=nil{return Transaction{},err}
  for i,sp:=range rr{sid,_:=ids.UUIDv7();if _,err:=tx.Exec(ctx,`INSERT INTO transaction_splits(id,transaction_id,line_no,account_id,amount,description) VALUES($1,$2,$3,$4,$5,NULLIF($6,''))`,sid,id,i+1,sp.id,sp.amount,sp.desc);err!=nil{return Transaction{},err}}
  if err:=insertAuditTx(ctx,tx,user,e,"TRANSACTION_CREATE","TRANSACTION",pub,map[string]any{"type":in.Type,"total":total.FloatString(6)});err!=nil{return Transaction{},err}
  if err:=tx.Commit(ctx);err!=nil{return Transaction{},err}
  return Transaction{pub,in.Type,"DRAFT",in.Date,in.Description,in.Currency,total.FloatString(6)},nil
}

func (s *Store) PostTransaction(ctx context.Context,user User,e Entity,pub string)(Transaction,error){
  tx,err:=s.Pool.Begin(ctx);if err!=nil{return Transaction{},err};defer tx.Rollback(ctx)
  var id,typ,date,desc,currency,total,faID,faCOA,faCurrency,status string
  if err:=tx.QueryRow(ctx,`SELECT t.id::text,t.transaction_type,t.status,t.transaction_date::text,t.description,t.currency_code,t.total_amount::text,fa.id::text,fa.account_id::text,fa.currency_code
FROM transactions t JOIN financial_accounts fa ON fa.id=t.primary_financial_account_id
WHERE t.entity_id=$1 AND t.public_id=$2 FOR UPDATE`,e.ID,pub).Scan(&id,&typ,&status,&date,&desc,&currency,&total,&faID,&faCOA,&faCurrency);err!=nil{return Transaction{},err}
  if status!="DRAFT"{return Transaction{},fmt.Errorf("transaction is not draft")}
  if err:=s.EnsureOpenDateTx(ctx,tx,e.ID,date);err!=nil{return Transaction{},err}
  rate:=new(big.Rat).SetInt64(1);var rateID any
  if currency!=e.FunctionalCurrency{
    var rs,id string
    err:=tx.QueryRow(ctx,`SELECT rate::text,id::text FROM exchange_rates WHERE entity_id=$1 AND from_currency_code=$2 AND to_currency_code=$3 AND rate_date<=$4 ORDER BY rate_date DESC,created_at DESC LIMIT 1`,e.ID,currency,e.FunctionalCurrency,date).Scan(&rs,&id)
    if err!=nil{return Transaction{},fmt.Errorf("exchange rate missing: %w",err)}
    rate,_=new(big.Rat).SetString(rs);rateID=id
  }
  rows,err:=tx.Query(ctx,`SELECT ts.line_no,a.id::text,ts.amount::text,COALESCE(ts.description,'') FROM transaction_splits ts JOIN accounts a ON a.id=ts.account_id WHERE ts.transaction_id=$1 ORDER BY ts.line_no`,id);if err!=nil{return Transaction{},err}
  type sp struct{n int; aid,amount,desc string};splits:=[]sp{}
  for rows.Next(){var x sp;if err:=rows.Scan(&x.n,&x.aid,&x.amount,&x.desc);err!=nil{rows.Close();return Transaction{},err};splits=append(splits,x)};rows.Close()
  jid,_:=ids.UUIDv7();jpub,_:=ids.ULID()
  if _,err:=tx.Exec(ctx,`INSERT INTO journal_entries(id,public_id,entity_id,transaction_id,journal_date,description,status,functional_currency_code,created_by) VALUES($1,$2,$3,$4,$5,$6,'DRAFT',$7,$8)`,jid,jpub,e.ID,id,date,desc,e.FunctionalCurrency,user.ID);err!=nil{return Transaction{},err}
  lineNo:=1
  totalFunctional:=new(big.Rat)
  for _,x:=range splits{
    amt,_:=accounting.ParseAmount(x.amount); f:=new(big.Rat).Mul(amt,rate); roundedFunctional:=f.FloatString(6)
    roundedRat,_:=accounting.ParseAmount(roundedFunctional); totalFunctional.Add(totalFunctional,roundedRat)
    lid,_:=ids.UUIDv7(); td,tc,fd,fc:="0","0","0","0"
    if typ=="EXPENSE"{td=amt.FloatString(6);fd=roundedFunctional}else{tc=amt.FloatString(6);fc=roundedFunctional}
    if _,err:=tx.Exec(ctx,`INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,$12,$13,$14)`,lid,jid,e.ID,lineNo,x.aid,x.desc,currency,td,tc,e.FunctionalCurrency,rate.FloatString(12),fd,fc,rateID);err!=nil{return Transaction{},err};lineNo++
  }
  lid,_:=ids.UUIDv7();td,tc,fd,fc:="0","0","0","0";native,_:=accounting.ParseAmount(total)
  if typ=="EXPENSE"{tc=native.FloatString(6);fc=totalFunctional.FloatString(6)}else{td=native.FloatString(6);fd=totalFunctional.FloatString(6)}
  if _,err:=tx.Exec(ctx,`INSERT INTO journal_lines(id,journal_entry_id,entity_id,line_no,account_id,financial_account_id,description,transaction_currency_code,transaction_debit_amount,transaction_credit_amount,functional_currency_code,fx_rate_to_functional,debit_amount,credit_amount,exchange_rate_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,lid,jid,e.ID,lineNo,faCOA,faID,desc,currency,td,tc,e.FunctionalCurrency,rate.FloatString(12),fd,fc,rateID);err!=nil{return Transaction{},err}
  var debits,credits string
  if err:=tx.QueryRow(ctx,`SELECT COALESCE(sum(debit_amount),0)::text,COALESCE(sum(credit_amount),0)::text FROM journal_lines WHERE journal_entry_id=$1`,jid).Scan(&debits,&credits);err!=nil{return Transaction{},err}
  d,_:=accounting.ParseAmount(debits);c,_:=accounting.ParseAmount(credits);if d.Cmp(c)!=0{return Transaction{},fmt.Errorf("generated journal is unbalanced")}
  now:=time.Now().UTC()
  if _,err:=tx.Exec(ctx,`UPDATE journal_entries SET status='POSTED',posted_by=$2,posted_at=$3 WHERE id=$1`,jid,user.ID,now);err!=nil{return Transaction{},err}
  if _,err:=tx.Exec(ctx,`UPDATE transactions SET status='POSTED',posted_by=$2,posted_at=$3,updated_at=$3 WHERE id=$1`,id,user.ID,now);err!=nil{return Transaction{},err}
  if err:=insertAuditTx(ctx,tx,user,e,"TRANSACTION_POST","TRANSACTION",pub,map[string]any{"journal_id":jpub});err!=nil{return Transaction{},err}
  if err:=tx.Commit(ctx);err!=nil{return Transaction{},err}
  return Transaction{pub,typ,"POSTED",date,desc,currency,total},nil
}

func insertAuditTx(ctx context.Context,tx pgx.Tx,user User,e Entity,action,resource,pub string,after map[string]any)error{
  id,_:=ids.UUIDv7();apub,_:=ids.ULID()
  _,err:=tx.Exec(ctx,`INSERT INTO audit_events(id,public_id,actor_type,actor_user_id,entity_id,action,resource_type,resource_public_id,outcome,source,request_id,after_data)
VALUES($1,$2,'USER',$3,$4,$5,$6,$7,'SUCCESS','API',$8,$9)`,id,apub,user.ID,e.ID,action,resource,pub,fmt.Sprintf("req-%d",time.Now().UnixNano()),after)
  return err
}
