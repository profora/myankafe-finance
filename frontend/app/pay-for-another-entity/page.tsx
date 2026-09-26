"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { uploadTransactionAttachments } from "@/lib/attachments";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount } from "@/components/types";
import { canManageInterEntitySetup } from "@/lib/permissions";

type Status={
  configured:boolean;
  same_currency:boolean;
  payer_currency:string;
  counterparty_currency:string;
  payer_name:string;
  counterparty_name:string;
  payer_due_from_name?:string;
  counterparty_due_to_name?:string;
};

type Posted={
  id:string;
  initiating_transaction_id:string;
  counterparty_transaction_id:string;
};

export default function PayForAnotherEntity(){
  const {entity,entities}=useEntity();
  const counterparts=useMemo(()=>entities.filter(x=>x.PublicID!==entity?.PublicID),[entities,entity]);
  const mayConfigure=canManageInterEntitySetup(entity?.Role);
  const [counterparty,setCounterparty]=useState("");
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [expenses,setExpenses]=useState<Account[]>([]);
  const [status,setStatus]=useState<Status|null>(null);
  const [error,setError]=useState("");
  const [posted,setPosted]=useState<Posted|null>(null);
  const [attachments,setAttachments]=useState<File[]>([]);
  const [busy,setBusy]=useState(false);
  const [showAccounting,setShowAccounting]=useState(false);
  const [form,setForm]=useState({
    Date:dateInTimeZone(entity?.Timezone??"Asia/Yangon"),
    PayFrom:"",
    Amount:"",
    CounterpartyAmount:"",
    Expense:"",
    Description:"",
  });

  useEffect(()=>{
    if(!entity)return;
    let cancelled=false;
    setError("");setPosted(null);setStatus(null);setExpenses([]);setAttachments([]);
    setCounterparty(counterparts[0]?.PublicID||"");
    setForm({Date:dateInTimeZone(entity.Timezone),PayFrom:"",Amount:"",CounterpartyAmount:"",Expense:"",Description:""});
    api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`)
      .then(result=>{
        if(cancelled)return;
        const active=result.items.filter(x=>x.Active&&x.Currency===entity.FunctionalCurrency);
        setFinancial(active);
        setForm(v=>({...v,PayFrom:active[0]?.PublicID||""}));
      })
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))});
    return ()=>{cancelled=true};
  },[entity?.PublicID]);

  useEffect(()=>{
    if(!entity||!counterparty){setStatus(null);setExpenses([]);return}
    let cancelled=false;
    setStatus(null);setExpenses([]);
    Promise.all([
      api<Status>(`/entities/${entity.PublicID}/inter-entity-status/${counterparty}`),
      api<{items:Account[]}>(`/entities/${counterparty}/accounts`),
    ]).then(([nextStatus,accounts])=>{
      if(cancelled)return;
      setStatus(nextStatus);
      setExpenses(accounts.items.filter(x=>x.Active&&x.Postable&&x.Type==="EXPENSE"));
    }).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))});
    return ()=>{cancelled=true};
  },[entity?.PublicID,counterparty]);

  const counterpartyEntity=entities.find(x=>x.PublicID===counterparty);
  const sameCurrency=status?.same_currency??entity?.FunctionalCurrency===counterpartyEntity?.FunctionalCurrency;
  const amount=form.Amount;
  const counterpartyAmount=sameCurrency?form.Amount:form.CounterpartyAmount;
  const payFrom=financial.find(x=>x.PublicID===form.PayFrom);
  const expense=expenses.find(x=>x.PublicID===form.Expense);
  const ready=Boolean(status?.configured&&form.PayFrom&&amount&&counterpartyAmount&&form.Expense&&form.Description.trim()&&form.Date);

  async function post(){
    if(!entity||!counterparty||!ready)return;
    setError("");setBusy(true);setPosted(null);
    try{
      const result=await api<Posted>("/inter-entity-transactions",{method:"POST",body:JSON.stringify({
        InitiatingEntityID:entity.PublicID,
        CounterpartyEntityID:counterparty,
        Date:form.Date,
        InitiatingFinancialAccountPublicID:form.PayFrom,
        InitiatingAmount:amount,
        CounterpartyAmount:counterpartyAmount,
        CounterpartyExpenseAccountPublicID:form.Expense,
        Description:form.Description.trim(),
      })});
      if(attachments.length){
        try{await uploadTransactionAttachments(entity.PublicID,result.initiating_transaction_id,attachments)}
        catch(uploadErr){
          setPosted(result);
          setError(`Both accounting entries were posted, but attachment upload failed. Add files from the paying-entity transaction. ${uploadErr instanceof Error?uploadErr.message:String(uploadErr)}`);
          return;
        }
      }
      setPosted(result);
      setAttachments([]);
      setForm(v=>({...v,Amount:"",CounterpartyAmount:"",Description:""}));
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Pay for Another Entity</h1><p>Record a payment from {entity?.Name??"the selected entity"} for work or costs that belong to another entity.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {posted&&<div className="alert success" role="status">
      Posted. <Link href={`/transactions/${posted.initiating_transaction_id}`}>Paying entity transaction</Link>
      {" · "}
      <Link href={`/transactions/${posted.counterparty_transaction_id}?entity=${counterparty}`}>Counterparty transaction</Link>
    </div>}
    <div className="card form">
      <div className="form-grid">
        <div className="field"><label>Paying entity</label><input value={entity?.Name??""} disabled/></div>
        <div className="field"><label>Paid for</label><select value={counterparty} onChange={e=>setCounterparty(e.target.value)}><option value="">Choose…</option>{counterparts.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name}</option>)}</select></div>
        <div className="field"><label>Pay from</label><select value={form.PayFrom} onChange={e=>setForm({...form,PayFrom:e.target.value})}><option value="">Choose…</option>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
        <div className="field"><label>Date</label><input type="date" value={form.Date} onChange={e=>setForm({...form,Date:e.target.value})}/></div>
        <div className="field"><label>{sameCurrency?"Amount":"Amount paid"}</label><input value={form.Amount} onChange={e=>setForm({...form,Amount:e.target.value})} placeholder={entity?.FunctionalCurrency}/></div>
        {!sameCurrency&&<div className="field"><label>Counterparty amount</label><input value={form.CounterpartyAmount} onChange={e=>setForm({...form,CounterpartyAmount:e.target.value})} placeholder={counterpartyEntity?.FunctionalCurrency}/></div>}
        <div className="field"><label>Expense category</label><select value={form.Expense} onChange={e=>setForm({...form,Expense:e.target.value})}><option value="">Choose…</option>{expenses.map(x=><option key={x.PublicID} value={x.PublicID}>{counterpartyEntity?.Name} · {x.Name}</option>)}</select></div>
      </div>
      {!sameCurrency&&status&&<p className="muted">These entities use different currencies. Enter both amounts. Finance stores them as the historical values for this payment and does not look up a live exchange rate.</p>}
      <div className="field"><label>Description</label><textarea rows={3} value={form.Description} onChange={e=>setForm({...form,Description:e.target.value})}/></div>
      <div className="field"><label>Attachments</label><input type="file" multiple onChange={e=>setAttachments(Array.from(e.target.files??[]))}/></div>
      {counterparty&&status&&!status.configured&&<div className="alert" role="status">
        Inter-entity accounting has not been set up between {entity?.Name} and {counterpartyEntity?.Name}.
        {mayConfigure?<div style={{marginTop:8}}><Link href="/inter-entity-setup">Open Inter-Entity Setup</Link></div>:<div style={{marginTop:8}}>Ask an administrator to complete Inter-Entity Setup.</div>}
      </div>}
      {status?.configured&&amount&&<div className="card" style={{marginTop:8}}>
        <p>{entity?.Name} will pay {Number(amount).toLocaleString()} {entity?.FunctionalCurrency} for {counterpartyEntity?.Name}.</p>
        <p>Money leaves: {entity?.Name} → {payFrom?.Name??"the selected account"}</p>
        <p>Expense belongs to: {counterpartyEntity?.Name} → {expense?.Name??"the selected expense"}</p>
        <button type="button" className="secondary" onClick={()=>setShowAccounting(v=>!v)}>{showAccounting?"Hide accounting preview":"View accounting preview"}</button>
        {showAccounting&&<div style={{marginTop:12}}>
          <strong>{entity?.Name}</strong>
          <p>{status.payer_due_from_name} +{Number(amount).toLocaleString()} {entity?.FunctionalCurrency}</p>
          <p>{payFrom?.Name} −{Number(amount).toLocaleString()} {entity?.FunctionalCurrency}</p>
          <strong>{counterpartyEntity?.Name}</strong>
          <p>{expense?.Name} +{Number(counterpartyAmount||0).toLocaleString()} {status.counterparty_currency}</p>
          <p>{status.counterparty_due_to_name} +{Number(counterpartyAmount||0).toLocaleString()} {status.counterparty_currency}</p>
        </div>}
      </div>}
      <div className="actions" style={{marginTop:16}}><button type="button" disabled={!ready||busy} onClick={post}>{busy?"Posting…":"Post"}</button></div>
    </div>
  </>;
}
