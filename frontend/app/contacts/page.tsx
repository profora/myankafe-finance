"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Contact={id:string;contact_type:string;display_name:string;phone?:string;email?:string;active:boolean};

export default function Contacts(){
  const {entity}=useEntity();
  const [items,setItems]=useState<Contact[]>([]);
  const [error,setError]=useState("");
  const [form,setForm]=useState({contact_type:"OTHER",display_name:"",phone:"",email:"",notes:""});

  const load=()=>{if(entity)api<{items:Contact[]}>(`/entities/${entity.PublicID}/contacts`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity]);

  async function create(){
    if(!entity)return;
    try{await api(`/entities/${entity.PublicID}/contacts`,{method:"POST",body:JSON.stringify(form)});setForm({contact_type:"OTHER",display_name:"",phone:"",email:"",notes:""});load()}
    catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Contacts</h1><p>Entity-scoped payees, payers, suppliers, customers and owners.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="card form" style={{marginBottom:16}}>
      <div className="form-grid">
        <div className="field"><label>Name</label><input value={form.display_name} onChange={e=>setForm({...form,display_name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={form.contact_type} onChange={e=>setForm({...form,contact_type:e.target.value})}>{["OTHER","SUPPLIER","CUSTOMER","EMPLOYEE","OWNER"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Phone</label><input value={form.phone} onChange={e=>setForm({...form,phone:e.target.value})}/></div>
        <div className="field"><label>Email</label><input value={form.email} onChange={e=>setForm({...form,email:e.target.value})}/></div>
        <div className="field span-2"><label>Notes</label><input value={form.notes} onChange={e=>setForm({...form,notes:e.target.value})}/></div>
      </div>
      <button disabled={!form.display_name.trim()} onClick={create}>Create contact</button>
    </div>
    <div className="table-wrap"><table><thead><tr><th>Name</th><th>Type</th><th>Phone</th><th>Email</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{x.display_name}</td><td>{x.contact_type}</td><td>{x.phone||"—"}</td><td>{x.email||"—"}</td></tr>)}</tbody></table></div>
  </>;
}
