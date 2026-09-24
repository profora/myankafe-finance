"use client";

import { FormEvent, useEffect, useState } from "react";
import { api } from "@/lib/api";

type Session={
  id:string;
  user_agent?:string|null;
  ip_address?:string|null;
  expires_at:string;
  last_seen_at:string;
  created_at:string;
  current:boolean;
};

export default function SecuritySettings(){
  const [currentPassword,setCurrentPassword]=useState("");
  const [newPassword,setNewPassword]=useState("");
  const [confirm,setConfirm]=useState("");
  const [busy,setBusy]=useState(false);
  const [sessionBusy,setSessionBusy]=useState("");
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [sessions,setSessions]=useState<Session[]>([]);

  async function loadSessions(){
    try{
      const result=await api<{items:Session[]}>("/auth/sessions");
      setSessions(result.items);
    }catch(err){setError(err instanceof Error?err.message:String(err))}
  }

  useEffect(()=>{loadSessions()},[]);

  async function submit(e:FormEvent){
    e.preventDefault();setError("");setMessage("");
    if(newPassword!==confirm){setError("New passwords do not match.");return}
    setBusy(true);
    try{
      await api("/auth/change-password",{method:"POST",body:JSON.stringify({current_password:currentPassword,new_password:newPassword})});
      setCurrentPassword("");setNewPassword("");setConfirm("");
      setMessage("Password changed. Other sessions were revoked.");
      await loadSessions();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  async function revokeSession(session:Session){
    setSessionBusy(session.id);setError("");setMessage("");
    try{
      await api(`/auth/sessions/${session.id}`,{method:"DELETE"});
      if(session.current){window.location.assign("/login");return}
      setMessage("Session revoked.");
      await loadSessions();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setSessionBusy("")}
  }

  async function revokeOthers(){
    setSessionBusy("others");setError("");setMessage("");
    try{
      const result=await api<{revoked:number}>("/auth/sessions/revoke-others",{method:"POST",body:"{}"});
      setMessage(`${result.revoked} other session${result.revoked===1?"":"s"} revoked.`);
      await loadSessions();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setSessionBusy("")}
  }

  function displayAgent(value?:string|null){
    if(!value)return "Unknown device";
    if(/Android/i.test(value))return "Android · "+value;
    if(/iPhone|iPad/i.test(value))return "iPhone/iPad · "+value;
    if(/Macintosh/i.test(value))return "Mac · "+value;
    if(/Windows/i.test(value))return "Windows · "+value;
    return value;
  }

  return <>
    <div className="page-head"><div><h1>Security</h1><p>Password and active login sessions.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}

    <div className="grid security-grid">
      <div className="card">
        <h3>Change password</h3>
        <form className="form" onSubmit={submit}>
          <div className="field"><label>Current password</label><input type="password" autoComplete="current-password" value={currentPassword} onChange={e=>setCurrentPassword(e.target.value)}/></div>
          <div className="field"><label>New password</label><input type="password" autoComplete="new-password" value={newPassword} onChange={e=>setNewPassword(e.target.value)}/><span className="muted">Minimum 12 characters.</span></div>
          <div className="field"><label>Confirm new password</label><input type="password" autoComplete="new-password" value={confirm} onChange={e=>setConfirm(e.target.value)}/></div>
          <button disabled={busy||!currentPassword||newPassword.length<12||newPassword!==confirm}>{busy?"Changing…":"Change password"}</button>
        </form>
      </div>

      <div className="card">
        <div className="page-head" style={{marginBottom:12}}>
          <div><h3 style={{margin:0}}>Active sessions</h3><p style={{marginTop:4}}>Review devices signed into your finance account.</p></div>
          <button className="secondary" disabled={sessionBusy==="others"||sessions.filter(x=>!x.current).length===0} onClick={revokeOthers}>{sessionBusy==="others"?"Revoking…":"Sign out other devices"}</button>
        </div>
        <div className="session-list">
          {sessions.length?sessions.map(session=><div className="session-row" key={session.id}>
            <div>
              <div><strong>{session.current?"This device":"Active session"}</strong>{session.current&&<span className="badge POSTED" style={{marginLeft:8}}>CURRENT</span>}</div>
              <div className="muted session-agent">{displayAgent(session.user_agent)}</div>
              <div className="muted">IP {session.ip_address??"unknown"} · Last active {new Date(session.last_seen_at).toLocaleString()} · Expires {new Date(session.expires_at).toLocaleString()}</div>
            </div>
            <button className={session.current?"danger":"secondary"} disabled={Boolean(sessionBusy)} onClick={()=>revokeSession(session)}>{sessionBusy===session.id?"Revoking…":session.current?"Sign out":"Revoke"}</button>
          </div>):<div className="empty">No active sessions found.</div>}
        </div>
      </div>
    </div>
  </>;
}
