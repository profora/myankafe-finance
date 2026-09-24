"use client";
import {useState} from "react";
import {api} from "@/lib/api";
import {useEntity} from "@/components/EntityContext";
export default function EntitySettings(){
 const {entities,reload}=useEntity(); const [err,setErr]=useState(""); const [msg,setMsg]=useState("");
 const [f,setF]=useState({Code:"",Name:"",EntityType:"BUSINESS",FunctionalCurrency:"MMK",Timezone:"Asia/Yangon",FiscalMonth:1,FiscalDay:1});
 async function create(){setErr("");setMsg("");try{await api("/entities",{method:"POST",body:JSON.stringify(f)});setMsg("Entity created.");setF({...f,Code:"",Name:""});reload()}catch(e){setErr(e instanceof Error?e.message:String(e))}}
 return <><div className="page-head"><div><h1>Entities</h1><p>Create independently accounted businesses or personal entities.</p></div></div>
 {err&&<div className="alert error">{err}</div>}{msg&&<div className="alert success">{msg}</div>}
 <div className="card form" style={{marginBottom:16}}><div className="form-grid">
 <div className="field"><label>Code</label><input value={f.Code} onChange={e=>setF({...f,Code:e.target.value.toUpperCase()})}/></div>
 <div className="field"><label>Name</label><input value={f.Name} onChange={e=>setF({...f,Name:e.target.value})}/></div>
 <div className="field"><label>Type</label><select value={f.EntityType} onChange={e=>setF({...f,EntityType:e.target.value})}><option>BUSINESS</option><option>PERSONAL</option><option>OTHER</option></select></div>
 <div className="field"><label>Functional currency</label><input value={f.FunctionalCurrency} onChange={e=>setF({...f,FunctionalCurrency:e.target.value.toUpperCase()})}/></div>
 <div className="field"><label>Timezone</label><input value={f.Timezone} onChange={e=>setF({...f,Timezone:e.target.value})}/></div>
 <div className="field"><label>Fiscal year start</label><div style={{display:"flex",gap:8}}><input type="number" min={1} max={12} value={f.FiscalMonth} onChange={e=>setF({...f,FiscalMonth:Number(e.target.value)})}/><input type="number" min={1} max={31} value={f.FiscalDay} onChange={e=>setF({...f,FiscalDay:Number(e.target.value)})}/></div></div>
 </div><button disabled={!f.Code||!f.Name} onClick={create}>Create entity</button></div>
 <div className="table-wrap"><table><thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Currency</th><th>Fiscal year</th></tr></thead><tbody>{entities.map(x=><tr key={x.PublicID}><td>{x.Code}</td><td>{x.Name}</td><td>{x.Type}</td><td>{x.FunctionalCurrency}</td><td>{x.FiscalMonth}/{x.FiscalDay}</td></tr>)}</tbody></table></div></>}
