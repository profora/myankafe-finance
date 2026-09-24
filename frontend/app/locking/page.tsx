"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type State={locked_through?:string|null;LockedThrough?:string|null;reason?:string|null;Reason?:string|null};

export default function Locking(){
  const {entity}=useEntity();
  const [state,setState]=useState<State>({});
  const [date,setDate]=useState("");
  const [reason,setReason]=useState("");
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");

  const load=()=>{if(entity)api<State>(`/entities/${entity.PublicID}/accounting-lock`).then(setState).catch(e=>setError(e.message))};
  useEffect(load,[entity]);
  const current=state.locked_through??state.LockedThrough??null;

  async function lock(){
    if(!entity)return;
    try{await api(`/entities/${entity.PublicID}/accounting-lock`,{method:"POST",body:JSON.stringify({locked_through:date,reason})});setMessage("Period locked.");setReason("");load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }
  async function unlock(){
    if(!entity)return;
    try{await api(`/entities/${entity.PublicID}/accounting-lock/unlock`,{method:"POST",body:JSON.stringify({reason})});setMessage("Period unlocked and audited.");setReason("");load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Transaction Locking</h1><p>OWNER and ACCOUNTANT can lock; only OWNER can unlock.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}
    <div className="grid cards">
      <div className="card"><div className="muted">Locked through</div><div className="metric">{current??"Open"}</div></div>
      <div className="card form">
        <div className="field"><label>Lock through</label><input type="date" value={date} onChange={e=>setDate(e.target.value)}/></div>
        <div className="field"><label>Reason</label><textarea value={reason} onChange={e=>setReason(e.target.value)}/></div>
        <div className="actions"><button disabled={!date||!reason} onClick={lock}>Lock</button><button className="danger" disabled={!current||!reason} onClick={unlock}>Owner unlock</button></div>
      </div>
    </div>
  </>;
}
