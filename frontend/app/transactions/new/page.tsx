"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount } from "@/components/types";

type Split={AccountPublicID:string;Amount:string;Description:string};

export default function NewTransaction(){
  const {entity}=useEntity();
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [financial,setFinancial]=useState<FinancialAccount[]>([]);
  const [type,setType]=useState("EXPENSE");
  const [date,setDate]=useState(new Date().toISOString().slice(0,10));
  const [description,setDescription]=useState("");
  const [fa,setFa]=useState("");
  const [splits,setSplits]=useState<Split[]>([{AccountPublicID:"",Amount:"",Description:""}]);
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");

  useEffect(()=>{
    if(!entity)return;
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`)
    ]).then(([a,f])=>{setAccounts(a.items);setFinancial(f.items);setFa(x=>x||f.items[0]?.PublicID||"")}).catch(e=>setError(e.message));
  },[entity]);

  const selectedFA=financial.find(x=>x.PublicID===fa);
  const eligible=accounts.filter(x=>x.Postable&&x.Type===type);
  const total=useMemo(()=>splits.reduce((n,x)=>n+(Number(x.Amount)||0),0),[splits]);

  function update(i:number,key:keyof Split,value:string){setSplits(xs=>xs.map((x,n)=>n===i?{...x,[key]:value}:x))}

  async function save(postNow:boolean){
    if(!entity||!selectedFA)return;
    setMessage("");setError("");
    try{
      const tx=await api<Transaction>(`/entities/${entity.PublicID}/transactions`,{
        method:"POST",
        body:JSON.stringify({
          Type:type,Date:date,Description:description,
          FinancialAccountPublicID:selectedFA.PublicID,
          Currency:selectedFA.Currency,
          Splits:splits
        })
      });
      if(postNow) await api(`/entities/${entity.PublicID}/transactions/${tx.PublicID}/post`,{method:"POST",body:"{}"});
      setMessage(postNow?"Transaction posted.":"Draft saved.");
      setDescription("");setSplits([{AccountPublicID:"",Amount:"",Description:""}]);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>New Entry</h1><p>Simple income/expense entry backed by double-entry journals.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}
    <div className="card form">
      <div className="form-grid">
        <div className="field"><label>Type</label><select value={type} onChange={e=>setType(e.target.value)}><option>EXPENSE</option><option>INCOME</option></select></div>
        <div className="field"><label>Date</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div>
        <div className="field span-2"><label>Description</label><input value={description} onChange={e=>setDescription(e.target.value)}/></div>
        <div className="field span-2"><label>{type==="EXPENSE"?"Paid from":"Received into"}</label><select value={fa} onChange={e=>setFa(e.target.value)}>{financial.map(x=><option key={x.PublicID} value={x.PublicID}>{x.Name} · {x.Currency}</option>)}</select></div>
      </div>
      <div><strong>Split</strong><p className="muted">One line for a normal entry; add more lines to split by category.</p></div>
      {splits.map((sp,i)=><div className="split-row" key={i}>
        <div className="field"><label>Account</label><select value={sp.AccountPublicID} onChange={e=>update(i,"AccountPublicID",e.target.value)}><option value="">Choose…</option>{eligible.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Amount</label><input inputMode="decimal" value={sp.Amount} onChange={e=>update(i,"Amount",e.target.value)}/></div>
        <div className="field"><label>Line description</label><input value={sp.Description} onChange={e=>update(i,"Description",e.target.value)}/></div>
        <button className="danger" disabled={splits.length===1} onClick={()=>setSplits(xs=>xs.filter((_,n)=>n!==i))}>×</button>
      </div>)}
      <div className="actions"><button className="secondary" onClick={()=>setSplits(xs=>[...xs,{AccountPublicID:"",Amount:"",Description:""}])}>+ Split</button><strong style={{marginLeft:"auto"}}>{total.toLocaleString()} {selectedFA?.Currency}</strong></div>
      <div className="actions"><button disabled={!description||!fa||total<=0||splits.some(x=>!x.AccountPublicID)} onClick={()=>save(false)}>Save draft</button><button disabled={!description||!fa||total<=0||splits.some(x=>!x.AccountPublicID)} onClick={()=>save(true)}>Save & post</button></div>
    </div>
  </>;
}
