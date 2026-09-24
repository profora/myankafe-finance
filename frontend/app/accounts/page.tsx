"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";

export default function Accounts() {
  const { entity } = useEntity();
  const [items,setItems] = useState<Account[]>([]);
  const [error,setError] = useState("");
  const [form,setForm] = useState({Code:"",Name:"",Type:"EXPENSE",Subtype:"",ParentPublicID:"",Postable:true});

  const load=()=>{if(entity)api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity]);

  async function create(){
    if(!entity)return;
    try{
      await api(`/entities/${entity.PublicID}/accounts`,{method:"POST",body:JSON.stringify(form)});
      setForm({Code:"",Name:"",Type:"EXPENSE",Subtype:"",ParentPublicID:"",Postable:true});
      load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Chart of Accounts</h1><p>Hierarchical accounts for {entity?.Name}.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="card form" style={{marginBottom:16}}>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={form.Code} onChange={e=>setForm({...form,Code:e.target.value})}/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={form.Type} onChange={e=>setForm({...form,Type:e.target.value})}>{["ASSET","LIABILITY","EQUITY","INCOME","EXPENSE"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Parent</label><select value={form.ParentPublicID} onChange={e=>setForm({...form,ParentPublicID:e.target.value})}><option value="">None</option>{items.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Subtype</label><input value={form.Subtype} onChange={e=>setForm({...form,Subtype:e.target.value})}/></div>
      </div>
      <button disabled={!form.Code||!form.Name} onClick={create}>Create account</button>
    </div>
    <div className="table-wrap"><table><thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Subtype</th><th>Posting</th></tr></thead><tbody>{items.map(a=><tr key={a.PublicID}><td><Link className="table-link" href={`/accounts/${a.PublicID}/ledger`}>{a.Code}</Link></td><td><Link className="table-link" href={`/accounts/${a.PublicID}/ledger`}>{a.Name}</Link></td><td>{a.Type}</td><td>{a.Subtype??"—"}</td><td>{a.Postable?"Yes":"Header"}</td></tr>)}</tbody></table></div>
  </>;
}
