"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount } from "@/components/types";
import { canConfigureAccounting } from "@/lib/permissions";
import TableStateRows from "@/components/TableStateRows";

export default function FinancialAccounts() {
  const { entity } = useEntity();
  const mayConfigure=canConfigureAccounting(entity?.Role);
  const [items,setItems]=useState<FinancialAccount[]>([]);
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [busy,setBusy]=useState(false);
  const [loading,setLoading]=useState(true);
  const [form,setForm]=useState({Code:"",Name:"",Kind:"BANK",Currency:"MMK",AccountPublicID:"",Institution:"",Reference:""});
  const [editing,setEditing]=useState<FinancialAccount|null>(null);
  const [edit,setEdit]=useState({Name:"",Institution:"",Reference:"",Active:true});

  const load=async()=>{
    if(!entity)return;
    setLoading(true);setError("");
    try{
      const [f,a]=await Promise.all([
        api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
        api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`)
      ]);
      setItems(f.items);
      setAccounts(a.items.filter(x=>x.Postable&&x.Active&&["ASSET","LIABILITY"].includes(x.Type)));
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  };
  useEffect(()=>{
    if(!entity)return;
    setItems([]);
    setAccounts([]);
    setEditing(null);
    void load();
  },[entity?.PublicID]);

  async function create(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/financial-accounts`,{method:"POST",body:JSON.stringify(form)});
      setForm({...form,Code:"",Name:"",AccountPublicID:"",Institution:"",Reference:""});
      setMessage("Financial account created.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(item:FinancialAccount){
    setEditing(item);
    setEdit({Name:item.Name,Institution:item.Institution??"",Reference:item.Reference??"",Active:item.Active});
    setError("");setMessage("");
  }

  async function saveEdit(){
    if(!entity||!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/financial-accounts/${editing.PublicID}`,{
        method:"PUT",
        body:JSON.stringify(edit),
      });
      setEditing(null);setMessage("Financial account updated.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Cash / Bank Accounts</h1><p>Cash, banks, mobile wallets, cards and other definable accounts.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {message&&<div className="alert success" role="status" aria-live="polite">{message}</div>}

    {mayConfigure&&editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}>
        <div><h1 style={{fontSize:20}}>Edit financial account</h1><p>{editing.Code} · {editing.Kind} · {editing.Currency}</p></div>
        <button type="button" className="secondary" onClick={()=>setEditing(null)}>Cancel</button>
      </div>
      <div className="alert">Code, kind, currency and linked COA account are accounting identity fields and are not changed here.</div>
      <div className="form-grid">
        <div className="field"><label>Name</label><input value={edit.Name} onChange={e=>setEdit({...edit,Name:e.target.value})}/></div>
        <div className="field"><label>Status</label><select value={edit.Active?"ACTIVE":"INACTIVE"} onChange={e=>setEdit({...edit,Active:e.target.value==="ACTIVE"})}><option>ACTIVE</option><option>INACTIVE</option></select></div>
        <div className="field"><label>Institution</label><input value={edit.Institution} onChange={e=>setEdit({...edit,Institution:e.target.value})}/></div>
        <div className="field"><label>Reference</label><input value={edit.Reference} onChange={e=>setEdit({...edit,Reference:e.target.value})}/></div>
      </div>
      <div className="actions"><button type="button" disabled={busy||!edit.Name.trim()} onClick={saveEdit}>{busy?"Saving…":"Save changes"}</button><span className="muted">Inactive accounts remain in historical reports but cannot be selected for new entries or transfers.</span></div>
    </div>}

    {mayConfigure&&<div className="card form" style={{marginBottom:16}}>
      <h3>New financial account</h3>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={form.Code} onChange={e=>setForm({...form,Code:e.target.value.toUpperCase().replace(/\s+/g,"_")})}/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})}/></div>
        <div className="field"><label>Kind</label><select value={form.Kind} onChange={e=>setForm({...form,Kind:e.target.value})}>{["CASH","BANK","MOBILE_WALLET","CREDIT_CARD","OTHER"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Currency</label><input value={form.Currency} onChange={e=>setForm({...form,Currency:e.target.value.toUpperCase()})}/></div>
        <div className="field span-2"><label>Linked COA account</label><select value={form.AccountPublicID} onChange={e=>setForm({...form,AccountPublicID:e.target.value})}><option value="">Choose…</option>{accounts.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Institution</label><input value={form.Institution} onChange={e=>setForm({...form,Institution:e.target.value})}/></div>
        <div className="field"><label>Reference</label><input value={form.Reference} onChange={e=>setForm({...form,Reference:e.target.value})}/></div>
      </div>
      <button type="button" disabled={busy||!form.Code||!form.Name||!form.AccountPublicID} onClick={create}>Add financial account</button>
    </div>}

    {!mayConfigure&&<div className="alert">Your {entity?.Role??"VIEWER"} role can view financial accounts but cannot change their accounting configuration.</div>}

    <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Code</th><th>Name</th><th>Kind</th><th>Currency</th><th>Institution</th><th>Reference</th><th>Status</th><th></th></tr></thead><tbody>{items.map(x=><tr key={x.PublicID}>
      <td>{x.Code}</td><td>{x.Name}</td><td>{x.Kind}</td><td>{x.Currency}</td><td>{x.Institution??"—"}</td><td>{x.Reference??"—"}</td>
      <td><span className={`badge ${x.Active?"POSTED":"VOIDED"}`}>{x.Active?"ACTIVE":"INACTIVE"}</span></td>
      <td>{mayConfigure?<button type="button" className="secondary compact" onClick={()=>startEdit(x)}>Edit</button>:<span className="muted">Read-only</span>}</td>
    </tr>)}<TableStateRows loading={loading&&items.length===0} empty={!loading&&items.length===0} columns={8} emptyText="No cash, bank, wallet, or card accounts configured yet."/></tbody></table></div>
  </>;
}
