"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { uploadTransactionAttachments } from "@/lib/attachments";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount } from "@/components/types";

export default function InterEntity(){
  const {entity,entities}=useEntity();
  const counterparts=useMemo(()=>entities.filter(x=>x.PublicID!==entity?.PublicID),[entities,entity]);
  const [counterparty,setCounterparty]=useState("");
  const [ownAccounts,setOwnAccounts]=useState<Account[]>([]);
  const [cpAccounts,setCpAccounts]=useState<Account[]>([]);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [attachments,setAttachments]=useState<File[]>([]);
  const [busy,setBusy]=useState(false);
  const [loadingOwn,setLoadingOwn]=useState(true);
  const [loadingCounterparty,setLoadingCounterparty]=useState(false);
  const [mapping,setMapping]=useState({due_from_account_id:"",due_to_account_id:""});
  const [form,setForm]=useState({Date:dateInTimeZone(entity?.Timezone??"Asia/Yangon"),InitiatingFinancialAccountPublicID:"",InitiatingAmount:"",CounterpartyAmount:"",CounterpartyExpenseAccountPublicID:"",Description:""});

  useEffect(()=>{
    if(!entity)return;
    let cancelled=false;
    const firstCounterparty=counterparts[0]?.PublicID||"";
    setLoadingOwn(true);setError("");setMessage("");
    setOwnAccounts([]);setFinancial([]);setCpAccounts([]);setMapping({due_from_account_id:"",due_to_account_id:""});
    setAttachments([]);setCounterparty(firstCounterparty);
    setForm({Date:dateInTimeZone(entity.Timezone),InitiatingFinancialAccountPublicID:"",InitiatingAmount:"",CounterpartyAmount:"",CounterpartyExpenseAccountPublicID:"",Description:""});
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`)
    ]).then(([a,f])=>{
      if(cancelled)return;
      const activeFinancial=f.items.filter(x=>x.Active);
      setOwnAccounts(a.items);setFinancial(activeFinancial);
      setForm(v=>({...v,InitiatingFinancialAccountPublicID:activeFinancial[0]?.PublicID||""}));
    }).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoadingOwn(false)});
    return()=>{cancelled=true};
  },[entity?.PublicID]);

  useEffect(()=>{
    if(!counterparty){setCpAccounts([]);setLoadingCounterparty(false);return}
    let cancelled=false;
    setCpAccounts([]);setLoadingCounterparty(true);
    api<{items:Account[]}>(`/entities/${counterparty}/accounts`)
      .then(x=>{if(!cancelled)setCpAccounts(x.items)})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoadingCounterparty(false)});
    return()=>{cancelled=true};
  },[counterparty]);

  async function saveMapping(){
    if(!entity||!counterparty)return;setError("");setMessage("");
    try{await api(`/entities/${entity.PublicID}/inter-entity-mappings/${counterparty}`,{method:"PUT",body:JSON.stringify(mapping)});setMessage("Mapping saved. Configure the reverse mapping on the counterparty entity too.")}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  async function post(){
    if(!entity||!counterparty)return;setError("");setMessage("");setBusy(true);
    try{
      const v=await api<{id:string;initiating_transaction_id:string;counterparty_transaction_id:string}>("/inter-entity-transactions",{method:"POST",body:JSON.stringify({InitiatingEntityID:entity.PublicID,CounterpartyEntityID:counterparty,...form})});
      if(attachments.length){
        try{await uploadTransactionAttachments(entity.PublicID,v.initiating_transaction_id,attachments)}
        catch(uploadErr){
          setMessage(`Inter-entity transaction posted atomically: ${v.id}`);
          setError(`Both accounting entries were posted, but attachment upload failed on the initiating transaction. Add files from transaction ${v.initiating_transaction_id}. ${uploadErr instanceof Error?uploadErr.message:String(uploadErr)}`);
          return;
        }
      }
      window.location.assign(`/transactions/${v.initiating_transaction_id}`);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  const dueFrom=ownAccounts.filter(a=>a.Active&&a.Postable&&a.Type==="ASSET");
  const dueTo=ownAccounts.filter(a=>a.Active&&a.Postable&&a.Type==="LIABILITY");
  const expenses=cpAccounts.filter(a=>a.Active&&a.Postable&&a.Type==="EXPENSE");

  return <>
    <div className="page-head"><div><h1>Inter-Entity</h1><p>Atomic due-to / due-from accounting across entities.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}{message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}
    <div className="grid" style={{gridTemplateColumns:"repeat(auto-fit,minmax(360px,1fr))"}}>
      <div className="card form" aria-busy={loadingOwn}>
        <h3>Mapping for {entity?.Name}</h3>
        <div className="field"><label>Counterparty</label><select value={counterparty} disabled={loadingOwn} onChange={e=>setCounterparty(e.target.value)}><option value="">{loadingOwn?"Loading entities…":"Choose…"}</option>{counterparts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name}</option>)}</select></div>
        <div className="field"><label>Due from account (Asset)</label><select value={mapping.due_from_account_id} disabled={loadingOwn} onChange={e=>setMapping({...mapping,due_from_account_id:e.target.value})}><option value="">Choose…</option>{dueFrom.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Due to account (Liability)</label><select value={mapping.due_to_account_id} disabled={loadingOwn} onChange={e=>setMapping({...mapping,due_to_account_id:e.target.value})}><option value="">Choose…</option>{dueTo.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <button type="button" disabled={loadingOwn||!counterparty||!mapping.due_from_account_id||!mapping.due_to_account_id} onClick={saveMapping}>Save mapping</button>
      </div>
      <div className="card form" aria-busy={loadingOwn||loadingCounterparty}>
        <h3>Pay expense on behalf</h3>
        <div className="field"><label>Pay from</label><select value={form.InitiatingFinancialAccountPublicID} disabled={loadingOwn} onChange={e=>setForm({...form,InitiatingFinancialAccountPublicID:e.target.value})}><option value="">{loadingOwn?"Loading accounts…":"Choose…"}</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="form-grid">
          <div className="field"><label>Date</label><input type="date" value={form.Date} onChange={e=>setForm({...form,Date:e.target.value})}/></div>
          <div className="field"><label>Paid amount ({entity?.FunctionalCurrency})</label><input inputMode="decimal" value={form.InitiatingAmount} onChange={e=>setForm({...form,InitiatingAmount:e.target.value,CounterpartyAmount:e.target.value})}/></div>
          <div className="field"><label>Counterparty amount</label><input inputMode="decimal" value={form.CounterpartyAmount} onChange={e=>setForm({...form,CounterpartyAmount:e.target.value})}/></div>
          <div className="field"><label>Counterparty expense account</label><select value={form.CounterpartyExpenseAccountPublicID} disabled={loadingCounterparty} onChange={e=>setForm({...form,CounterpartyExpenseAccountPublicID:e.target.value})}><option value="">{loadingCounterparty?"Loading counterparty accounts…":"Choose…"}</option>{expenses.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
          <div className="field span-2"><label>Description</label><input value={form.Description} onChange={e=>setForm({...form,Description:e.target.value})}/></div>
        </div>
        <div className="field"><label>Attachments</label><input type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif,application/pdf,text/plain,text/csv,.docx,.xlsx" onChange={e=>setAttachments(Array.from(e.target.files??[]))}/><span className="muted">{attachments.length?attachments.map(x=>x.name).join(", "):"Optional supporting documents. They are stored on the initiating transaction."}</span></div>
        <button type="button" disabled={loadingOwn||loadingCounterparty||busy||!counterparty||!form.InitiatingFinancialAccountPublicID||!form.InitiatingAmount||!form.CounterpartyAmount||!form.CounterpartyExpenseAccountPublicID||!form.Description} onClick={post}>{busy?"Posting…":"Post both entities atomically"}</button>
      </div>
    </div>
  </>;
}
