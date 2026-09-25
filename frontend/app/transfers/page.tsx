"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { uploadTransactionAttachments } from "@/lib/attachments";
import { useEntity } from "@/components/EntityContext";
import type { FinancialAccount } from "@/components/types";

export default function Transfers(){
  const {entity}=useEntity();
  const [accounts,setAccounts]=useState<FinancialAccount[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [attachments,setAttachments]=useState<File[]>([]);
  const [busy,setBusy]=useState(false);
  const [loadingRefs,setLoadingRefs]=useState(true);
  const [form,setForm]=useState({Date:dateInTimeZone(entity?.Timezone??"Asia/Yangon"),FromFinancialAccountPublicID:"",ToFinancialAccountPublicID:"",FromAmount:"",ToAmount:"",Description:""});

  useEffect(()=>{
    if(!entity)return;
    let cancelled=false;
    setLoadingRefs(true);setError("");setMessage("");setAccounts([]);setAttachments([]);
    setForm({Date:dateInTimeZone(entity.Timezone),FromFinancialAccountPublicID:"",ToFinancialAccountPublicID:"",FromAmount:"",ToAmount:"",Description:""});
    api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`)
      .then(x=>{
        if(cancelled)return;
        const active=x.items.filter(a=>a.Active);
        setAccounts(active);
        setForm(v=>({...v,FromFinancialAccountPublicID:active[0]?.PublicID||"",ToFinancialAccountPublicID:active[1]?.PublicID||""}));
      })
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoadingRefs(false)});
    return()=>{cancelled=true};
  },[entity?.PublicID]);

  async function submit(){
    if(!entity)return;setError("");setMessage("");setBusy(true);
    try{
      const v=await api<{id:string}>(`/entities/${entity.PublicID}/transfers`,{method:"POST",body:JSON.stringify(form)});
      if(attachments.length){
        try{await uploadTransactionAttachments(entity.PublicID,v.id,attachments)}
        catch(uploadErr){
          setMessage(`Transfer posted: ${v.id}`);
          setError(`The transfer was posted, but attachment upload failed. You can add the files from the transaction detail page. ${uploadErr instanceof Error?uploadErr.message:String(uploadErr)}`);
          return;
        }
      }
      window.location.assign(`/transactions/${v.id}`);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  const from=accounts.find(x=>x.PublicID===form.FromFinancialAccountPublicID);
  const to=accounts.find(x=>x.PublicID===form.ToFinancialAccountPublicID);

  return <>
    <div className="page-head"><div><h1>Account Transfer</h1><p>Move money between cash, bank, wallet and card accounts.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}{message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}
    <div className="card form" aria-busy={loadingRefs}>
      <div className="form-grid">
        <div className="field"><label>Date</label><input type="date" value={form.Date} onChange={e=>setForm({...form,Date:e.target.value})}/></div>
        <div className="field"><label>Description</label><input value={form.Description} onChange={e=>setForm({...form,Description:e.target.value})}/></div>
        <div className="field"><label>From</label><select value={form.FromFinancialAccountPublicID} disabled={loadingRefs} onChange={e=>setForm({...form,FromFinancialAccountPublicID:e.target.value})}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{accounts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>From amount ({from?.Currency??"—"})</label><input inputMode="decimal" value={form.FromAmount} onChange={e=>setForm({...form,FromAmount:e.target.value})}/></div>
        <div className="field"><label>To</label><select value={form.ToFinancialAccountPublicID} disabled={loadingRefs} onChange={e=>setForm({...form,ToFinancialAccountPublicID:e.target.value})}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{accounts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>To amount ({to?.Currency??"—"})</label><input inputMode="decimal" value={form.ToAmount} onChange={e=>setForm({...form,ToAmount:e.target.value})}/></div>
      </div>
      {from&&to&&from.Currency!==to.Currency&&<div className="alert">Cross-currency transfers use stored rates. The entered source/destination amounts must translate to the same functional value in V1.</div>}
      <div className="field"><label>Attachments</label><input type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif,application/pdf,text/plain,text/csv,.docx,.xlsx" onChange={e=>setAttachments(Array.from(e.target.files??[]))}/><span className="muted">{attachments.length?attachments.map(x=>x.name).join(", "):"Optional receipts or transfer confirmations."}</span></div>
      <button type="button" disabled={loadingRefs||busy||!form.Description||!form.FromFinancialAccountPublicID||!form.ToFinancialAccountPublicID||!form.FromAmount||!form.ToAmount||form.FromFinancialAccountPublicID===form.ToFinancialAccountPublicID} onClick={submit}>{busy?"Posting…":"Post transfer"}</button>
    </div>
  </>;
}
