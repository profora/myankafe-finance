"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type JournalLine={
  line_no:number;
  account:{id:string;code:string;name:string};
  financial_account?:{id?:string|null;name?:string|null};
  description:string;
  transaction_currency:string;
  transaction_debit:string;
  transaction_credit:string;
  functional_currency:string;
  fx_rate:string;
  debit:string;
  credit:string;
  exchange_rate_id?:string|null;
};
type Journal={
  id:string;
  date:string;
  description:string;
  status:string;
  functional_currency:string;
  posted_at?:string|null;
  lines:JournalLine[];
};
type Detail={
  id:string;
  type:string;
  status:string;
  date:string;
  description:string;
  currency:string;
  total:string;
  contact?:{id?:string|null;name?:string|null};
  financial_account?:{id?:string|null;name?:string|null};
  original_transaction_id?:string|null;
  reversal_transaction_id?:string|null;
  posted_at?:string|null;
  voided_at?:string|null;
  void_reason?:string|null;
  splits:{line_no:number;account_id:string;account_code:string;account_name:string;amount:string;description:string}[];
  journals:Journal[];
};

export default function TransactionDetailPage(){
  const {entity}=useEntity();
  const params=useParams<{id:string}>();
  const id=Array.isArray(params.id)?params.id[0]:params.id;
  const [detail,setDetail]=useState<Detail|null>(null);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(false);

  useEffect(()=>{
    if(!entity||!id)return;
    setLoading(true);setError("");
    api<Detail>(`/entities/${entity.PublicID}/transactions/${id}`)
      .then(setDetail)
      .catch(e=>setError(e.message))
      .finally(()=>setLoading(false));
  },[entity,id]);

  if(loading&&!detail)return <div className="empty">Loading transaction…</div>;

  return <>
    <div className="page-head">
      <div>
        <div className="muted"><Link href="/transactions">Transactions</Link> / Detail</div>
        <h1>{detail?.description??"Transaction"}</h1>
        {detail&&<p>{detail.date} · {detail.type} · <span className={`badge ${detail.status}`}>{detail.status}</span></p>}
      </div>
    </div>
    {error&&<div className="alert error">{error}</div>}
    {detail&&<>
      <div className="grid cards">
        <div className="card"><div className="muted">Amount</div><div className="metric">{Number(detail.total).toLocaleString()} {detail.currency}</div></div>
        <div className="card"><div className="muted">Financial account</div><div style={{fontWeight:750,marginTop:8}}>{detail.financial_account?.name??"—"}</div></div>
        <div className="card"><div className="muted">Contact</div><div style={{fontWeight:750,marginTop:8}}>{detail.contact?.name??"—"}</div></div>
        <div className="card"><div className="muted">Public ID</div><div style={{fontFamily:"monospace",fontSize:12,marginTop:8}}>{detail.id}</div></div>
      </div>

      {(detail.original_transaction_id||detail.reversal_transaction_id||detail.void_reason)&&<div className="card" style={{marginTop:16}}>
        <h3>Correction history</h3>
        {detail.original_transaction_id&&<p>Reverses: <Link className="table-link" href={`/transactions/${detail.original_transaction_id}`}>{detail.original_transaction_id}</Link></p>}
        {detail.reversal_transaction_id&&<p>Reversal: <Link className="table-link" href={`/transactions/${detail.reversal_transaction_id}`}>{detail.reversal_transaction_id}</Link></p>}
        {detail.void_reason&&<p><strong>Reason:</strong> {detail.void_reason}</p>}
      </div>}

      {detail.splits.length>0&&<>
        <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Entry splits</h1></div></div>
        <div className="table-wrap"><table><thead><tr><th>#</th><th>Account</th><th>Description</th><th>Amount</th></tr></thead><tbody>{detail.splits.map(x=><tr key={x.line_no}><td>{x.line_no}</td><td><Link className="table-link" href={`/accounts/${x.account_id}/ledger`}>{x.account_code} · {x.account_name}</Link></td><td>{x.description||"—"}</td><td>{Number(x.amount).toLocaleString()} {detail.currency}</td></tr>)}</tbody></table></div>
      </>}

      {detail.journals.map(j=><section key={j.id}>
        <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Journal · {j.status}</h1><p>{j.date} · {j.id}</p></div></div>
        <div className="table-wrap"><table><thead><tr><th>#</th><th>Account</th><th>Description</th><th>Transaction debit</th><th>Transaction credit</th><th>FX</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{j.lines.map(x=><tr key={x.line_no}>
          <td>{x.line_no}</td>
          <td><Link className="table-link" href={`/accounts/${x.account.id}/ledger`}>{x.account.code} · {x.account.name}</Link>{x.financial_account?.name&&<div className="muted">{x.financial_account.name}</div>}</td>
          <td>{x.description||"—"}</td>
          <td>{Number(x.transaction_debit).toLocaleString()} {x.transaction_currency}</td>
          <td>{Number(x.transaction_credit).toLocaleString()} {x.transaction_currency}</td>
          <td>{x.fx_rate}</td>
          <td>{Number(x.debit).toLocaleString()} {x.functional_currency}</td>
          <td>{Number(x.credit).toLocaleString()} {x.functional_currency}</td>
        </tr>)}</tbody></table></div>
      </section>)}
    </>}
  </>;
}
