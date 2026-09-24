"use client";

import { FormEvent, useState } from "react";
import { ApiError, clearDevUser, login } from "@/lib/api";

export default function LoginPage(){
  const [username,setUsername]=useState("");
  const [password,setPassword]=useState("");
  const [error,setError]=useState("");
  const [busy,setBusy]=useState(false);

  async function submit(e:FormEvent){
    e.preventDefault();
    setBusy(true);setError("");
    try{
      clearDevUser();
      await login(username,password);
      window.location.assign("/");
    }catch(e){
      if(e instanceof ApiError){
        setError(e.status===401?"Invalid username or password.":e.message);
      }else{
        setError(e instanceof Error?e.message:String(e));
      }
    }finally{setBusy(false)}
  }

  return <div className="auth-card">
    <div className="brand auth-brand">MyanKafe <span>Finance</span></div>
    <div>
      <h1>Sign in</h1>
      <p className="muted">Use your MyanKafe Finance username and password.</p>
    </div>
    {error&&<div className="alert error">{error}</div>}
    <form className="form" onSubmit={submit}>
      <div className="field">
        <label>Username</label>
        <input
          autoFocus
          autoComplete="username"
          value={username}
          onChange={e=>setUsername(e.target.value)}
          placeholder="owner"
        />
      </div>
      <div className="field">
        <label>Password</label>
        <input
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={e=>setPassword(e.target.value)}
        />
      </div>
      <button type="submit" disabled={busy||!username.trim()||!password}>{busy?"Signing in…":"Sign in"}</button>
    </form>
    <p className="muted auth-footnote">Sessions are stored in an HttpOnly cookie. Development ULID login remains available at <a className="table-link" href="/dev-login">/dev-login</a> when the backend is running in dev auth mode.</p>
  </div>;
}
