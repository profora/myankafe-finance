"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import ConfirmDialog from "@/components/ConfirmDialog";
import TableStateRows from "@/components/TableStateRows";

type User={id:string;username:string;display_name:string;email?:string;status:string};
type Member={id:string;username:string;display_name:string;email?:string;role:string};

export default function Users(){
  const {entities,entity,platformOwner}=useEntity();
  const ownerAnywhere=platformOwner||entities.some(x=>x.Role==="OWNER");
  const mayViewEntityAccess=entity?.Role==="OWNER"||entity?.Role==="ADMIN";
  const mayManageEntityAccess=entity?.Role==="OWNER";
  const [users,setUsers]=useState<User[]>([]);
  const [members,setMembers]=useState<Member[]>([]);
  const [err,setErr]=useState("");
  const [msg,setMsg]=useState("");
  const [showPassword,setShowPassword]=useState(false);
  const [f,setF]=useState({Username:"",DisplayName:"",Email:"",Password:""});
  const [assign,setAssign]=useState({user_id:"",role:"BOOKKEEPER"});
  const [reset,setReset]=useState({user_id:"",password:""});
  const [showResetPassword,setShowResetPassword]=useState(false);
  const [statusTarget,setStatusTarget]=useState<User|null>(null);
  const [statusBusy,setStatusBusy]=useState(false);
  const [accessTarget,setAccessTarget]=useState<Member|null>(null);
  const [accessBusy,setAccessBusy]=useState(false);
  const [loading,setLoading]=useState(true);

  const load=async()=>{
    setErr("");
    if(!ownerAnywhere){setUsers([]);setMembers([]);setLoading(false);return}
    setLoading(true);
    try{
      const [u,m]=await Promise.all([
        api<{items:User[]}>("/users"),
        entity&&mayViewEntityAccess?api<{items:Member[]}>(`/entities/${entity.PublicID}/users`):Promise.resolve({items:[]})
      ]);
      setUsers(u.items);
      setMembers(m.items);
      const activeUsers=u.items.filter(x=>x.status==="ACTIVE");
      setAssign(a=>({...a,user_id:activeUsers.some(x=>x.id===a.user_id)?a.user_id:(activeUsers[0]?.id||"")}));
      setReset(a=>({...a,user_id:u.items.some(x=>x.id===a.user_id)?a.user_id:(u.items[0]?.id||"")}));
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  };

  useEffect(()=>{
    setUsers([]);
    setMembers([]);
    void load();
  },[entity?.PublicID,ownerAnywhere,mayViewEntityAccess]);

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

  async function setUserStatus(){
    if(!statusTarget)return;
    const next=statusTarget.status==="ACTIVE"?"DISABLED":"ACTIVE";
    setStatusBusy(true);setErr("");setMsg("");
    try{
      await api(`/users/${statusTarget.id}/status`,{method:"PUT",body:JSON.stringify({status:next})});
      setMsg(next==="DISABLED"?"User disabled and active sessions revoked.":"User reactivated.");
      setStatusTarget(null);
      load();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setStatusBusy(false)}
  }

  async function revokeEntityAccess(){
    if(!entity||!accessTarget)return;
    setAccessBusy(true);setErr("");setMsg("");
    try{
      await api(`/entities/${entity.PublicID}/users/${accessTarget.id}`,{method:"DELETE"});
      setMsg(`Access removed from ${entity.Name}.`);
      setAccessTarget(null);
      load();
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
    finally{setAccessBusy(false)}
  }

  async function resetPassword(){
    if(!reset.user_id||reset.password.length<12)return;
    setErr("");setMsg("");
    try{
      await api(`/users/${reset.user_id}/reset-password`,{
        method:"POST",
        body:JSON.stringify({new_password:reset.password}),
      });
      setMsg("Password reset. All existing sessions for that user were revoked.");
      setReset(v=>({...v,password:""}));
      setShowResetPassword(false);
    }catch(e){setErr(e instanceof Error?e.message:String(e))}
  }

  return <>
    <div className="page-head">
      <div><h1>Users & Access</h1><p>Platform identities, initial credentials, and per-entity roles.</p></div>
    </div>
    {err&&<div className="alert error" role="alert">{err}</div>}
    {msg&&<div className="alert success" role="status" aria-live="polite">{msg}</div>}
    {!ownerAnywhere&&<div className="alert error">Platform user administration requires OWNER access on at least one entity.</div>}

    {ownerAnywhere&&<div className="grid" style={{gridTemplateColumns:"repeat(auto-fit,minmax(min(100%,360px),1fr))",marginBottom:16}}>
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
            <button type="button" className="secondary compact" aria-pressed={showPassword} onClick={()=>setShowPassword(x=>!x)}>{showPassword?"Hide":"Show"}</button>
          </div>
          <div className="muted">Minimum 12 characters. Share it securely; the user can change it after signing in.</div>
        </div>
        <button type="button" disabled={!f.Username||!f.DisplayName||f.Password.length<12} onClick={create}>Create user</button>
      </div>

      {mayManageEntityAccess&&<div className="card form">
        <h3>Role for {entity?.Name}</h3>
        <div className="field">
          <label>User</label>
          <select value={assign.user_id} onChange={e=>setAssign({...assign,user_id:e.target.value})}>
            {users.filter(x=>x.status==="ACTIVE").map(x=><option key={x.id} value={x.id}>{x.display_name} · {x.username}</option>)}
          </select>
        </div>
        <div className="field">
          <label>Role</label>
          <select value={assign.role} onChange={e=>setAssign({...assign,role:e.target.value})}>
            {["OWNER","ADMIN","ACCOUNTANT","BOOKKEEPER","VIEWER"].map(x=><option key={x}>{x}</option>)}
          </select>
        </div>
        <button type="button" disabled={!assign.user_id||!entity} onClick={setRole}>Set entity role</button>
      </div>}

      <div className="card form">
        <h3>Reset user password</h3>
        <p className="muted">OWNER only. Resetting a password immediately revokes all existing sessions for that user.</p>
        <div className="field">
          <label>User</label>
          <select value={reset.user_id} onChange={e=>setReset({...reset,user_id:e.target.value})}>
            {users.map(x=><option key={x.id} value={x.id}>{x.display_name} · {x.username}</option>)}
          </select>
        </div>
        <div className="field">
          <label>New password</label>
          <div className="password-field">
            <input
              type={showResetPassword?"text":"password"}
              autoComplete="new-password"
              value={reset.password}
              onChange={e=>setReset({...reset,password:e.target.value})}
              placeholder="At least 12 characters"
            />
            <button type="button" className="secondary compact" aria-pressed={showResetPassword} onClick={()=>setShowResetPassword(x=>!x)}>{showResetPassword?"Hide":"Show"}</button>
          </div>
        </div>
        <button type="button" className="danger" disabled={!reset.user_id||reset.password.length<12} onClick={resetPassword}>Reset password & revoke sessions</button>
      </div>
    </div>}

    {ownerAnywhere&&<><div className="page-head" style={{marginTop:24}}><div><h1 style={{fontSize:20}}>Platform users</h1><p>Disable access without deleting accounting history or role assignments.</p></div></div>
    <div className="table-wrap" style={{marginBottom:18}} aria-busy={loading}>
      <table>
        <thead><tr><th>User</th><th>Username</th><th>Email</th><th>Status</th><th></th></tr></thead>
        <tbody>{users.map(x=><tr key={x.id}>
          <td>{x.display_name}</td>
          <td>{x.username}</td>
          <td>{x.email||"—"}</td>
          <td><span className={`badge ${x.status==="ACTIVE"?"POSTED":"VOIDED"}`}>{x.status}</span></td>
          <td><button type="button" className={x.status==="ACTIVE"?"danger":"secondary"} onClick={()=>setStatusTarget(x)}>{x.status==="ACTIVE"?"Disable":"Reactivate"}</button></td>
        </tr>)}<TableStateRows loading={loading&&users.length===0} empty={!loading&&users.length===0} columns={5} emptyText="No platform users found."/></tbody>
      </table>
    </div>

    {mayViewEntityAccess&&<><div className="page-head"><div><h1 style={{fontSize:20}}>Access for {entity?.Name}</h1><p>Current active entity-role assignments.</p></div></div>
    <div className="table-wrap" aria-busy={loading}>
      <table>
        <thead><tr><th>User</th><th>Username</th><th>Role</th><th>Email</th><th></th></tr></thead>
        <tbody>{members.map(x=><tr key={x.id}><td>{x.display_name}</td><td>{x.username}</td><td>{x.role}</td><td>{x.email||"—"}</td><td>{mayManageEntityAccess?<button type="button" className="danger compact" onClick={()=>setAccessTarget(x)}>Remove access</button>:<span className="muted">ADMIN view</span>}</td></tr>)}<TableStateRows loading={loading&&members.length===0} empty={!loading&&members.length===0} columns={5} emptyText="No users currently have access to this entity."/></tbody>
      </table>
    </div></>}</>}

    <ConfirmDialog
      open={Boolean(statusTarget)}
      title={statusTarget?.status==="ACTIVE"?"Disable user?":"Reactivate user?"}
      description={statusTarget?.status==="ACTIVE"
        ?"The user will be signed out everywhere immediately. Accounting history and entity roles remain intact."
        :"The user will be allowed to sign in again using their existing password and retained entity roles."}
      confirmLabel={statusTarget?.status==="ACTIVE"?"Disable user":"Reactivate user"}
      danger={statusTarget?.status==="ACTIVE"}
      busy={statusBusy}
      onCancel={()=>{if(!statusBusy)setStatusTarget(null)}}
      onConfirm={setUserStatus}
    >
      {statusTarget&&<p><strong>{statusTarget.display_name}</strong> · {statusTarget.username}</p>}
    </ConfirmDialog>

    <ConfirmDialog
      open={Boolean(accessTarget)}
      title="Remove entity access?"
      description={`This revokes access to ${entity?.Name??"this entity"} only. The user account and access to other entities remain unchanged.`}
      confirmLabel="Remove access"
      danger
      busy={accessBusy}
      onCancel={()=>{if(!accessBusy)setAccessTarget(null)}}
      onConfirm={revokeEntityAccess}
    >
      {accessTarget&&<p><strong>{accessTarget.display_name}</strong> · {accessTarget.role}</p>}
    </ConfirmDialog>
  </>;
}
