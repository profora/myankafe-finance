"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Transaction } from "@/components/types";
import ConfirmDialog from "@/components/ConfirmDialog";

export default function Transactions() {
  const { entity }=useEntity();
  const [items,setItems]=useState<Transaction[]>([]);
  const [error,setError]=useState("");
  const [busy,setBusy]=useState("");
  const [reverseID,setReverseID]=useState("");
  const [reverseReason,setReverseReason]=useState("");
  const [reverseDate,setReverseDate]=useState(()=>new Date().toISOString().slice(0,10));

  const load=()=>{if(entity)api<{items:Transaction[]}>(`/entities/${entity.PublicID}/transactions`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity]);

  async function reverse(){
    if(!entity||!reverseID||!reverseReason.trim())return;
    setBusy(reverseID);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${reverseID}/reverse`,{method:"POST",body:JSON.stringify({reversal_date:reverseDate,reason:reverseReason.trim()})});
      setReverseID("");setReverseReason("");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  async function post(id:string){
    if(!entity)return;
    setBusy(id);setError("");
    try{await api(`/entities/${entity.PublicID}/transactions/${id}/post`,{method:"POST",body:"{}"});load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy("")}
  }

  return <>
    <div className="page-head"><div><h1>Transactions</h1><p>Draft and posted accounting transactions.</p></div><Link className="button" href="/transactions/new">New income / expense</Link></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="table-wrap">{items.length?<table><thead><tr><th>Date</th><th>Type</th><th>Description</th><th>Status</th><th>Amount</th><th></th></tr></thead><tbody>{items.map(t=><tr key={t.PublicID}><td>{t.Date}</td><td>{t.Type}</td><td>{t.Description}</td><td><span className={`badge ${t.Status}`}>{t.Status}</span></td><td>{Number(t.Total).toLocaleString()} {t.Currency}</td><td>{t.Status==="DRAFT"&&<button disabled={busy===t.PublicID} onClick={()=>post(t.PublicID)}>{busy===t.PublicID?"Posting…":"Post"}</button>}{t.Status==="POSTED"&&<button className="danger" disabled={busy===t.PublicID} onClick={()=>{setReverseID(t.PublicID);setReverseReason("");setReverseDate(new Date().toISOString().slice(0,10))}}>Reverse</button>}</td></tr>)}</tbody></table>:<div className="empty">No transactions yet.</div>}</div>
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
