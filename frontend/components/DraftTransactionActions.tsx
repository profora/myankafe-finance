"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { Account, Entity, FinancialAccount } from "@/components/types";
import ConfirmDialog from "@/components/ConfirmDialog";

type Contact={id:string;display_name:string;active:boolean};
type Split={
  line_no:number;
  account_id:string;
  account_code:string;
  account_name:string;
  amount:string;
  description:string;
};
type Draft={
  id:string;
  type:string;
  date:string;
  description:string;
  currency:string;
  contact?:{id?:string|null;name?:string|null};
  financial_account?:{id?:string|null;name?:string|null};
  splits:Split[];
};

type Props={
  entity:Entity;
  draft:Draft;
  onChanged:()=>void|Promise<void>;
};

type EditableSplit={AccountPublicID:string;Amount:string;Description:string};

export default function DraftTransactionActions({entity,draft,onChanged}:Props){
  const editRef=useRef<HTMLDialogElement>(null);
  const [editOpen,setEditOpen]=useState(false);
  const [postOpen,setPostOpen]=useState(false);
  const [cancelOpen,setCancelOpen]=useState(false);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");

  const [accounts,setAccounts]=useState<Account[]>([]);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [contacts,setContacts]=useState<Contact[]>([]);
  const [date,setDate]=useState(draft.date);
  const [description,setDescription]=useState(draft.description);
  const [financialID,setFinancialID]=useState(draft.financial_account?.id??"");
  const [contactID,setContactID]=useState(draft.contact?.id??"");
  const [splits,setSplits]=useState<EditableSplit[]>(draft.splits.map(x=>({
    AccountPublicID:x.account_id,Amount:x.amount,Description:x.description
  })));
  const [cancelReason,setCancelReason]=useState("");

  useEffect(()=>{
    const dialog=editRef.current;
    if(!dialog)return;
    if(editOpen&&!dialog.open)dialog.showModal();
    if(!editOpen&&dialog.open)dialog.close();
  },[editOpen]);

  useEffect(()=>{
    if(!editOpen)return;
    setDate(draft.date);
    setDescription(draft.description);
    setFinancialID(draft.financial_account?.id??"");
    setContactID(draft.contact?.id??"");
    setSplits(draft.splits.map(x=>({AccountPublicID:x.account_id,Amount:x.amount,Description:x.description})));
    setError("");
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
      api<{items:Contact[]}>(`/entities/${entity.PublicID}/contacts`)
    ]).then(([a,f,c])=>{
      setAccounts(a.items);
      setFinancial(f.items.filter(x=>x.Active));
      setContacts(c.items.filter(x=>x.active));
    }).catch(e=>setError(e instanceof Error?e.message:String(e)));
  },[editOpen,draft.id]);

  const eligibleAccounts=useMemo(
    ()=>accounts.filter(x=>x.Active&&x.Postable&&x.Type===draft.type),
    [accounts,draft.type]
  );
  const selectedFinancial=financial.find(x=>x.PublicID===financialID);
  const total=splits.reduce((sum,x)=>sum+(Number(x.Amount)||0),0);
  const ready=Boolean(
    date&&description.trim()&&financialID&&splits.length&&
    splits.every(x=>x.AccountPublicID&&Number(x.Amount)>0)
  );

  function closeEdit(){
    if(busy)return;
    setEditOpen(false);setError("");
  }

  async function save(){
    if(!ready)return;
    setBusy(true);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${draft.id}`,{
        method:"PUT",
        body:JSON.stringify({
          Type:draft.type,
          Date:date,
          Description:description.trim(),
          FinancialAccountPublicID:financialID,
          Currency:selectedFinancial?.Currency??draft.currency,
          ContactPublicID:contactID,
          Splits:splits
        })
      });
      setEditOpen(false);
      await onChanged();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function post(){
    setBusy(true);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${draft.id}/post`,{method:"POST",body:"{}"});
      setPostOpen(false);
      await onChanged();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function cancelDraft(){
    if(cancelReason.trim().length<3)return;
    setBusy(true);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${draft.id}/cancel`,{
        method:"POST",
        body:JSON.stringify({reason:cancelReason.trim()})
      });
      setCancelOpen(false);setCancelReason("");
      await onChanged();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function updateSplit(index:number,key:keyof EditableSplit,value:string){
    setSplits(current=>current.map((x,i)=>i===index?{...x,[key]:value}:x));
  }

  return <>
    {error&&<div className="alert error">{error}</div>}
    <div className="actions draft-actions">
      <button className="secondary" onClick={()=>setEditOpen(true)}>Edit draft</button>
      <button onClick={()=>setPostOpen(true)}>Post draft</button>
      <button className="danger" onClick={()=>setCancelOpen(true)}>Cancel draft</button>
    </div>

    <dialog ref={editRef} className="dialog entry-dialog" onCancel={e=>{e.preventDefault();closeEdit()}}>
      <div className="dialog-card">
        <div className="page-head compact-head">
          <div><h2>Edit draft</h2><p>Changes remain non-accounting until this draft is posted.</p></div>
          <button className="secondary" disabled={busy} onClick={closeEdit}>Close</button>
        </div>
        {error&&<div className="alert error">{error}</div>}
        <div className="form">
          <div className="form-grid">
            <div className="field"><label>Type</label><input value={draft.type} disabled/></div>
            <div className="field"><label>Date</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div>
            <div className="field"><label>{draft.type==="EXPENSE"?"Paid from":"Received into"}</label><select value={financialID} onChange={e=>setFinancialID(e.target.value)}>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
            <div className="field"><label>{draft.type==="EXPENSE"?"Payee":"Payer"} <span className="muted">(optional)</span></label><select value={contactID} onChange={e=>setContactID(e.target.value)}><option value="">None</option>{contacts.map(x=><option key={x.id} value={x.id}>{x.display_name}</option>)}</select></div>
            <div className="field span-2"><label>Description</label><input value={description} onChange={e=>setDescription(e.target.value)}/></div>
          </div>

          <div>
            <div className="page-head compact-head"><div><h2 style={{fontSize:18}}>Splits</h2></div><button className="secondary" onClick={()=>setSplits(x=>[...x,{AccountPublicID:eligibleAccounts[0]?.PublicID??"",Amount:"",Description:""}])}>+ Split</button></div>
            <div className="form">
              {splits.map((split,index)=><div className="split-row" key={index}>
                <div className="field"><label>Account</label><select value={split.AccountPublicID} onChange={e=>updateSplit(index,"AccountPublicID",e.target.value)}>{eligibleAccounts.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
                <div className="field"><label>Amount</label><input inputMode="decimal" value={split.Amount} onChange={e=>updateSplit(index,"Amount",e.target.value)}/></div>
                <div className="field"><label>Line description</label><input value={split.Description} onChange={e=>updateSplit(index,"Description",e.target.value)}/></div>
                <button className="danger compact" disabled={splits.length===1} onClick={()=>setSplits(x=>x.filter((_,i)=>i!==index))}>Remove</button>
              </div>)}
            </div>
          </div>

          <div className="actions modal-actions">
            <span className="muted">Total {total.toLocaleString()} {selectedFinancial?.Currency??draft.currency}</span>
            <span style={{flex:1}}/>
            <button disabled={busy||!ready} onClick={save}>{busy?"Saving…":"Save draft changes"}</button>
          </div>
        </div>
      </div>
    </dialog>

    <ConfirmDialog
      open={postOpen}
      title="Post this draft?"
      description="Posting creates the accounting journal. After posting, accounting fields become immutable; corrections must use a reversal."
      confirmLabel="Post transaction"
      busy={busy}
      onCancel={()=>{if(!busy)setPostOpen(false)}}
      onConfirm={post}
    />

    <ConfirmDialog
      open={cancelOpen}
      title="Cancel this draft?"
      description="The draft will be marked VOIDED rather than deleted so its history and attachments remain auditable."
      confirmLabel="Cancel draft"
      danger
      busy={busy}
      onCancel={()=>{if(!busy){setCancelOpen(false);setCancelReason("")}}}
      onConfirm={cancelDraft}
    >
      <div className="field" style={{marginTop:16}}>
        <label>Cancellation reason</label>
        <textarea rows={3} value={cancelReason} onChange={e=>setCancelReason(e.target.value)} placeholder="Why is this draft being cancelled?"/>
        {cancelReason.trim().length<3&&<span className="muted">A reason is required for the audit trail.</span>}
      </div>
    </ConfirmDialog>
  </>;
}
