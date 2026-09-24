"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Rate={id:string;rate_date:string;from_currency:string;to_currency:string;rate:string;source:string};

export default function ExchangeRates(){
  const {entity}=useEntity();
  const [items,setItems]=useState<Rate[]>([]);
  const [error,setError]=useState("");
  const [form,setForm]=useState({rate_date:new Date().toISOString().slice(0,10),from_currency:"USD",to_currency:"MMK",rate:"",source:"MANUAL",source_reference:""});

  const load=()=>{if(entity)api<{items:Rate[]}>(`/entities/${entity.PublicID}/exchange-rates`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity]);

  async function create(){
    if(!entity)return;
    try{await api(`/entities/${entity.PublicID}/exchange-rates`,{method:"POST",body:JSON.stringify(form)});setForm({...form,rate:"",source_reference:""});load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Exchange Rates</h1><p>Stored rate snapshots used by foreign-currency posting.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="card form" style={{marginBottom:16}}>
      <div className="form-grid">
        <div className="field"><label>Date</label><input type="date" value={form.rate_date} onChange={e=>setForm({...form,rate_date:e.target.value})}/></div>
        <div className="field"><label>Rate</label><input inputMode="decimal" value={form.rate} onChange={e=>setForm({...form,rate:e.target.value})}/></div>
        <div className="field"><label>From</label><input value={form.from_currency} onChange={e=>setForm({...form,from_currency:e.target.value.toUpperCase()})}/></div>
        <div className="field"><label>To</label><input value={form.to_currency} onChange={e=>setForm({...form,to_currency:e.target.value.toUpperCase()})}/></div>
        <div className="field span-2"><label>Source reference</label><input value={form.source_reference} onChange={e=>setForm({...form,source_reference:e.target.value})}/></div>
      </div>
      <button disabled={!form.rate} onClick={create}>Save rate</button>
    </div>
    <div className="table-wrap"><table><thead><tr><th>Date</th><th>From</th><th>To</th><th>Rate</th><th>Source</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{x.rate_date}</td><td>{x.from_currency}</td><td>{x.to_currency}</td><td>{x.rate}</td><td>{x.source}</td></tr>)}</tbody></table></div>
  </>;
}
