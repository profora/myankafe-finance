"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { uploadTransactionAttachments } from "@/lib/attachments";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount, Transaction } from "@/components/types";
type Contact={id:string;display_name:string;contact_type:string;active:boolean};

type Split={AccountPublicID:string;Amount:string;Description:string};

export default function NewTransaction(){
  const {entity}=useEntity();
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [contacts,setContacts]=useState<Contact[]>([]);
  const [contact,setContact]=useState("");
  const [type,setType]=useState("EXPENSE");
  const [date,setDate]=useState(dateInTimeZone(entity?.Timezone??"Asia/Yangon"));
  const [description,setDescription]=useState("");
  const [fa,setFa]=useState("");
  const [splits,setSplits]=useState<Split[]>([{AccountPublicID:"",Amount:"",Description:""}]);
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");
  const [attachments,setAttachments]=useState<File[]>([]);
  const [busy,setBusy]=useState(false);
  const [loadingRefs,setLoadingRefs]=useState(true);

  useEffect(()=>{
    if(typeof window!=="undefined"){
      const requested=new URLSearchParams(window.location.search).get("type");
      if(requested==="INCOME"||requested==="EXPENSE")setType(requested);
    }
  },[]);

  useEffect(()=>{
    if(!entity)return;
    let cancelled=false;
    setLoadingRefs(true);setError("");setMessage("");
    setAccounts([]);setFinancial([]);setContacts([]);
    setFa("");setContact("");
    setDate(dateInTimeZone(entity.Timezone));
    setDescription("");
    setSplits([{AccountPublicID:"",Amount:"",Description:""}]);
    setAttachments([]);
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
      api<{items:Contact[]}>(`/entities/${entity.PublicID}/contacts`)
    ]).then(([a,f,ct])=>{
      if(cancelled)return;
      const activeFinancial=f.items.filter(x=>x.Active);
      setAccounts(a.items);setFinancial(activeFinancial);setContacts(ct.items.filter(x=>x.active));
      setFa(activeFinancial[0]?.PublicID||"");
    }).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoadingRefs(false)});
    return()=>{cancelled=true};
  },[entity?.PublicID]);

  const selectedFA=financial.find(x=>x.PublicID===fa);
  const eligible=accounts.filter(x=>x.Active&&x.Postable&&x.Type===type);
  const total=useMemo(()=>splits.reduce((n,x)=>n+(Number(x.Amount)||0),0),[splits]);

  function update(i:number,key:keyof Split,value:string){setSplits(xs=>xs.map((x,n)=>n===i?{...x,[key]:value}:x))}

  async function save(postNow:boolean){
    if(!entity||!selectedFA)return;
    setMessage("");setError("");setBusy(true);
    let tx:Transaction|undefined;
    try{
      tx=await api<Transaction>(`/entities/${entity.PublicID}/transactions`,{
        method:"POST",
        body:JSON.stringify({
          Type:type,Date:date,Description:description,
          FinancialAccountPublicID:selectedFA.PublicID,
          Currency:selectedFA.Currency,
          ContactPublicID:contact,
          Splits:splits
        })
      });
      if(attachments.length){
        try{
          await uploadTransactionAttachments(entity.PublicID,tx.PublicID,attachments);
        }catch(uploadErr){
          setError(`Draft ${tx.PublicID} was created, but attachment upload failed. It was not posted. ${uploadErr instanceof Error?uploadErr.message:String(uploadErr)}`);
          return;
        }
      }
      if(postNow)await api(`/entities/${entity.PublicID}/transactions/${tx.PublicID}/post`,{method:"POST",body:"{}"});
      window.location.assign(`/transactions/${tx.PublicID}`);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>New Entry</h1><p>Simple income/expense entry backed by double-entry journals.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}
    <div className="card form" aria-busy={loadingRefs}>
      <div className="form-grid">
        <div className="field"><label>Type</label><select value={type} onChange={e=>setType(e.target.value)}><option>EXPENSE</option><option>INCOME</option></select></div>
        <div className="field"><label>Date</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div>
        <div className="field span-2"><label>Description</label><input value={description} onChange={e=>setDescription(e.target.value)}/></div>
        <div className="field span-2"><label>{type==="EXPENSE"?"Paid from":"Received into"}</label><select value={fa} disabled={loadingRefs} onChange={e=>setFa(e.target.value)}><option value="">{loadingRefs?"Loading financial accounts…":"Choose…"}</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field span-2"><label>{type==="EXPENSE"?"Payee":"Payer"}</label><select value={contact} disabled={loadingRefs} onChange={e=>setContact(e.target.value)}><option value="">{loadingRefs?"Loading contacts…":"None"}</option>{contacts.map(x=><option key={x.id} value={x.id}>{x.display_name} · {x.contact_type}</option>)}</select></div>
      </div>
      <div><strong>Split</strong><p className="muted">One line for a normal entry; add more lines to split by category.</p></div>
      {splits.map((sp,i)=><div className="split-row" key={i}>
        <div className="field"><label>Account</label><select value={sp.AccountPublicID} disabled={loadingRefs} onChange={e=>update(i,"AccountPublicID",e.target.value)}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{eligible.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Amount</label><input inputMode="decimal" value={sp.Amount} onChange={e=>update(i,"Amount",e.target.value)}/></div>
        <div className="field"><label>Line description</label><input value={sp.Description} onChange={e=>update(i,"Description",e.target.value)}/></div>
        <button type="button" className="danger" disabled={splits.length===1} onClick={()=>setSplits(xs=>xs.filter((_,n)=>n!==i))}>×</button>
      </div>)}
      <div className="actions"><button type="button" className="secondary" disabled={loadingRefs} onClick={()=>setSplits(xs=>[...xs,{AccountPublicID:"",Amount:"",Description:""}])}>+ Split</button><strong style={{marginLeft:"auto"}}>{total.toLocaleString()} {selectedFA?.Currency}</strong></div>
      <div className="field">
        <label>Attachments</label>
        <input type="file" multiple accept="image/jpeg,image/png,image/webp,image/gif,application/pdf,text/plain,text/csv,.docx,.xlsx" onChange={e=>setAttachments(Array.from(e.target.files??[]))}/>
        <span className="muted">{attachments.length?attachments.map(x=>x.name).join(", "):"Optional. Select multiple receipts/invoices; files upload before posting."}</span>
      </div>
      <div className="actions"><button type="button" disabled={loadingRefs||busy||!description||!fa||total<=0||splits.some(x=>!x.AccountPublicID)} onClick={()=>save(false)}>{busy?"Saving…":"Save draft"}</button><button type="button" disabled={loadingRefs||busy||!description||!fa||total<=0||splits.some(x=>!x.AccountPublicID)} onClick={()=>save(true)}>{busy?"Saving…":"Save & post"}</button></div>
    </div>
  </>;
}
