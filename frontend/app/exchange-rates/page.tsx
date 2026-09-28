"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import ConfirmDialog from "@/components/ConfirmDialog";
import { canConfigureAccounting } from "@/lib/permissions";
import TableStateRows from "@/components/TableStateRows";
import { currencyChoices, useActiveCurrencies } from "@/components/CurrencySelect";

type Rate={
  id:string;
  rate_date:string;
  from_currency:string;
  to_currency:string;
  rate:string;
  source:string;
  source_reference?:string|null;
  used:boolean;
};

export default function ExchangeRates(){
  const {entity}=useEntity();
  const mayConfigure=canConfigureAccounting(entity?.Role);
  const {items:currencies}=useActiveCurrencies();
  const [items,setItems]=useState<Rate[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [busy,setBusy]=useState(false);
  const [loading,setLoading]=useState(true);
  const [editing,setEditing]=useState<Rate|null>(null);
  const [deleteTarget,setDeleteTarget]=useState<Rate|null>(null);
  const [form,setForm]=useState({rate_date:dateInTimeZone(entity?.Timezone??"Asia/Yangon"),from_currency:"USD",to_currency:"MMK",rate:"",source:"MANUAL",source_reference:""});
  const [edit,setEdit]=useState({RateDate:"",FromCurrency:"",ToCurrency:"",Rate:"",SourceReference:""});

  const load=async()=>{
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const result=await api<{items:Rate[]}>(`/entities/${entity.PublicID}/exchange-rates`);
      setItems(result.items);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  };
  useEffect(()=>{
    if(!entity)return;
    setItems([]);
    setEditing(null);
    setDeleteTarget(null);
    setForm(current=>({...current,rate_date:dateInTimeZone(entity.Timezone)}));
    void load();
  },[entity?.PublicID]);

  async function create(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/exchange-rates`,{method:"POST",body:JSON.stringify(form)});
      setForm({...form,rate:"",source_reference:""});
      setMessage("Exchange rate saved.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(rate:Rate){
    setEditing(rate);
    setEdit({
      RateDate:rate.rate_date,
      FromCurrency:rate.from_currency,
      ToCurrency:rate.to_currency,
      Rate:rate.rate,
      SourceReference:rate.source_reference??"",
    });
    setError("");setMessage("");
  }

  async function saveEdit(){
    if(!entity||!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/exchange-rates/${editing.id}`,{method:"PUT",body:JSON.stringify(edit)});
      setEditing(null);setMessage("Exchange rate corrected.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function remove(){
    if(!entity||!deleteTarget)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/exchange-rates/${deleteTarget.id}`,{method:"DELETE"});
      setDeleteTarget(null);setMessage("Unused exchange rate deleted.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Exchange Rates</h1><p>Stored rate snapshots used by foreign-currency posting.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}

    {mayConfigure&&editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}><div><h1 style={{fontSize:20}}>Correct unused rate</h1><p>Once used in posted accounting, the database permanently locks the snapshot.</p></div><button type="button" className="secondary" onClick={()=>setEditing(null)}>Cancel</button></div>
      <div className="form-grid">
        <div className="field"><label>Date</label><input type="date" value={edit.RateDate} onChange={e=>setEdit({...edit,RateDate:e.target.value})}/></div>
        <div className="field"><label>Rate</label><input inputMode="decimal" value={edit.Rate} onChange={e=>setEdit({...edit,Rate:e.target.value})}/></div>
        <div className="field"><label>From</label><select value={edit.FromCurrency} onChange={e=>setEdit({...edit,FromCurrency:e.target.value})}>{currencyChoices(currencies,[edit.FromCurrency,edit.ToCurrency]).map(item=><option key={item.code} value={item.code} disabled={item.code===edit.ToCurrency}>{item.code} · {item.name}</option>)}</select></div>
        <div className="field"><label>To</label><select value={edit.ToCurrency} onChange={e=>setEdit({...edit,ToCurrency:e.target.value})}>{currencyChoices(currencies,[edit.FromCurrency,edit.ToCurrency]).map(item=><option key={item.code} value={item.code} disabled={item.code===edit.FromCurrency}>{item.code} · {item.name}</option>)}</select></div>
        <div className="field span-2"><label>Source reference</label><input value={edit.SourceReference} onChange={e=>setEdit({...edit,SourceReference:e.target.value})}/></div>
      </div>
      <button type="button" disabled={busy||!edit.Rate} onClick={saveEdit}>{busy?"Saving…":"Save correction"}</button>
    </div>}

    {mayConfigure&&<div className="card form" style={{marginBottom:16}}>
      <h3>New manual rate</h3>
      <div className="form-grid">
        <div className="field"><label>Date</label><input type="date" value={form.rate_date} onChange={e=>setForm({...form,rate_date:e.target.value})}/></div>
        <div className="field"><label>Rate</label><input inputMode="decimal" value={form.rate} onChange={e=>setForm({...form,rate:e.target.value})}/></div>
        <div className="field"><label>From</label><select value={form.from_currency} onChange={e=>setForm({...form,from_currency:e.target.value})}>{currencies.map(item=><option key={item.code} value={item.code} disabled={item.code===form.to_currency}>{item.code} · {item.name}</option>)}</select></div>
        <div className="field"><label>To</label><select value={form.to_currency} onChange={e=>setForm({...form,to_currency:e.target.value})}>{currencies.map(item=><option key={item.code} value={item.code} disabled={item.code===form.from_currency}>{item.code} · {item.name}</option>)}</select></div>
        <div className="field span-2"><label>Source reference</label><input value={form.source_reference} onChange={e=>setForm({...form,source_reference:e.target.value})}/></div>
      </div>
      <button type="button" disabled={busy||!form.rate} onClick={create}>Save rate</button>
    </div>}

    {!mayConfigure&&<div className="alert">Your {entity?.Role??"VIEWER"} role can view stored FX snapshots but cannot create or correct rates.</div>}

    <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Date</th><th>From</th><th>To</th><th>Rate</th><th>Source</th><th>Reference</th><th>Status</th><th></th></tr></thead><tbody>{items.map(x=>{
      const editable=mayConfigure&&x.source==="MANUAL"&&!x.used;
      return <tr key={x.id}>
        <td>{x.rate_date}</td><td>{x.from_currency}</td><td>{x.to_currency}</td><td>{x.rate}</td><td>{x.source}</td><td>{x.source_reference??"—"}</td>
        <td><span className={`badge ${x.used?"POSTED":""}`}>{x.used?"USED · LOCKED":"UNUSED"}</span></td>
        <td><div className="actions">{editable&&<><button type="button" className="secondary compact" onClick={()=>startEdit(x)}>Edit</button><button type="button" className="danger compact" onClick={()=>setDeleteTarget(x)}>Delete</button></>}{!editable&&<span className="muted">Read-only</span>}</div></td>
      </tr>;
    })}<TableStateRows loading={loading&&items.length===0} empty={!loading&&items.length===0} columns={8} emptyText="No stored exchange rates for this entity yet."/></tbody></table></div>

    <ConfirmDialog
      open={Boolean(deleteTarget)}
      title="Delete unused exchange rate?"
      description="This is allowed only because the rate has not been used in posted accounting."
      confirmLabel="Delete rate"
      danger
      busy={busy}
      onCancel={()=>{if(!busy)setDeleteTarget(null)}}
      onConfirm={remove}
    >
      {deleteTarget&&<p><strong>{deleteTarget.rate_date}</strong> · {deleteTarget.from_currency} → {deleteTarget.to_currency} · {deleteTarget.rate}</p>}
    </ConfirmDialog>
  </>;
}
