"use client";

import { FormEvent, useState } from "react";
import { api } from "@/lib/api";

export default function SecuritySettings(){
  const [currentPassword,setCurrentPassword]=useState("");
  const [newPassword,setNewPassword]=useState("");
  const [confirm,setConfirm]=useState("");
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");

  async function submit(e:FormEvent){
    e.preventDefault();setError("");setMessage("");
    if(newPassword!==confirm){setError("New passwords do not match.");return}
    setBusy(true);
    try{
      await api("/auth/change-password",{method:"POST",body:JSON.stringify({current_password:currentPassword,new_password:newPassword})});
      setCurrentPassword("");setNewPassword("");setConfirm("");
      setMessage("Password changed. Other sessions were revoked.");
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head"><div><h1>Security</h1><p>Change your finance login password.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}
    <div className="card" style={{maxWidth:620}}>
      <form className="form" onSubmit={submit}>
        <div className="field"><label>Current password</label><input type="password" autoComplete="current-password" value={currentPassword} onChange={e=>setCurrentPassword(e.target.value)}/></div>
        <div className="field"><label>New password</label><input type="password" autoComplete="new-password" value={newPassword} onChange={e=>setNewPassword(e.target.value)}/><span className="muted">Minimum 12 characters.</span></div>
        <div className="field"><label>Confirm new password</label><input type="password" autoComplete="new-password" value={confirm} onChange={e=>setConfirm(e.target.value)}/></div>
        <button disabled={busy||!currentPassword||newPassword.length<12||newPassword!==confirm}>{busy?"Changing…":"Change password"}</button>
      </form>
    </div>
  </>;
}
