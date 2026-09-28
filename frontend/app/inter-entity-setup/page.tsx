"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";
import { canManageInterEntitySetup } from "@/lib/permissions";

type Pair={
  counterparty_entity_id:string;
  counterparty_name:string;
  payer_due_from_account:{id:string;code:string;name:string};
  payer_due_to_account:{id:string;code:string;name:string};
  counterparty_due_from_account?:{id:string;code:string;name:string}|null;
  counterparty_due_to_account?:{id:string;code:string;name:string}|null;
};

export default function InterEntitySetup(){
  const {entity,entities}=useEntity();
  const allowed=canManageInterEntitySetup(entity?.Role);
  const counterparts=useMemo(()=>entities.filter(x=>x.PublicID!==entity?.PublicID&&(x.Role==="OWNER"||x.Role==="ADMIN")),[entities,entity]);
  const [counterparty,setCounterparty]=useState("");
  const [own,setOwn]=useState<Account[]>([]);
  const [theirs,setTheirs]=useState<Account[]>([]);
  const [pairs,setPairs]=useState<Pair[]>([]);
  const [form,setForm]=useState({payerFrom:"",payerTo:"",counterpartyFrom:"",counterpartyTo:""});
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [busy,setBusy]=useState(false);

  useEffect(()=>{
    if(!entity||!allowed)return;
    let cancelled=false;
    setError("");setMessage("");setOwn([]);setTheirs([]);setPairs([]);
    setCounterparty("");
    setForm({payerFrom:"",payerTo:"",counterpartyFrom:"",counterpartyTo:""});
    Promise.all([
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
      api<{items:Pair[]}>(`/entities/${entity.PublicID}/inter-entity-setup`),
    ]).then(([accounts,setup])=>{
      if(cancelled)return;
      setOwn(accounts.items);
      setPairs(setup.items??[]);
    }).catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))});
    return ()=>{cancelled=true};
  },[entity?.PublicID,allowed]);

  useEffect(()=>{
    if(!counterparty){setTheirs([]);return}
    const existing=pairs.find(x=>x.counterparty_entity_id===counterparty);
    setForm({
      payerFrom:existing?.payer_due_from_account.id??"",
      payerTo:existing?.payer_due_to_account.id??"",
      counterpartyFrom:existing?.counterparty_due_from_account?.id??"",
      counterpartyTo:existing?.counterparty_due_to_account?.id??"",
    });
    let cancelled=false;
    api<{items:Account[]}>(`/entities/${counterparty}/accounts`)
      .then(result=>{if(!cancelled)setTheirs(result.items)})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))});
    return ()=>{cancelled=true};
  },[counterparty,pairs]);

  async function save(){
    if(!entity||!counterparty)return;
    setError("");setMessage("");setBusy(true);
    try{
      await api(`/entities/${entity.PublicID}/inter-entity-pairs/${counterparty}`,{method:"PUT",body:JSON.stringify({
        PayerDueFromAccountID:form.payerFrom,
        PayerDueToAccountID:form.payerTo,
        CounterpartyDueFromAccountID:form.counterpartyFrom,
        CounterpartyDueToAccountID:form.counterpartyTo,
      })});
      const setup=await api<{items:Pair[]}>(`/entities/${entity.PublicID}/inter-entity-setup`);
      setPairs(setup.items??[]);
      setMessage("Inter-entity setup saved for both entities.");
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  const assets=(rows:Account[])=>rows.filter(x=>x.Active&&x.Postable&&x.Type==="ASSET");
  const liabilities=(rows:Account[])=>rows.filter(x=>x.Active&&x.Postable&&x.Type==="LIABILITY");
  const other=entities.find(x=>x.PublicID===counterparty);

  if(entity&&!allowed){
    return <div className="alert error" role="alert">Inter-Entity Setup is limited to OWNER and ADMIN.</div>;
  }

  return <>
    <div className="page-head"><div><h1>Inter-Entity Setup</h1><p>Configure both directions of a due-from and due-to pair in one save. Daily payments do not use this screen.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status">{message}</div>}
    <div className="card form">
      <div className="field"><label>Entity pair</label><select value={counterparty} onChange={e=>setCounterparty(e.target.value)}><option value="">Choose the other entity…</option>{counterparts.map(x=><option key={x.PublicID} value={x.PublicID}>{entity?.Name} ↔ {x.Name}</option>)}</select></div>
      {counterparty&&<>
        <h3>{entity?.Name}</h3>
        <div className="form-grid">
          <div className="field"><label>Due from {other?.Name} · Asset</label><select value={form.payerFrom} onChange={e=>setForm({...form,payerFrom:e.target.value})}><option value="">Choose…</option>{assets(own).map(x=><option key={x.PublicID} value={x.PublicID}>{x.Code} · {x.Name}</option>)}</select></div>
          <div className="field"><label>Due to {other?.Name} · Liability</label><select value={form.payerTo} onChange={e=>setForm({...form,payerTo:e.target.value})}><option value="">Choose…</option>{liabilities(own).map(x=><option key={x.PublicID} value={x.PublicID}>{x.Code} · {x.Name}</option>)}</select></div>
        </div>
        <h3>{other?.Name}</h3>
        <div className="form-grid">
          <div className="field"><label>Due from {entity?.Name} · Asset</label><select value={form.counterpartyFrom} onChange={e=>setForm({...form,counterpartyFrom:e.target.value})}><option value="">Choose…</option>{assets(theirs).map(x=><option key={x.PublicID} value={x.PublicID}>{x.Code} · {x.Name}</option>)}</select></div>
          <div className="field"><label>Due to {entity?.Name} · Liability</label><select value={form.counterpartyTo} onChange={e=>setForm({...form,counterpartyTo:e.target.value})}><option value="">Choose…</option>{liabilities(theirs).map(x=><option key={x.PublicID} value={x.PublicID}>{x.Code} · {x.Name}</option>)}</select></div>
        </div>
        <button type="button" disabled={busy||!form.payerFrom||!form.payerTo||!form.counterpartyFrom||!form.counterpartyTo} onClick={save}>{busy?"Saving…":"Save pair"}</button>
      </>}
    </div>
    {pairs.length>0&&<div className="table-wrap" style={{marginTop:16}}><table><thead><tr><th>Counterparty</th><th>{entity?.Name} due from</th><th>{entity?.Name} due to</th><th>Counterparty due from</th><th>Counterparty due to</th></tr></thead><tbody>{pairs.map(x=><tr key={x.counterparty_entity_id}><td>{x.counterparty_name}</td><td>{x.payer_due_from_account.code} · {x.payer_due_from_account.name}</td><td>{x.payer_due_to_account.code} · {x.payer_due_to_account.name}</td><td>{x.counterparty_due_from_account?`${x.counterparty_due_from_account.code} · ${x.counterparty_due_from_account.name}`:"—"}</td><td>{x.counterparty_due_to_account?`${x.counterparty_due_to_account.code} · ${x.counterparty_due_to_account.name}`:"—"}</td></tr>)}</tbody></table></div>}
  </>;
}
