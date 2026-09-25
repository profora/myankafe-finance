"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";
import { canConfigureAccounting } from "@/lib/permissions";
import TableStateRows from "@/components/TableStateRows";

export default function Accounts() {
  const { entity } = useEntity();
  const mayConfigure=canConfigureAccounting(entity?.Role);
  const [items,setItems] = useState<Account[]>([]);
  const [error,setError] = useState("");
  const [message,setMessage] = useState("");
  const [busy,setBusy] = useState(false);
  const [loading,setLoading] = useState(true);
  const [form,setForm] = useState({Code:"",Name:"",Type:"EXPENSE",Subtype:"",ParentPublicID:"",Postable:true});
  const [editing,setEditing]=useState<Account|null>(null);
  const [edit,setEdit]=useState({Name:"",Subtype:"",Active:true});

  const load=async()=>{
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const result=await api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`);
      setItems(result.items);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  };
  useEffect(()=>{
    if(!entity)return;
    setItems([]);
    setEditing(null);
    void load();
  },[entity?.PublicID]);

  async function create(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/accounts`,{method:"POST",body:JSON.stringify(form)});
      setForm({Code:"",Name:"",Type:"EXPENSE",Subtype:"",ParentPublicID:"",Postable:true});
      setMessage("Account created.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(account:Account){
    setEditing(account);
    setEdit({Name:account.Name,Subtype:account.Subtype??"",Active:account.Active});
    setError("");setMessage("");
  }

  async function saveEdit(){
    if(!entity||!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/accounts/${editing.PublicID}`,{
        method:"PUT",
        body:JSON.stringify(edit),
      });
      setEditing(null);setMessage("Account updated.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Chart of Accounts</h1><p>Hierarchical accounts for {entity?.Name}.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}

    {mayConfigure&&editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}>
        <div><h1 style={{fontSize:20}}>Edit account</h1><p>{editing.Code} · {editing.Type} · {editing.Postable?"Posting account":"Header account"}</p></div>
        <button type="button" className="secondary" onClick={()=>setEditing(null)}>Cancel</button>
      </div>
      <div className="alert">Code, fundamental account type, hierarchy and posting/header identity are protected accounting fields in this editor.</div>
      <div className="form-grid">
        <div className="field"><label>Name</label><input value={edit.Name} onChange={e=>setEdit({...edit,Name:e.target.value})}/></div>
        <div className="field"><label>Subtype</label><input value={edit.Subtype} onChange={e=>setEdit({...edit,Subtype:e.target.value})}/></div>
        <div className="field"><label>Status</label><select value={edit.Active?"ACTIVE":"INACTIVE"} onChange={e=>setEdit({...edit,Active:e.target.value==="ACTIVE"})}><option>ACTIVE</option><option>INACTIVE</option></select></div>
      </div>
      <div className="actions">
        <button type="button" disabled={busy||!edit.Name.trim()} onClick={saveEdit}>{busy?"Saving…":"Save changes"}</button>
        <span className="muted">To deactivate a parent, deactivate active descendants and linked financial accounts first.</span>
      </div>
    </div>}

    {mayConfigure&&<div className="card form" style={{marginBottom:16}}>
      <h3>New account</h3>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={form.Code} onChange={e=>setForm({...form,Code:e.target.value})}/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={form.Type} onChange={e=>setForm({...form,Type:e.target.value})}>{["ASSET","LIABILITY","EQUITY","INCOME","EXPENSE"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Parent</label><select value={form.ParentPublicID} onChange={e=>setForm({...form,ParentPublicID:e.target.value})}><option value="">None</option>{items.filter(a=>a.Active).map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Subtype</label><input value={form.Subtype} onChange={e=>setForm({...form,Subtype:e.target.value})}/></div>
      </div>
      <button type="button" disabled={busy||!form.Code||!form.Name} onClick={create}>Create account</button>
    </div>}

    {!mayConfigure&&<div className="alert">Your {entity?.Role??"VIEWER"} role can view the Chart of Accounts and ledgers but cannot change account configuration.</div>}

    <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Subtype</th><th>Posting</th><th>Status</th><th></th></tr></thead><tbody>{items.map(a=><tr key={a.PublicID}>
      <td><Link className="table-link" href={`/accounts/${a.PublicID}/ledger`}>{a.Code}</Link></td>
      <td><Link className="table-link" href={`/accounts/${a.PublicID}/ledger`}>{a.Name}</Link></td>
      <td>{a.Type}</td><td>{a.Subtype??"—"}</td><td>{a.Postable?"Yes":"Header"}</td>
      <td><span className={`badge ${a.Active?"POSTED":"VOIDED"}`}>{a.Active?"ACTIVE":"INACTIVE"}</span></td>
      <td>{mayConfigure?<button type="button" className="secondary compact" onClick={()=>startEdit(a)}>Edit</button>:<span className="muted">Read-only</span>}</td>
    </tr>)}<TableStateRows loading={loading&&items.length===0} empty={!loading&&items.length===0} columns={7} emptyText="No accounts configured for this entity."/></tbody></table></div>
  </>;
}
