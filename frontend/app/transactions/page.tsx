"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import type { FinancialAccount } from "@/components/types";
import ConfirmDialog from "@/components/ConfirmDialog";

type TransactionRow={
  id:string;
  type:string;
  status:"DRAFT"|"POSTED"|"VOIDED";
  date:string;
  description:string;
  currency:string;
  total:string;
  contact_name?:string|null;
  financial_account_id?:string|null;
  financial_account_name?:string|null;
  functional_effect:string;
  running_net:string;
};

type Result={
  items:TransactionRow[];
  count:number;
  income_total:string;
  expense_total:string;
  net_total:string;
  functional_currency:string;
  has_more:boolean;
};

const pageSize=100;

export default function Transactions() {
  const {entity}=useEntity();
  const [result,setResult]=useState<Result>({
    items:[],count:0,income_total:"0",expense_total:"0",net_total:"0",functional_currency:"MMK",has_more:false,
  });
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(false);
  const [busy,setBusy]=useState("");
  const [reverseID,setReverseID]=useState("");
  const [reverseReason,setReverseReason]=useState("");
  const [reverseDate,setReverseDate]=useState(()=>dateInTimeZone(entity?.Timezone??"Asia/Yangon"));
  const [search,setSearch]=useState("");
  const [status,setStatus]=useState("");
  const [type,setType]=useState("");
  const [from,setFrom]=useState("");
  const [to,setTo]=useState("");
  const [financialAccount,setFinancialAccount]=useState("");
  const [offset,setOffset]=useState(0);
  const [debouncedSearch,setDebouncedSearch]=useState("");

  useEffect(()=>{
    const timer=setTimeout(()=>setDebouncedSearch(search.trim()),300);
    return()=>clearTimeout(timer);
  },[search]);

  useEffect(()=>{setOffset(0)},[debouncedSearch,status,type,from,to,financialAccount,entity?.PublicID]);

  const query=useMemo(()=>{
    const p=new URLSearchParams();
    if(debouncedSearch)p.set("q",debouncedSearch);
    if(status)p.set("status",status);
    if(type)p.set("type",type);
    if(from)p.set("from",from);
    if(to)p.set("to",to);
    if(financialAccount)p.set("financial_account_id",financialAccount);
    p.set("limit",String(pageSize));
    p.set("offset",String(offset));
    return p.toString();
  },[debouncedSearch,status,type,from,to,financialAccount,offset]);

  async function load(){
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const data=await api<Result>(`/entities/${entity.PublicID}/transactions?${query}`);
      setResult(data);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{load()},[entity?.PublicID,query]);

  useEffect(()=>{
    if(!entity)return;
    api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`)
      .then(x=>setFinancial(x.items))
      .catch(()=>{});
  },[entity?.PublicID]);

  function clearFilters(){
    setSearch("");setStatus("");setType("");setFrom("");setTo("");setFinancialAccount("");setOffset(0);
  }

  async function reverse(){
    if(!entity||!reverseID||!reverseReason.trim())return;
    setBusy(reverseID);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${reverseID}/reverse`,{
        method:"POST",
        body:JSON.stringify({reversal_date:reverseDate,reason:reverseReason.trim()}),
      });
      setReverseID("");setReverseReason("");await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  async function post(id:string){
    if(!entity)return;
    setBusy(id);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${id}/post`,{method:"POST",body:"{}"});
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  const currency=result.functional_currency||entity?.FunctionalCurrency||"MMK";
  const first=result.count===0?0:offset+1;
  const last=Math.min(offset+result.items.length,result.count);

  return <>
    <div className="page-head">
      <div><h1>Transactions</h1><p>Searchable accounting activity with running totals in {currency}.</p></div>
      <details className="action-menu">
        <summary className="button">New transaction ▾</summary>
        <div className="action-menu-popover">
          <Link href="/transactions/new?type=INCOME"><strong>Income</strong><span>Receive money</span></Link>
          <Link href="/transactions/new?type=EXPENSE"><strong>Expense</strong><span>Pay or spend money</span></Link>
          <Link href="/transfers"><strong>Transfer</strong><span>Move between cash/bank accounts</span></Link>
          <Link href="/manual-journal"><strong>Manual journal</strong><span>Debit / credit adjustment</span></Link>
          <Link href="/inter-entity"><strong>Inter-entity</strong><span>Due-to / due-from transaction</span></Link>
        </div>
      </details>
    </div>

    {error&&<div className="alert error">{error}</div>}

    <div className="grid cards transaction-summary">
      <div className="card"><div className="muted">Income</div><div className="metric">{Number(result.income_total).toLocaleString()} {currency}</div></div>
      <div className="card"><div className="muted">Expenses</div><div className="metric">{Number(result.expense_total).toLocaleString()} {currency}</div></div>
      <div className="card"><div className="muted">Net effect</div><div className="metric">{Number(result.net_total).toLocaleString()} {currency}</div></div>
      <div className="card"><div className="muted">Matched transactions</div><div className="metric">{result.count.toLocaleString()}</div></div>
    </div>

    <div className="card transaction-filters">
      <div className="transaction-filter-grid">
        <div className="field transaction-search"><label>Search</label><input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Description, contact, account, reference or ULID"/></div>
        <div className="field"><label>Status</label><select value={status} onChange={e=>setStatus(e.target.value)}><option value="">All</option><option>DRAFT</option><option>POSTED</option><option>VOIDED</option></select></div>
        <div className="field"><label>Type</label><select value={type} onChange={e=>setType(e.target.value)}><option value="">All</option><option>INCOME</option><option>EXPENSE</option><option>ACCOUNT_TRANSFER</option><option>INTER_ENTITY</option><option>MANUAL_JOURNAL</option><option>ADJUSTMENT</option><option>REVERSAL</option></select></div>
        <div className="field"><label>Financial account</label><select value={financialAccount} onChange={e=>setFinancialAccount(e.target.value)}><option value="">All</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>From</label><input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></div>
        <div className="field"><label>To</label><input type="date" value={to} onChange={e=>setTo(e.target.value)}/></div>
        <div className="actions transaction-filter-actions"><button className="secondary" onClick={clearFilters}>Clear filters</button>{loading&&<span className="muted">Refreshing…</span>}</div>
      </div>
    </div>

    <div className="table-wrap transaction-table">
      {result.items.length?<table>
        <thead><tr><th>Date</th><th>Type</th><th>Description</th><th>Account / Contact</th><th>Status</th><th>Transaction amount</th><th>Functional effect</th><th>Running net</th><th></th></tr></thead>
        <tbody>{result.items.map(t=><tr key={t.id}>
          <td>{t.date}</td>
          <td><span className="badge">{t.type.replaceAll("_"," ")}</span></td>
          <td><Link className="table-link transaction-description" href={`/transactions/${t.id}`}>{t.description}</Link></td>
          <td><div>{t.financial_account_name??"—"}</div>{t.contact_name&&<div className="muted">{t.contact_name}</div>}</td>
          <td><span className={`badge ${t.status}`}>{t.status}</span></td>
          <td>{Number(t.total).toLocaleString()} {t.currency}</td>
          <td className={Number(t.functional_effect)<0?"amount-negative":Number(t.functional_effect)>0?"amount-positive":""}>{Number(t.functional_effect).toLocaleString()} {currency}</td>
          <td><strong>{Number(t.running_net).toLocaleString()} {currency}</strong></td>
          <td><div className="actions table-actions">
            {t.status==="DRAFT"&&<button disabled={busy===t.id} onClick={()=>post(t.id)}>{busy===t.id?"Posting…":"Post"}</button>}
            {t.status==="POSTED"&&<button className="danger" disabled={busy===t.id} onClick={()=>{setReverseID(t.id);setReverseReason("");setReverseDate(dateInTimeZone(entity?.Timezone??"Asia/Yangon"))}}>Reverse</button>}
          </div></td>
        </tr>)}</tbody>
      </table>:<div className="empty">{loading?"Loading transactions…":"No transactions match these filters."}</div>}
    </div>

    <div className="list-pagination">
      <span className="muted">{result.count?`Showing ${first}–${last} of ${result.count}`:"No results"}</span>
      <div className="actions">
        <button className="secondary" disabled={offset===0||loading} onClick={()=>setOffset(Math.max(0,offset-pageSize))}>Previous</button>
        <button className="secondary" disabled={!result.has_more||loading} onClick={()=>setOffset(offset+pageSize)}>Next</button>
      </div>
    </div>

    <ConfirmDialog
      open={Boolean(reverseID)}
      title="Reverse posted transaction?"
      description="This does not edit or delete the original. A new opposite journal will be posted and the original will remain in the audit trail."
      confirmLabel="Post reversal"
      danger
      busy={Boolean(busy)}
      onCancel={()=>{if(!busy){setReverseID("");setReverseReason("")}}}
      onConfirm={reverse}
    >
      <div className="form" style={{marginTop:16}}>
        <div className="field"><label>Reversal date</label><input type="date" value={reverseDate} onChange={e=>setReverseDate(e.target.value)}/></div>
        <div className="field"><label>Reason</label><textarea autoFocus rows={3} value={reverseReason} onChange={e=>setReverseReason(e.target.value)} placeholder="Why is this transaction being reversed?"/></div>
        {!reverseReason.trim()&&<div className="muted">A reason is required for the audit trail.</div>}
      </div>
    </ConfirmDialog>
  </>;
}
