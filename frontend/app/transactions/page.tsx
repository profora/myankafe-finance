"use client";

import Link from "next/link";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { api, apiBlob } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import type { FinancialAccount } from "@/components/types";
import ConfirmDialog from "@/components/ConfirmDialog";
import TransactionEntryModal, { type QuickEntryKind } from "@/components/TransactionEntryModal";
import { canCorrectPostedAccounting, canOperateLedger } from "@/lib/permissions";

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
  attachment_count:number;
};

type TransactionList={
  items:TransactionRow[];
  count:number;
  income_total:string;
  expense_total:string;
  net_total:string;
  functional_currency:string;
  has_more:boolean;
};

type Filters={
  q:string;
  status:string;
  type:string;
  from:string;
  to:string;
  financial_account_id:string;
};

const emptyFilters:Filters={q:"",status:"",type:"",from:"",to:"",financial_account_id:""};

function signedClass(v:string){
  const n=Number(v);
  return n>0?"money-positive":n<0?"money-negative":"";
}

export default function Transactions(){
  const {entity}=useEntity();
  const mayOperate=canOperateLedger(entity?.Role);
  const mayReverse=canCorrectPostedAccounting(entity?.Role);
  const [items,setItems]=useState<TransactionRow[]>([]);
  const [summary,setSummary]=useState<Omit<TransactionList,"items"|"has_more">>({count:0,income_total:"0",expense_total:"0",net_total:"0",functional_currency:"MMK"});
  const [hasMore,setHasMore]=useState(false);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [filters,setFilters]=useState<Filters>(emptyFilters);
  const [applied,setApplied]=useState<Filters>(emptyFilters);
  const [loading,setLoading]=useState(false);
  const [error,setError]=useState("");
  const [busy,setBusy]=useState("");
  const [entryKind,setEntryKind]=useState<QuickEntryKind|null>(null);
  const [reverseID,setReverseID]=useState("");
  const [reverseReason,setReverseReason]=useState("");
  const [reverseDate,setReverseDate]=useState(()=>dateInTimeZone(entity?.Timezone??"Asia/Yangon"));

  const query=useMemo(()=>{
    const q=new URLSearchParams();
    Object.entries(applied).forEach(([k,v])=>{if(v)q.set(k,v)});
    q.set("limit","100");
    q.set("offset","0");
    return q;
  },[applied]);

  async function load(reset=true){
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const offset=reset?0:items.length;
      const q=new URLSearchParams(query);
      q.set("offset",String(offset));
      const result=await api<TransactionList>(`/entities/${entity.PublicID}/transactions?${q.toString()}`);
      setItems(current=>reset?result.items:[...current,...result.items]);
      setSummary({
        count:result.count,
        income_total:result.income_total,
        expense_total:result.expense_total,
        net_total:result.net_total,
        functional_currency:result.functional_currency
      });
      setHasMore(result.has_more);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{
    if(!entity)return;
    setFilters(emptyFilters);setApplied(emptyFilters);setItems([]);
    Promise.all([
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
      api<TransactionList>(`/entities/${entity.PublicID}/transactions?limit=100&offset=0`)
    ]).then(([f,result])=>{
      setFinancial(f.items);
      setItems(result.items);
      setSummary({count:result.count,income_total:result.income_total,expense_total:result.expense_total,net_total:result.net_total,functional_currency:result.functional_currency});
      setHasMore(result.has_more);
    }).catch(e=>setError(e instanceof Error?e.message:String(e)));
  },[entity?.PublicID]);

  async function exportCSV(){
    if(!entity)return;
    setError("");
    try{
      const q=new URLSearchParams(query);
      q.delete("limit");q.delete("offset");
      const blob=await apiBlob(`/entities/${entity.PublicID}/transactions/export.csv?${q.toString()}`);
      const href=URL.createObjectURL(blob);
      const a=document.createElement("a");
      a.href=href;
      a.download=`${entity.Code.toLowerCase()}-transactions.csv`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(href);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  async function post(id:string){
    if(!entity)return;
    setBusy(id);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${id}/post`,{method:"POST",body:"{}"});
      await load(true);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  async function reverse(){
    if(!entity||!reverseID||!reverseReason.trim())return;
    setBusy(reverseID);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${reverseID}/reverse`,{method:"POST",body:JSON.stringify({reversal_date:reverseDate,reason:reverseReason.trim()})});
      setReverseID("");setReverseReason("");
      await load(true);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  function applyFilters(e?:FormEvent){
    e?.preventDefault();
    setApplied({...filters});
  }

  function clearFilters(){
    setFilters(emptyFilters);setApplied(emptyFilters);
  }

  useEffect(()=>{if(entity)load(true)},[query.toString()]);

  return <>
    <div className="page-head transaction-head">
      <div>
        <h1>Transactions</h1>
        <p>Search, filter and enter daily transactions. Running totals are in the entity functional currency.</p>
      </div>
      {mayOperate&&<div className="entry-actions">
        <details className="entry-menu">
          <summary className="button">+ Add transaction <span aria-hidden>▾</span></summary>
          <div className="entry-menu-panel">
            <button type="button" onClick={()=>setEntryKind("INCOME")}><strong>Income</strong><span>Money received</span></button>
            <button type="button" onClick={()=>setEntryKind("EXPENSE")}><strong>Expense</strong><span>Money paid</span></button>
            <button type="button" onClick={()=>setEntryKind("TRANSFER")}><strong>Transfer</strong><span>Cash / bank / wallet move</span></button>
            <div className="menu-separator"/>
            <Link href="/inter-entity"><strong>Inter-Entity</strong><span>Pay or move value across entities</span></Link>
            <Link href="/manual-journal"><strong>Manual Journal</strong><span>Advanced debit / credit entry</span></Link>
          </div>
        </details>
      </div>}
    </div>

    {error&&<div className="alert error">{error}</div>}

    <form className="card transaction-filters" onSubmit={applyFilters}>
      <div className="transaction-filter-grid">
        <div className="field filter-search"><label>Search</label><input value={filters.q} onChange={e=>setFilters({...filters,q:e.target.value})} placeholder="Description, ID, contact, account…"/></div>
        <div className="field"><label>Type</label><select value={filters.type} onChange={e=>setFilters({...filters,type:e.target.value})}><option value="">All types</option>{["INCOME","EXPENSE","ACCOUNT_TRANSFER","INTER_ENTITY","MANUAL_JOURNAL","ADJUSTMENT","REVERSAL"].map(x=><option key={x} value={x}>{x.replaceAll("_"," ")}</option>)}</select></div>
        <div className="field"><label>Status</label><select value={filters.status} onChange={e=>setFilters({...filters,status:e.target.value})}><option value="">All statuses</option><option>DRAFT</option><option>POSTED</option><option>VOIDED</option></select></div>
        <div className="field"><label>Financial account</label><select value={filters.financial_account_id} onChange={e=>setFilters({...filters,financial_account_id:e.target.value})}><option value="">All accounts</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>From</label><input type="date" value={filters.from} onChange={e=>setFilters({...filters,from:e.target.value})}/></div>
        <div className="field"><label>To</label><input type="date" value={filters.to} onChange={e=>setFilters({...filters,to:e.target.value})}/></div>
      </div>
      <div className="actions">
        <button type="submit" disabled={loading}>Apply filters</button>
        <button type="button" className="secondary" onClick={clearFilters}>Clear</button>
        <button type="button" className="secondary" disabled={loading||summary.count===0} onClick={exportCSV}>Export CSV</button>
        <span className="muted">{summary.count.toLocaleString()} matching transaction{summary.count===1?"":"s"}</span>
      </div>
    </form>

    <div className="transaction-summary">
      <div className="card"><div className="muted">Posted income</div><div className="metric money-positive">{Number(summary.income_total).toLocaleString()} <small>{summary.functional_currency}</small></div></div>
      <div className="card"><div className="muted">Posted expenses</div><div className="metric money-negative">{Number(summary.expense_total).toLocaleString()} <small>{summary.functional_currency}</small></div></div>
      <div className="card"><div className="muted">Running net</div><div className={`metric ${signedClass(summary.net_total)}`}>{Number(summary.net_total).toLocaleString()} <small>{summary.functional_currency}</small></div></div>
    </div>

    <div className="table-wrap transaction-table">
      {items.length?<table>
        <thead><tr><th>Date</th><th>Type</th><th>Description</th><th>Account / Contact</th><th>Status</th><th>Amount</th><th>Functional effect</th><th>Running net</th><th></th></tr></thead>
        <tbody>{items.map(t=><tr key={t.id}>
          <td>{t.date}</td>
          <td><span className="type-pill">{t.type.replaceAll("_"," ")}</span></td>
          <td><Link className="table-link transaction-description" href={`/transactions/${t.id}`}>{t.description}</Link>{t.attachment_count>0&&<div className="muted">📎 {t.attachment_count} attachment{t.attachment_count===1?"":"s"}</div>}</td>
          <td><div>{t.financial_account_name||"—"}</div>{t.contact_name&&<div className="muted">{t.contact_name}</div>}</td>
          <td><span className={`badge ${t.status}`}>{t.status}</span></td>
          <td>{Number(t.total).toLocaleString()} {t.currency}</td>
          <td className={signedClass(t.functional_effect)}>{Number(t.functional_effect).toLocaleString()} {summary.functional_currency}</td>
          <td className={signedClass(t.running_net)}><strong>{Number(t.running_net).toLocaleString()}</strong> {summary.functional_currency}</td>
          <td><div className="actions compact-actions">
            {mayOperate&&t.status==="DRAFT"&&<button disabled={busy===t.id} onClick={()=>post(t.id)}>{busy===t.id?"Posting…":"Post"}</button>}
            {mayReverse&&t.status==="POSTED"&&<button className="danger" disabled={busy===t.id} onClick={()=>{setReverseID(t.id);setReverseReason("");setReverseDate(dateInTimeZone(entity?.Timezone??"Asia/Yangon"))}}>Reverse</button>}
          </div></td>
        </tr>)}</tbody>
      </table>:<div className="empty">{loading?"Loading transactions…":"No transactions match these filters."}</div>}
    </div>

    {hasMore&&<div className="load-more"><button className="secondary" disabled={loading} onClick={()=>load(false)}>{loading?"Loading…":"Load more"}</button></div>}

    {mayOperate&&<TransactionEntryModal open={Boolean(entryKind)} kind={entryKind} entity={entity??null} onClose={()=>setEntryKind(null)} onSaved={()=>load(true)}/>} 

    <ConfirmDialog
      open={Boolean(reverseID)}
      title="Reverse posted transaction?"
      description="This does not edit or delete the original. A new opposite journal will be posted and the original remains in the audit trail."
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
