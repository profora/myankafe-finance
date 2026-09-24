"use client";
import { useState } from "react";
import { currentDevUser, setDevUser } from "@/lib/api";

export default function DevLogin() {
  const [value, setValue] = useState(() => currentDevUser());
  return <div className="card form">
    <h1>Development user</h1>
    <p className="muted">Paste the owner ULID printed by the bootstrap command. Production authentication is intentionally separate.</p>
    <div className="field"><label>User ULID</label><input value={value} onChange={(e)=>setValue(e.target.value)} /></div>
    <button disabled={value.length!==26} onClick={()=>{setDevUser(value);location.assign("/")}}>Use development user</button>
  </div>;
}
