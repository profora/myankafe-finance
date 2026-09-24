"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Contact={
  id:string;
  contact_type:string;
  display_name:string;
  phone?:string|null;
  email?:string|null;
  notes?:string|null;
  active:boolean;
};

type ContactForm={
  contact_type:string;
  display_name:string;
  phone:string;
  email:string;
  notes:string;
};

const blank:ContactForm={contact_type:"OTHER",display_name:"",phone:"",email:"",notes:""};

export default function Contacts(){
  const {entity}=useEntity();
  const [items,setItems]=useState<Contact[]>([]);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [form,setForm]=useState<ContactForm>(blank);
  const [editing,setEditing]=useState<Contact|null>(null);
  const [edit,setEdit]=useState<ContactForm&{active:boolean}>({...blank,active:true});
  const [busy,setBusy]=useState(false);

  const load=()=>{if(entity)api<{items:Contact[]}>(`/entities/${entity.PublicID}/contacts`).then(x=>setItems(x.items)).catch(e=>setError(e.message))};
  useEffect(load,[entity?.PublicID]);

  async function create(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/contacts`,{method:"POST",body:JSON.stringify(form)});
      setForm(blank);setMessage("Contact created.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  function startEdit(contact:Contact){
    setEditing(contact);
    setEdit({
      contact_type:contact.contact_type,
      display_name:contact.display_name,
      phone:contact.phone??"",
      email:contact.email??"",
      notes:contact.notes??"",
      active:contact.active,
    });
    setError("");setMessage("");
  }

  async function saveEdit(){
    if(!entity||!editing)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/contacts/${editing.id}`,{method:"PUT",body:JSON.stringify({
        Type:edit.contact_type,
        DisplayName:edit.display_name,
        Phone:edit.phone,
        Email:edit.email,
        Notes:edit.notes,
        Active:edit.active,
      })});
      setEditing(null);setMessage("Contact updated.");load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Contacts</h1><p>Entity-scoped payees, payers, suppliers, customers and owners.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}

    {editing&&<div className="card form" style={{marginBottom:16}}>
      <div className="page-head" style={{marginBottom:0}}>
        <div><h1 style={{fontSize:20}}>Edit contact</h1><p>{editing.display_name}</p></div>
        <button className="secondary" onClick={()=>setEditing(null)}>Cancel</button>
      </div>
      <div className="form-grid">
        <div className="field"><label>Name</label><input value={edit.display_name} onChange={e=>setEdit({...edit,display_name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={edit.contact_type} onChange={e=>setEdit({...edit,contact_type:e.target.value})}>{["OTHER","SUPPLIER","CUSTOMER","EMPLOYEE","OWNER"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Phone</label><input value={edit.phone} onChange={e=>setEdit({...edit,phone:e.target.value})}/></div>
        <div className="field"><label>Email</label><input type="email" value={edit.email} onChange={e=>setEdit({...edit,email:e.target.value})}/></div>
        <div className="field span-2"><label>Notes</label><textarea rows={3} value={edit.notes} onChange={e=>setEdit({...edit,notes:e.target.value})}/></div>
        <div className="field"><label>Status</label><select value={edit.active?"ACTIVE":"INACTIVE"} onChange={e=>setEdit({...edit,active:e.target.value==="ACTIVE"})}><option>ACTIVE</option><option>INACTIVE</option></select></div>
      </div>
      <div className="actions"><button disabled={busy||!edit.display_name.trim()} onClick={saveEdit}>{busy?"Saving…":"Save changes"}</button><span className="muted">Inactive contacts remain on historical transactions but cannot be selected for new entries.</span></div>
    </div>}

    <div className="card form" style={{marginBottom:16}}>
      <h3>New contact</h3>
      <div className="form-grid">
        <div className="field"><label>Name</label><input value={form.display_name} onChange={e=>setForm({...form,display_name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={form.contact_type} onChange={e=>setForm({...form,contact_type:e.target.value})}>{["OTHER","SUPPLIER","CUSTOMER","EMPLOYEE","OWNER"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Phone</label><input value={form.phone} onChange={e=>setForm({...form,phone:e.target.value})}/></div>
        <div className="field"><label>Email</label><input type="email" value={form.email} onChange={e=>setForm({...form,email:e.target.value})}/></div>
        <div className="field span-2"><label>Notes</label><input value={form.notes} onChange={e=>setForm({...form,notes:e.target.value})}/></div>
      </div>
      <button disabled={busy||!form.display_name.trim()} onClick={create}>Create contact</button>
    </div>

    <div className="table-wrap"><table><thead><tr><th>Name</th><th>Type</th><th>Phone</th><th>Email</th><th>Status</th><th></th></tr></thead><tbody>{items.map(x=><tr key={x.id}>
      <td>{x.display_name}</td><td>{x.contact_type}</td><td>{x.phone||"—"}</td><td>{x.email||"—"}</td>
      <td><span className={`badge ${x.active?"POSTED":"VOIDED"}`}>{x.active?"ACTIVE":"INACTIVE"}</span></td>
      <td><button className="secondary compact" onClick={()=>startEdit(x)}>Edit</button></td>
    </tr>)}</tbody></table></div>
  </>;
}
