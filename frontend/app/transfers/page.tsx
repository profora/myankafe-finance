"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import type { FinancialAccount } from "@/components/types";

export default function Transfers(){
  const {entity}=useEntity();
  const [accounts,setAccounts]=useState<FinancialAccount[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [form,setForm]=useState({Date:dateInTimeZone(entity?.Timezone??"Asia/Yangon"),FromFinancialAccountPublicID:"",ToFinancialAccountPublicID:"",FromAmount:"",ToAmount:"",Description:""});

  useEffect(()=>{if(entity)api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`).then(x=>{setAccounts(x.items);setForm(v=>({...v,FromFinancialAccountPublicID:x.items[0]?.PublicID||"",ToFinancialAccountPublicID:x.items[1]?.PublicID||""}))}).catch(e=>setError(e.message))},[entity]);

  async function submit(){
    if(!entity)return;setError("");setMessage("");
    try{const v=await api<{id:string}>(`/entities/${entity.PublicID}/transfers`,{method:"POST",body:JSON.stringify(form)});setMessage(`Transfer posted: ${v.id}`);setForm(v=>({...v,FromAmount:"",ToAmount:"",Description:""}))}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  const from=accounts.find(x=>x.PublicID===form.FromFinancialAccountPublicID);
  const to=accounts.find(x=>x.PublicID===form.ToFinancialAccountPublicID);

  return <>
    <div className="page-head"><div><h1>Account Transfer</h1><p>Move money between cash, bank, wallet and card accounts.</p></div></div>
    {error&&<div className="alert error">{error}</div>}{message&&<div className="alert success">{message}</div>}
    <div className="card form">
      <div className="form-grid">
        <div className="field"><label>Date</label><input type="date" value={form.Date} onChange={e=>setForm({...form,Date:e.target.value})}/></div>
        <div className="field"><label>Description</label><input value={form.Description} onChange={e=>setForm({...form,Description:e.target.value})}/></div>
        <div className="field"><label>From</label><select value={form.FromFinancialAccountPublicID} onChange={e=>setForm({...form,FromFinancialAccountPublicID:e.target.value})}>{accounts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>From amount ({from?.Currency??"—"})</label><input inputMode="decimal" value={form.FromAmount} onChange={e=>setForm({...form,FromAmount:e.target.value})}/></div>
        <div className="field"><label>To</label><select value={form.ToFinancialAccountPublicID} onChange={e=>setForm({...form,ToFinancialAccountPublicID:e.target.value})}>{accounts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>To amount ({to?.Currency??"—"})</label><input inputMode="decimal" value={form.ToAmount} onChange={e=>setForm({...form,ToAmount:e.target.value})}/></div>
      </div>
      {from&&to&&from.Currency!==to.Currency&&<div className="alert">Cross-currency transfers use stored rates. The entered source/destination amounts must translate to the same functional value in V1.</div>}
      <button disabled={!form.Description||!form.FromAmount||!form.ToAmount||form.FromFinancialAccountPublicID===form.ToFinancialAccountPublicID} onClick={submit}>Post transfer</button>
    </div>
  </>;
}
