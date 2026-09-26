"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import TableStateRows from "@/components/TableStateRows";

type Currency={code:string;name:string;symbol:string;decimal_places:number;active:boolean};

const blank={Code:"",Name:"",Symbol:"",DecimalPlaces:2,Active:true};

export default function Currencies(){
  const {entities}=useEntity();
  const owner=entities.some(x=>x.Role==="OWNER");
  const [items,setItems]=useState<Currency[]>([]);
  const [form,setForm]=useState(blank);
  const [editing,setEditing]=useState<Currency|null>(null);
  const [edit,setEdit]=useState(blank);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [loading,setLoading]=useState(true);
  const [busy,setBusy]=useState(false);

  async function load(){
    setLoading(true);setError("");
    try{
      const result=await api<{items:Currency[]}>("/currencies");
      setItems(result.items);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }
  useEffect(()=>{if(owner)void load()},[owner]);

  async function create(){
    setBusy(true);setError("");setMessage("");
    try{
      await api("/currencies",{method:"POST",body:JSON.stringify(form)});
      setForm(blank);setMessage("Currency created.");await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(item:Currency){
    setEditing(item);
    setEdit({Code:item.code,Name:item.name,Symbol:item.symbol,DecimalPlaces:item.decimal_places,Active:item.active});
  }

  async function save(){
    if(!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/currencies/${editing.code}`,{method:"PUT",body:JSON.stringify(edit)});
      setEditing(null);setMessage("Currency updated.");await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  if(!owner)return <div className="alert">Currency configuration requires an OWNER role.</div>;

  return <>
    <div className="page-head"><div><h1>Currencies</h1><p>The currency master used by entities, financial accounts, journals and exchange rates.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}
    {editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}><div><h1 style={{fontSize:20}}>Edit {editing.code}</h1><p>The code stays fixed once the currency exists.</p></div><button type="button" className="secondary" onClick={()=>setEditing(null)}>Cancel</button></div>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={editing.code} disabled/></div>
        <div className="field"><label>Name</label><input value={edit.Name} onChange={e=>setEdit({...edit,Name:e.target.value})}/></div>
        <div className="field"><label>Symbol</label><input value={edit.Symbol} onChange={e=>setEdit({...edit,Symbol:e.target.value})}/></div>
        <div className="field"><label>Decimal places</label><input type="number" min={0} max={6} value={edit.DecimalPlaces} onChange={e=>setEdit({...edit,DecimalPlaces:Number(e.target.value)})}/></div>
        <div className="field"><label>Status</label><select value={edit.Active?"ACTIVE":"INACTIVE"} onChange={e=>setEdit({...edit,Active:e.target.value==="ACTIVE"})}><option>ACTIVE</option><option>INACTIVE</option></select></div>
      </div>
      <button type="button" disabled={busy||!edit.Name.trim()} onClick={save}>{busy?"Saving…":"Save currency"}</button>
    </div>}
    <div className="card form" style={{marginBottom:16}}>
      <h3>New currency</h3>
      <div className="form-grid">
        <div className="field"><label>Code</label><input maxLength={3} value={form.Code} onChange={e=>setForm({...form,Code:e.target.value.toUpperCase()})} placeholder="EUR"/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})}/></div>
        <div className="field"><label>Symbol</label><input value={form.Symbol} onChange={e=>setForm({...form,Symbol:e.target.value})}/></div>
        <div className="field"><label>Decimal places</label><input type="number" min={0} max={6} value={form.DecimalPlaces} onChange={e=>setForm({...form,DecimalPlaces:Number(e.target.value)})}/></div>
      </div>
      <button type="button" disabled={busy||form.Code.length!==3||!form.Name.trim()} onClick={create}>Create currency</button>
    </div>
    <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Code</th><th>Name</th><th>Symbol</th><th>Decimals</th><th>Status</th><th></th></tr></thead><tbody>{items.map(item=><tr key={item.code}>
      <td>{item.code}</td><td>{item.name}</td><td>{item.symbol||"—"}</td><td>{item.decimal_places}</td>
      <td><span className={`badge ${item.active?"POSTED":"VOIDED"}`}>{item.active?"ACTIVE":"INACTIVE"}</span></td>
      <td><button type="button" className="secondary compact" onClick={()=>startEdit(item)}>Edit</button></td>
    </tr>)}<TableStateRows loading={loading&&items.length===0} empty={!loading&&items.length===0} columns={6} emptyText="No currencies are configured."/></tbody></table></div>
  </>;
}
