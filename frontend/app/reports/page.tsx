"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Row={id:string;code:string;name:string;type:string;amount?:string;debits?:string;credits?:string};

export default function Reports(){
  const {entity}=useEntity();
  const [pl,setPL]=useState<Row[]>([]);
  const [tb,setTB]=useState<Row[]>([]);
  const [error,setError]=useState("");

  useEffect(()=>{
    if(!entity)return;
    Promise.all([
      api<{items:Row[]}>(`/entities/${entity.PublicID}/reports/profit-loss`),
      api<{items:Row[]}>(`/entities/${entity.PublicID}/reports/trial-balance`)
    ]).then(([p,t])=>{setPL(p.items);setTB(t.items)}).catch(e=>setError(e.message));
  },[entity]);

  return <>
    <div className="page-head"><div><h1>Reports</h1><p>Profit & Loss and Trial Balance from posted journal lines.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="grid" style={{gridTemplateColumns:"repeat(auto-fit,minmax(420px,1fr))"}}>
      <div><h3>Profit & Loss</h3><div className="table-wrap"><table><thead><tr><th>Account</th><th>Type</th><th>Amount</th></tr></thead><tbody>{pl.map(x=><tr key={x.id}><td>{x.code} · {x.name}</td><td>{x.type}</td><td>{Number(x.amount||0).toLocaleString()}</td></tr>)}</tbody></table></div></div>
      <div><h3>Trial Balance</h3><div className="table-wrap"><table><thead><tr><th>Account</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{tb.map(x=><tr key={x.id}><td>{x.code} · {x.name}</td><td>{Number(x.debits||0).toLocaleString()}</td><td>{Number(x.credits||0).toLocaleString()}</td></tr>)}</tbody></table></div></div>
    </div>
  </>;
}
