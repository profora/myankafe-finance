"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import { canConfigureAccounting } from "@/lib/permissions";
import type { SalesChannel } from "@/components/types";
import TableStateRows from "@/components/TableStateRows";

export default function SalesChannels(){
  const {entity}=useEntity();
  const mayConfigure=canConfigureAccounting(entity?.Role);
  const [items,setItems]=useState<SalesChannel[]>([]);
  const [form,setForm]=useState({Code:"",Name:"",Active:true});
  const [editing,setEditing]=useState<SalesChannel|null>(null);
  const [edit,setEdit]=useState({Name:"",Active:true});
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [loading,setLoading]=useState(true);
  const [busy,setBusy]=useState(false);

  async function load(){
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const result=await api<{items:SalesChannel[]}>(`/entities/${entity.PublicID}/sales-channels`);
      setItems(result.items);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{
    setItems([]);setEditing(null);
    if(entity)void load();
  },[entity?.PublicID]);

  async function create(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/sales-channels`,{
        method:"POST",
        body:JSON.stringify(form),
      });
      setForm({Code:"",Name:"",Active:true});
      setMessage("Sales channel created.");
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(item:SalesChannel){
    setEditing(item);
    setEdit({Name:item.name,Active:item.active});
    setError("");setMessage("");
  }

  async function save(){
    if(!entity||!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/sales-channels/${editing.code}`,{
        method:"PUT",
        body:JSON.stringify({Name:edit.Name,Active:edit.Active}),
      });
      setEditing(null);
      setMessage("Sales channel updated.");
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  if(!mayConfigure)return <div className="alert">Sales channel configuration requires OWNER, ADMIN, or ACCOUNTANT.</div>;

  return <>
    <div className="page-head">
      <div>
        <h1>Sales Channels</h1>
        <p>Define how sales reach the customer. Codes stay stable so future MyanKafe Platform integration can map to them.</p>
      </div>
    </div>

    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}

    {editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}>
        <div><h1 style={{fontSize:20}}>Edit {editing.code}</h1><p>Deactivate a channel instead of deleting it so historical sales remain reportable.</p></div>
        <button type="button" className="secondary" onClick={()=>setEditing(null)}>Cancel</button>
      </div>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={editing.code} disabled/></div>
        <div className="field"><label>Name</label><input value={edit.Name} onChange={e=>setEdit({...edit,Name:e.target.value})}/></div>
        <div className="field"><label>Status</label><select value={edit.Active?"ACTIVE":"INACTIVE"} onChange={e=>setEdit({...edit,Active:e.target.value==="ACTIVE"})}><option>ACTIVE</option><option>INACTIVE</option></select></div>
      </div>
      <button type="button" disabled={busy||!edit.Name.trim()} onClick={save}>{busy?"Saving…":"Save sales channel"}</button>
    </div>}

    <div className="card form" style={{marginBottom:16}}>
      <h3>New sales channel</h3>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={form.Code} onChange={e=>setForm({...form,Code:e.target.value.toUpperCase()})} placeholder="ONLINE"/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})} placeholder="Online"/></div>
      </div>
      <div className="alert">Suggested starting channels for MyanKafe: WALK_IN, ONLINE, PHONE, OTHER. You can define different channels per entity.</div>
      <button type="button" disabled={busy||!form.Code.trim()||!form.Name.trim()} onClick={create}>Create sales channel</button>
    </div>

    <div className="table-wrap" aria-busy={loading}>
      <table>
        <thead><tr><th>Code</th><th>Name</th><th>Status</th><th></th></tr></thead>
        <tbody>
          {items.map(item=><tr key={item.id}>
            <td>{item.code}</td><td>{item.name}</td>
            <td><span className={`badge ${item.active?"POSTED":"VOIDED"}`}>{item.active?"ACTIVE":"INACTIVE"}</span></td>
            <td><button type="button" className="secondary compact" onClick={()=>startEdit(item)}>Edit</button></td>
          </tr>)}
          <TableStateRows loading={loading&&items.length===0} empty={!loading&&items.length===0} columns={4} emptyText="No sales channels defined for this entity yet."/>
        </tbody>
      </table>
    </div>
  </>;
}
