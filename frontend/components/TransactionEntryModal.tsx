"use client";

import Link from "next/link";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import type { Account, Entity, FinancialAccount, Transaction } from "@/components/types";

type Contact={id:string;display_name:string;contact_type:string;active:boolean};
export type QuickEntryKind="INCOME"|"EXPENSE"|"TRANSFER";

type Props={
  open:boolean;
  kind:QuickEntryKind|null;
  entity:Entity|null;
  onClose:()=>void;
  onSaved:()=>void;
};

function emptyFiles(input:HTMLInputElement|null){
  if(input) input.value="";
}

export default function TransactionEntryModal({open,kind,entity,onClose,onSaved}:Props){
  const dialogRef=useRef<HTMLDialogElement>(null);
  const titleID=useId();
  const descriptionID=useId();
  const fileRef=useRef<HTMLInputElement>(null);
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [contacts,setContacts]=useState<Contact[]>([]);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const [attachments,setAttachments]=useState<File[]>([]);
  const [loadingRefs,setLoadingRefs]=useState(false);

  const [date,setDate]=useState(()=>dateInTimeZone(entity?.Timezone??"Asia/Yangon"));
  const [description,setDescription]=useState("");
  const [financialID,setFinancialID]=useState("");
  const [accountID,setAccountID]=useState("");
  const [contactID,setContactID]=useState("");
  const [amount,setAmount]=useState("");

  const [toFinancialID,setToFinancialID]=useState("");
  const [toAmount,setToAmount]=useState("");
  const [feeAmount,setFeeAmount]=useState("");
  const [feeAccountID,setFeeAccountID]=useState("");

  useEffect(()=>{
    const d=dialogRef.current;
    if(!d)return;
    if(open&&!d.open)d.showModal();
    if(!open&&d.open)d.close();
  },[open]);

  useEffect(()=>{
    if(!open||!entity)return;
    let cancelled=false;
    setLoadingRefs(true);setError("");
    setAccounts([]);setFinancial([]);setContacts([]);
    setFinancialID("");setToFinancialID("");setAccountID("");setContactID("");
    setDescription("");setAmount("");setToAmount("");setFeeAmount("");setFeeAccountID("");setAttachments([]);
    emptyFiles(fileRef.current);
    setDate(dateInTimeZone(entity.Timezone));
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
      api<{items:Contact[]}>(`/entities/${entity.PublicID}/contacts`)
    ]).then(([a,f,c])=>{
      if(cancelled)return;
      const activeFinancial=f.items.filter(x=>x.Active);
      setAccounts(a.items);setFinancial(activeFinancial);setContacts(c.items.filter(x=>x.active));
      setFinancialID(activeFinancial[0]?.PublicID??"");
      setToFinancialID(activeFinancial[1]?.PublicID??activeFinancial[0]?.PublicID??"");
      const want=kind==="INCOME"?"INCOME":"EXPENSE";
      setAccountID(a.items.find(x=>x.Postable&&x.Active&&x.Type===want)?.PublicID??"");
    }).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoadingRefs(false)});
    return()=>{cancelled=true};
  },[open,entity?.PublicID,kind]);

  const eligible=useMemo(()=>accounts.filter(a=>a.Active&&a.Postable&&a.Type===(kind==="INCOME"?"INCOME":"EXPENSE")),[accounts,kind]);
  const fromFinancial=financial.find(x=>x.PublicID===financialID);
  const toFinancial=financial.find(x=>x.PublicID===toFinancialID);

  function resetAndClose(){
    if(busy)return;
    setDescription("");setAmount("");setToAmount("");setFeeAmount("");setFeeAccountID("");setContactID("");setAttachments([]);setError("");
    emptyFiles(fileRef.current);
    onClose();
  }

  async function uploadFiles(transactionID:string){
    if(!entity||attachments.length===0)return;
    const form=new FormData();
    attachments.forEach(f=>form.append("files",f));
    await api(`/entities/${entity.PublicID}/transactions/${transactionID}/attachments`,{method:"POST",body:form});
  }

  async function saveIncomeExpense(postNow:boolean){
    if(!entity||!kind||kind==="TRANSFER"||!fromFinancial)return;
    setBusy(true);setError("");
    try{
      const tx=await api<Transaction>(`/entities/${entity.PublicID}/transactions`,{
        method:"POST",
        body:JSON.stringify({
          Type:kind,
          Date:date,
          Description:description.trim(),
          FinancialAccountPublicID:fromFinancial.PublicID,
          Currency:fromFinancial.Currency,
          ContactPublicID:contactID,
          Splits:[{AccountPublicID:accountID,Amount:amount,Description:""}]
        })
      });
      try{
        await uploadFiles(tx.PublicID);
      }catch(e){
        setError(`Draft saved, but attachment upload failed: ${e instanceof Error?e.message:String(e)}`);
        onSaved();
        return;
      }
      if(postNow){
        await api(`/entities/${entity.PublicID}/transactions/${tx.PublicID}/post`,{method:"POST",body:"{}"});
      }
      onSaved();resetAndClose();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function saveTransfer(){
    if(!entity)return;
    setBusy(true);setError("");
    try{
      const tx=await api<{id:string}>(`/entities/${entity.PublicID}/transfers`,{
        method:"POST",
        body:JSON.stringify({
          Date:date,
          FromFinancialAccountPublicID:financialID,
          ToFinancialAccountPublicID:toFinancialID,
          FromAmount:amount,
          ToAmount:toAmount,
          FeeAmount:feeAmount,
          FeeExpenseAccountPublicID:feeAccountID,
          Description:description.trim()
        })
      });
      try{
        await uploadFiles(tx.id);
      }catch(e){
        setError(`Transfer posted, but attachment upload failed: ${e instanceof Error?e.message:String(e)}`);
        onSaved();
        return;
      }
      onSaved();resetAndClose();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  if(!kind)return null;
  const transfer=kind==="TRANSFER";
  const title=kind==="INCOME"?"New Income":kind==="EXPENSE"?"New Expense":"New Transfer";
  const sameCurrency=Boolean(transfer&&fromFinancial&&toFinancial&&fromFinancial.Currency===toFinancial.Currency);
  const ready=!loadingRefs&&(transfer
    ? Boolean(description.trim()&&financialID&&toFinancialID&&financialID!==toFinancialID&&Number(amount)>0&&Number(toAmount)>0&&(!sameCurrency||!(Number(feeAmount)>0)||feeAccountID))
    : Boolean(description.trim()&&financialID&&accountID&&Number(amount)>0));

  return <dialog ref={dialogRef} className="dialog entry-dialog" aria-labelledby={titleID} aria-describedby={descriptionID} aria-busy={loadingRefs||busy} onCancel={e=>{e.preventDefault();resetAndClose()}}>
    <div className="dialog-card">
      <div className="page-head compact-head">
        <div><h2 id={titleID}>{title}</h2><p id={descriptionID}>{transfer?"Move money between financial accounts.":"Compact daily entry; double-entry is created automatically."}</p></div>
        <button className="secondary" type="button" onClick={resetAndClose}>Close</button>
      </div>
      {error&&<div className="alert error" role="alert">{error}</div>}
      <div className="form">
        <div className="form-grid">
          <div className="field"><label>Date</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div>
          {!transfer&&<div className="field"><label>{kind==="EXPENSE"?"Paid from":"Received into"}</label><select value={financialID} disabled={loadingRefs} onChange={e=>setFinancialID(e.target.value)}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>}
          {transfer&&<><div className="field"><label>From</label><select value={financialID} disabled={loadingRefs} onChange={e=>setFinancialID(e.target.value)}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div><div className="field"><label>To</label><select value={toFinancialID} disabled={loadingRefs} onChange={e=>setToFinancialID(e.target.value)}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div></>}
          {!transfer&&<div className="field"><label>{kind==="INCOME"?"Income account":"Expense account"}</label><select value={accountID} disabled={loadingRefs} onChange={e=>setAccountID(e.target.value)}><option value="">{loadingRefs?"Loading accounts…":"Choose…"}</option>{eligible.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>}
          <div className="field"><label>{transfer?`From amount (${fromFinancial?.Currency??"—"})`:`Amount (${fromFinancial?.Currency??"—"})`}</label><input inputMode="decimal" value={amount} onChange={e=>{const value=e.target.value;setAmount(value);if(sameCurrency&&!(Number(feeAmount)>0))setToAmount(value);}} placeholder="0"/></div>
          {transfer&&<div className="field"><label>To amount ({toFinancial?.Currency??"—"})</label><input inputMode="decimal" value={toAmount} onChange={e=>setToAmount(e.target.value)} placeholder="0"/></div>}
          {sameCurrency&&<div className="field"><label>Transfer fee</label><input inputMode="decimal" value={feeAmount} onChange={e=>setFeeAmount(e.target.value)} placeholder="0"/></div>}
          {sameCurrency&&<div className="field"><label>Fee expense account</label><select value={feeAccountID} onChange={e=>setFeeAccountID(e.target.value)}><option value="">None</option>{accounts.filter(a=>a.Active&&a.Postable&&a.Type==="EXPENSE").map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>}
          {!transfer&&<div className="field"><label>{kind==="EXPENSE"?"Payee":"Payer"} <span className="muted">(optional)</span></label><select value={contactID} disabled={loadingRefs} onChange={e=>setContactID(e.target.value)}><option value="">{loadingRefs?"Loading contacts…":"None"}</option>{contacts.map(c=><option key={c.id} value={c.id}>{c.display_name}</option>)}</select></div>}
          <div className="field span-2"><label>Description</label><input autoFocus value={description} onChange={e=>setDescription(e.target.value)} placeholder={kind==="EXPENSE"?"e.g. Packaging and ribbons":kind==="INCOME"?"e.g. Flower arrangement sale":"e.g. Move cash to KBZPay"}/></div>
          <div className="field span-2"><label>Attachments <span className="muted">(optional, multiple)</span></label><input ref={fileRef} type="file" multiple accept="image/*,application/pdf,.csv,.txt,.docx,.xlsx" onChange={e=>setAttachments(Array.from(e.target.files??[]))}/>{attachments.length>0&&<div className="attachment-selection">{attachments.map(f=><span key={f.name+f.size} className="file-chip">{f.name}</span>)}</div>}</div>
        </div>

        {sameCurrency&&<div className="alert">{Number(feeAmount)>0?"Source amount must equal destination amount plus the fee.":"Same-currency amounts stay equal unless you enter a transfer fee."}</div>}
        {transfer&&fromFinancial&&toFinancial&&fromFinancial.Currency!==toFinancial.Currency&&<div className="alert">Cross-currency transfer amounts must translate to the same functional value using stored exchange rates.</div>}

        <div className="actions modal-actions">
          {!transfer&&<Link className="button secondary" href="/transactions/new" onClick={onClose}>Advanced / split entry</Link>}
          <span style={{flex:1}}/>
          {!transfer&&<button type="button" className="secondary" disabled={busy||!ready} onClick={()=>saveIncomeExpense(false)}>Save draft</button>}
          <button type="button" disabled={busy||!ready} onClick={()=>transfer?saveTransfer():saveIncomeExpense(true)}>{busy?"Saving…":transfer?"Post transfer":"Save & post"}</button>
        </div>
      </div>
    </div>
  </dialog>;
}
