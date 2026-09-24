"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Transaction } from "@/components/types";

export default function Transactions() {
  const { entity }=useEntity();
  const [items,setItems]=useState<Transaction[]>([]);
  const [error,setError]=useState("");
  const [busy,setBusy]=useState("");

  const load=()=>{if(entity)api<{items:Transaction[]}>(`/entities/${entity.PublicID}/transactions`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity]);

  async function reverse(id:string){
    if(!entity)return;
    const reason=window.prompt("Reason for reversal");
    if(!reason)return;
    const reversal_date=new Date().toISOString().slice(0,10);
    setBusy(id);setError("");
    try{await api(`/entities/${entity.PublicID}/transactions/${id}/reverse`,{method:"POST",body:JSON.stringify({reversal_date,reason})});load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
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
    <div className="table-wrap">{items.length?<table><thead><tr><th>Date</th><th>Type</th><th>Description</th><th>Status</th><th>Amount</th><th></th></tr></thead><tbody>{items.map(t=><tr key={t.PublicID}><td>{t.Date}</td><td>{t.Type}</td><td>{t.Description}</td><td><span className={`badge ${t.Status}`}>{t.Status}</span></td><td>{Number(t.Total).toLocaleString()} {t.Currency}</td><td>{t.Status==="DRAFT"&&<button disabled={busy===t.PublicID} onClick={()=>post(t.PublicID)}>{busy===t.PublicID?"Posting…":"Post"}</button>}{t.Status==="POSTED"&&<button className="danger" disabled={busy===t.PublicID} onClick={()=>reverse(t.PublicID)}>Reverse</button>}</td></tr>)}</tbody></table>:<div className="empty">No transactions yet.</div>}</div>
  </>;
}
