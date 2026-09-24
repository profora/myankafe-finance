"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type User={id:string;username:string;display_name:string;email?:string;status:string};
type Member={id:string;username:string;display_name:string;email?:string;role:string};

export default function Users(){
  const {entity}=useEntity();
  const [users,setUsers]=useState<User[]>([]);
  const [members,setMembers]=useState<Member[]>([]);
  const [err,setErr]=useState("");
  const [msg,setMsg]=useState("");
  const [showPassword,setShowPassword]=useState(false);
  const [f,setF]=useState({Username:"",DisplayName:"",Email:"",Password:""});
  const [assign,setAssign]=useState({user_id:"",role:"BOOKKEEPER"});

  const load=()=>{
    setErr("");
    Promise.all([
      api<{items:User[]}>("/users"),
      entity?api<{items:Member[]}>(`/entities/${entity.PublicID}/users`):Promise.resolve({items:[]})
    ]).then(([u,m])=>{
      setUsers(u.items);
      setMembers(m.items);
      setAssign(a=>({...a,user_id:a.user_id||u.items[0]?.id||""}));setReset(a=>({...a,user_id:a.user_id||u.items[0]?.id||""}));
    }).catch(e=>setErr(e instanceof Error?e.message:String(e)));
  };

  useEffect(load,[entity?.PublicID]);

  async function create(){
    setErr("");setMsg("");
    try{
      await api("/users",{method:"POST",body:JSON.stringify(f)});
      setMsg("User created. Assign an entity role before they begin work.");
      setF({Username:"",DisplayName:"",Email:"",Password:""});
      setShowPassword(false);
      load();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
  }

  async function setRole(){
    if(!entity)return;
    setErr("");setMsg("");
    try{
      await api(`/entities/${entity.PublicID}/users/role`,{method:"PUT",body:JSON.stringify(assign)});
      setMsg("Entity role updated.");
      load();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head">
      <div><h1>Users & Access</h1><p>Platform identities, initial credentials, and per-entity roles.</p></div>
    </div>
    {err&&<div className="alert error">{err}</div>}
    {msg&&<div className="alert success">{msg}</div>}

    <div className="grid" style={{gridTemplateColumns:"repeat(auto-fit,minmax(360px,1fr))",marginBottom:16}}>
      <div className="card form">
        <h3>New user</h3>
        <div className="field">
          <label>Username</label>
          <input autoComplete="off" value={f.Username} onChange={e=>setF({...f,Username:e.target.value.toLowerCase().trimStart()})} placeholder="staff.name"/>
        </div>
        <div className="field">
          <label>Display name</label>
          <input value={f.DisplayName} onChange={e=>setF({...f,DisplayName:e.target.value})}/>
        </div>
        <div className="field">
          <label>Email</label>
          <input type="email" autoComplete="off" value={f.Email} onChange={e=>setF({...f,Email:e.target.value})}/>
        </div>
        <div className="field">
          <label>Initial password</label>
          <div className="password-field">
            <input
              type={showPassword?"text":"password"}
              autoComplete="new-password"
              value={f.Password}
              onChange={e=>setF({...f,Password:e.target.value})}
              placeholder="At least 12 characters"
            />
            <button type="button" className="secondary compact" onClick={()=>setShowPassword(x=>!x)}>{showPassword?"Hide":"Show"}</button>
          </div>
          <div className="muted">Minimum 12 characters. Share it securely; the user can change it after signing in.</div>
        </div>
        <button disabled={!f.Username||!f.DisplayName||f.Password.length<12} onClick={create}>Create user</button>
      </div>

      <div className="card form">
        <h3>Role for {entity?.Name}</h3>
        <div className="field">
          <label>User</label>
          <select value={assign.user_id} onChange={e=>setAssign({...assign,user_id:e.target.value})}>
            {users.map(x=><option key={x.id} value={x.id}>{x.display_name} · {x.username}</option>)}
          </select>
        </div>
        <div className="field">
          <label>Role</label>
          <select value={assign.role} onChange={e=>setAssign({...assign,role:e.target.value})}>
            {["OWNER","ADMIN","ACCOUNTANT","BOOKKEEPER","VIEWER"].map(x=><option key={x}>{x}</option>)}
          </select>
        </div>
        <button disabled={!assign.user_id||!entity} onClick={setRole}>Set entity role</button>
      </div>
    </div>

    <div className="table-wrap">
      <table>
        <thead><tr><th>User</th><th>Username</th><th>Role</th><th>Email</th></tr></thead>
        <tbody>{members.map(x=><tr key={x.id}><td>{x.display_name}</td><td>{x.username}</td><td>{x.role}</td><td>{x.email||"—"}</td></tr>)}</tbody>
      </table>
    </div>
  </>;
}
