"use client";

import { FormEvent, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";

export default function LoginPage(){
  const router=useRouter();
  const params=useSearchParams();
  const [username,setUsername]=useState("");
  const [password,setPassword]=useState("");
  const [showPassword,setShowPassword]=useState(false);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");

  const expired=useMemo(()=>params.get("expired")==="1",[params]);
  const next=useMemo(()=>{
    const raw=params.get("next");
    return raw&&raw.startsWith("/")&&!raw.startsWith("//")?raw:"/";
  },[params]);

  async function submit(e:FormEvent){
    e.preventDefault();
    setBusy(true);setError("");
    try{
      await api("/auth/login",{method:"POST",body:JSON.stringify({username,password})});
      router.replace(next);router.refresh();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  return <main className="login-page">
    <div className="login-card">
      <div className="brand login-brand">MyanKafe <span>Finance</span></div>
      <h1>Sign in</h1>
      <p className="muted">Access is limited to active finance users.</p>
      {expired&&!error&&<div className="alert">Your session expired. Please sign in again.</div>}
      {error&&<div className="alert error">{error}</div>}
      <form className="form" onSubmit={submit}>
        <div className="field"><label htmlFor="username">Username</label><input id="username" autoComplete="username" autoFocus required value={username} onChange={e=>setUsername(e.target.value)}/></div>
        <div className="field">
          <label htmlFor="password">Password</label>
          <div className="password-row">
            <input id="password" type={showPassword?"text":"password"} autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)}/>
            <button type="button" className="secondary password-toggle" onClick={()=>setShowPassword(v=>!v)}>{showPassword?"Hide":"Show"}</button>
          </div>
        </div>
        <button disabled={busy||!username||!password}>{busy?"Signing in…":"Sign in"}</button>
      </form>
    </div>
  </main>;
}
