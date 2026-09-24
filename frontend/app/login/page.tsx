"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";

export default function LoginPage(){
  const router=useRouter();
  const [username,setUsername]=useState("");
  const [password,setPassword]=useState("");
  const [showPassword,setShowPassword]=useState(false);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");

  const [expired,setExpired]=useState(false);
  const [next,setNext]=useState("/");

  useEffect(()=>{
    const params=new URLSearchParams(window.location.search);
    setExpired(params.get("expired")==="1");
    const raw=params.get("next");
    if(raw&&raw.startsWith("/")&&!raw.startsWith("//"))setNext(raw);
  },[]);

  async function submit(e:FormEvent){
    e.preventDefault();
    setBusy(true);setError("");
    try{
      await api("/auth/login",{method:"POST",body:JSON.stringify({username,password})});
      router.replace(next);router.refresh();
    }catch(err){setError(err instanceof Error?err.message:String(err))}
    finally{setBusy(false)}
  }

  return <main className="login-page finance-login-page">
    <section className="login-brand-panel">
      <img className="brand-logo-hero" src="/brand/chieftain-logo.webp" alt="Chieftain Chin Coffee"/>
      <div>
        <div className="login-kicker">MyanKafe · Finance workspace</div>
        <h1>MyanKafe Finance</h1>
        <p>Multi-entity double-entry accounting for MyanKafe, Royal Masterpiece, Personal, and future entities.</p>
      </div>
      <small>Authenticated finance users only</small>
    </section>
    <section className="login-form-side">
      <div className="login-card">
      <h1>Sign in</h1>
      <p className="muted">Sign in with your finance username and password.</p>
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
    </section>
  </main>;
}
