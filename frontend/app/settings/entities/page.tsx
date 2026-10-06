"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import { canConfigureAccounting, canManageEntitySettings } from "@/lib/permissions";
import { useActiveCurrencies } from "@/components/CurrencySelect";
import FiscalYearFields from "@/components/FiscalYearFields";
import { fiscalYearLabel } from "@/lib/fiscal";

export default function EntitySettings(){
  const {entities,entity,reload,platformOwner}=useEntity();
  const mayEdit=canManageEntitySettings(entity?.Role);
  const maySetStart=canConfigureAccounting(entity?.Role);
  const [startDate,setStartDate]=useState("");
  const mayCreate=entities.some(x=>x.Role==="OWNER")||platformOwner;
  const {items:currencies}=useActiveCurrencies();
  const [err,setErr]=useState("");
  const [msg,setMsg]=useState("");
  const [busy,setBusy]=useState(false);
  const [f,setF]=useState({Code:"",Name:"",EntityType:"BUSINESS",FunctionalCurrency:"MMK",Timezone:"Asia/Yangon",FiscalMonth:1,FiscalDay:1});
  const [edit,setEdit]=useState({Name:"",Timezone:"Asia/Yangon",FiscalMonth:1,FiscalDay:1});

  useEffect(()=>{
    if(!entity)return;
    setEdit({Name:entity.Name,Timezone:entity.Timezone,FiscalMonth:entity.FiscalMonth,FiscalDay:entity.FiscalDay});
    setStartDate(entity.AccountingStartDate??"");
  },[entity?.PublicID,entity?.AccountingStartDate]);

  async function create(){
    setErr("");setMsg("");setBusy(true);
    try{
      await api("/entities",{method:"POST",body:JSON.stringify(f)});
      setMsg("Entity created.");
      setF({...f,Code:"",Name:""});
      reload();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function saveStartDate(){
    if(!entity)return;
    setErr("");setMsg("");setBusy(true);
    try{
      await api(`/entities/${entity.PublicID}/accounting-start-date`,{method:"PUT",body:JSON.stringify({AccountingStartDate:startDate||null})});
      setMsg("Accounting start date updated.");
      reload();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function saveEntity(){
    if(!entity)return;
    setErr("");setMsg("");setBusy(true);
    try{
      await api(`/entities/${entity.PublicID}/`,{method:"PUT",body:JSON.stringify(edit)});
      setMsg("Entity settings updated.");
      reload();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Entities</h1><p>Create and configure independently accounted businesses or personal entities.</p></div></div>
    {err&&<div className="alert error">{err}</div>}
    {msg&&<div className="alert success">{msg}</div>}

    {entity&&mayEdit&&<div className="card form" style={{marginBottom:16}}>
      <h3>Settings for {entity.Name}</h3>
      <div className="alert">Entity code, type and functional currency are protected accounting identity fields after creation.</div>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={entity.Code} disabled/></div>
        <div className="field"><label>Type</label><input value={entity.Type} disabled/></div>
        <div className="field"><label>Functional currency</label><input value={entity.FunctionalCurrency} disabled/></div>
        <div className="field"><label>Name</label><input value={edit.Name} onChange={e=>setEdit({...edit,Name:e.target.value})}/></div>
        <div className="field"><label>Timezone</label><input value={edit.Timezone} onChange={e=>setEdit({...edit,Timezone:e.target.value})} placeholder="Asia/Yangon"/></div>
        <FiscalYearFields month={edit.FiscalMonth} day={edit.FiscalDay} onChange={(FiscalMonth,FiscalDay)=>setEdit({...edit,FiscalMonth,FiscalDay})}/>
      </div>
      <button disabled={busy||!edit.Name.trim()||!edit.Timezone.trim()||edit.FiscalDay<1} onClick={saveEntity}>{busy?"Saving…":"Save entity settings"}</button>
    </div>}

    {entity&&maySetStart&&<div className="card form" style={{marginBottom:16}}>
      <h3>Accounting start date</h3>
      {!entity.AccountingStartDate&&<div className="alert">Set the accounting start date before entering accounting transactions.</div>}
      <p>Finance is authoritative from this date. It is separate from the fiscal year and becomes fixed after opening balances or posted accounting exist.</p>
      <div className="field"><label htmlFor="accounting-start-date">Accounting start date</label><input id="accounting-start-date" type="date" value={startDate} onChange={e=>setStartDate(e.target.value)}/></div>
      <button disabled={busy} onClick={saveStartDate}>{busy?"Saving…":"Save accounting start date"}</button>
    </div>}

    {mayCreate&&<div className="card form" style={{marginBottom:16}}>
      <h3>New entity</h3>
      <div className="form-grid">
        <div className="field"><label>Code</label><input value={f.Code} onChange={e=>setF({...f,Code:e.target.value.toUpperCase()})}/></div>
        <div className="field"><label>Name</label><input value={f.Name} onChange={e=>setF({...f,Name:e.target.value})}/></div>
        <div className="field"><label>Type</label><select value={f.EntityType} onChange={e=>setF({...f,EntityType:e.target.value})}><option>BUSINESS</option><option>PERSONAL</option><option>OTHER</option></select></div>
        <div className="field"><label>Functional currency</label><select value={f.FunctionalCurrency} onChange={e=>setF({...f,FunctionalCurrency:e.target.value})}><option value="">Choose…</option>{currencies.map(item=><option key={item.code} value={item.code}>{item.code} · {item.name}</option>)}</select></div>
        <div className="field"><label>Timezone</label><input value={f.Timezone} onChange={e=>setF({...f,Timezone:e.target.value})}/></div>
        <FiscalYearFields month={f.FiscalMonth} day={f.FiscalDay} onChange={(FiscalMonth,FiscalDay)=>setF({...f,FiscalMonth,FiscalDay})}/>
      </div>
      <button disabled={busy||!f.Code||!f.Name||f.FiscalDay<1} onClick={create}>Create entity</button>
    </div>}

    {!mayEdit&&!maySetStart&&entity&&<div className="alert">Your {entity.Role} role can view this entity but cannot change entity settings.</div>}
    {!mayCreate&&<div className="alert">Creating additional entities requires OWNER access somewhere in the platform.</div>}

    <div className="table-wrap"><table><thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Currency</th><th>Timezone</th><th>Fiscal year</th><th>Your role</th></tr></thead><tbody>{entities.map(x=><tr key={x.PublicID}><td>{x.Code}</td><td>{x.Name}</td><td>{x.Type}</td><td>{x.FunctionalCurrency}</td><td>{x.Timezone}</td><td>{fiscalYearLabel(x.FiscalMonth,x.FiscalDay)||"—"}</td><td><span className="badge">{x.Role}</span></td></tr>)}</tbody></table></div>
  </>;
}
