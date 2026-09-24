"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";

type Line={AccountPublicID:string;Debit:string;Credit:string;Description:string};

export default function ManualJournal(){
  const {entity}=useEntity();
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [date,setDate]=useState(dateInTimeZone(entity?.Timezone??"Asia/Yangon"));
  const [description,setDescription]=useState("");
  const [lines,setLines]=useState<Line[]>([
    {AccountPublicID:"",Debit:"",Credit:"",Description:""},
    {AccountPublicID:"",Debit:"",Credit:"",Description:""}
  ]);
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");

  useEffect(()=>{if(entity)api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`).then(x=>setAccounts(x.items.filter(a=>a.Postable&&a.Active))).catch(e=>setError(e.message))},[entity]);
  const debit=useMemo(()=>lines.reduce((n,x)=>n+(Number(x.Debit)||0),0),[lines]);
  const credit=useMemo(()=>lines.reduce((n,x)=>n+(Number(x.Credit)||0),0),[lines]);
  function update(i:number,k:keyof Line,v:string){setLines(xs=>xs.map((x,n)=>n===i?{...x,[k]:v}:x))}

  async function post(){
    if(!entity)return;setError("");setMessage("");
    try{
      const v=await api<{id:string}>(`/entities/${entity.PublicID}/manual-journals`,{method:"POST",body:JSON.stringify({Date:date,Description:description,Lines:lines})});
      setMessage(`Manual journal posted: ${v.id}`);
      setDescription("");setLines([{AccountPublicID:"",Debit:"",Credit:"",Description:""},{AccountPublicID:"",Debit:"",Credit:"",Description:""}]);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Manual Journal</h1><p>OWNER / ACCOUNTANT debit-credit entry. Posted journals are immutable.</p></div></div>
    {error&&<div className="alert error">{error}</div>}{message&&<div className="alert success">{message}</div>}
    <div className="card form">
      <div className="form-grid"><div className="field"><label>Date</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div><div className="field"><label>Description</label><input value={description} onChange={e=>setDescription(e.target.value)}/></div></div>
      {lines.map((l,i)=><div className="split-row" key={i}>
        <div className="field"><label>Account</label><select value={l.AccountPublicID} onChange={e=>update(i,"AccountPublicID",e.target.value)}><option value="">Choose…</option>{accounts.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Debit</label><input inputMode="decimal" value={l.Debit} onChange={e=>update(i,"Debit",e.target.value)}/></div>
        <div className="field"><label>Credit</label><input inputMode="decimal" value={l.Credit} onChange={e=>update(i,"Credit",e.target.value)}/></div>
        <button className="danger" disabled={lines.length===2} onClick={()=>setLines(xs=>xs.filter((_,n)=>n!==i))}>×</button>
      </div>)}
      <div className="actions"><button className="secondary" onClick={()=>setLines(xs=>[...xs,{AccountPublicID:"",Debit:"",Credit:"",Description:""}])}>+ Line</button><span style={{marginLeft:"auto"}}>Debits <strong>{debit.toLocaleString()}</strong> · Credits <strong>{credit.toLocaleString()}</strong></span></div>
      <button disabled={!description||debit<=0||debit!==credit||lines.some(x=>!x.AccountPublicID)} onClick={post}>Post journal</button>
    </div>
  </>;
}
