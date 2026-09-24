"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account, FinancialAccount } from "@/components/types";

export default function FinancialAccounts() {
  const { entity } = useEntity();
  const [items,setItems]=useState<FinancialAccount[]>([]);
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [error,setError]=useState("");
  const [form,setForm]=useState({Code:"",Name:"",Kind:"BANK",Currency:"MMK",AccountPublicID:"",Institution:"",Reference:""});

  const load=()=>{
    if(!entity)return;
    Promise.all([
      api<{items:FinancialAccount[]}>(`/entities/${entity.PublicID}/financial-accounts`),
      api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`)
    ]).then(([f,a])=>{setItems(f.items);setAccounts(a.items.filter(x=>x.Postable&&["ASSET","LIABILITY"].includes(x.Type)))}).catch(e=>setError(e.message));
  };
  useEffect(load,[entity]);

  async function create(){
    if(!entity)return;
    try{
      await api(`/entities/${entity.PublicID}/financial-accounts`,{method:"POST",body:JSON.stringify(form)});
      setForm({...form,Code:"",Name:"",AccountPublicID:"",Institution:"",Reference:""});
      load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head"><div><h1>Cash / Bank Accounts</h1><p>Cash, banks, mobile wallets, cards and other definable accounts.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="card form" style={{marginBottom:16}}>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={form.Code} onChange={e=>setForm({...form,Code:e.target.value.toUpperCase().replace(/\s+/g,"_")})}/></div>
        <div className="field"><label>Name</label><input value={form.Name} onChange={e=>setForm({...form,Name:e.target.value})}/></div>
        <div className="field"><label>Kind</label><select value={form.Kind} onChange={e=>setForm({...form,Kind:e.target.value})}>{["CASH","BANK","MOBILE_WALLET","CREDIT_CARD","OTHER"].map(x=><option key={x}>{x}</option>)}</select></div>
        <div className="field"><label>Currency</label><input value={form.Currency} onChange={e=>setForm({...form,Currency:e.target.value.toUpperCase()})}/></div>
        <div className="field span-2"><label>Linked COA account</label><select value={form.AccountPublicID} onChange={e=>setForm({...form,AccountPublicID:e.target.value})}><option value="">Choose…</option>{accounts.map(a=><option key={a.PublicID} value={a.PublicID}>{a.Code} · {a.Name}</option>)}</select></div>
        <div className="field"><label>Institution</label><input value={form.Institution} onChange={e=>setForm({...form,Institution:e.target.value})}/></div>
        <div className="field"><label>Reference</label><input value={form.Reference} onChange={e=>setForm({...form,Reference:e.target.value})}/></div>
      </div>
      <button disabled={!form.Code||!form.Name||!form.AccountPublicID} onClick={create}>Add financial account</button>
    </div>
    <div className="table-wrap"><table><thead><tr><th>Code</th><th>Name</th><th>Kind</th><th>Currency</th><th>Institution</th></tr></thead><tbody>{items.map(x=><tr key={x.PublicID}><td>{x.Code}</td><td>{x.Name}</td><td>{x.Kind}</td><td>{x.Currency}</td><td>{x.Institution??"—"}</td></tr>)}</tbody></table></div>
  </>;
}
