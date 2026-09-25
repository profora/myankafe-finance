"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import ConfirmDialog from "@/components/ConfirmDialog";
import { canLockAccounting, canUnlockAccounting } from "@/lib/permissions";

type State={
  locked_through?:string|null;
  LockedThrough?:string|null;
  reason?:string|null;
  Reason?:string|null;
};

export default function Locking(){
  const {entity}=useEntity();
  const mayLock=canLockAccounting(entity?.Role);
  const mayUnlock=canUnlockAccounting(entity?.Role);
  const [state,setState]=useState<State>({});
  const [date,setDate]=useState("");
  const [lockReason,setLockReason]=useState("");
  const [unlockReason,setUnlockReason]=useState("");
  const [confirmUnlock,setConfirmUnlock]=useState(false);
  const [busy,setBusy]=useState(false);
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");

  const load=()=>{
    if(!entity)return;
    api<State>(`/entities/${entity.PublicID}/accounting-lock`)
      .then(setState)
      .catch(e=>setError(e instanceof Error?e.message:String(e)));
  };

  useEffect(()=>{
    setMessage("");setError("");setDate("");setLockReason("");setUnlockReason("");setConfirmUnlock(false);
    load();
  },[entity?.PublicID]);

  const current=state.locked_through??state.LockedThrough??null;

  async function lock(){
    if(!entity||!date||!lockReason.trim())return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/accounting-lock`,{
        method:"POST",
        body:JSON.stringify({locked_through:date,reason:lockReason.trim()}),
      });
      setMessage(`Transactions locked through ${date}.`);
      setLockReason("");setDate("");
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function unlock(){
    if(!entity||!current||!unlockReason.trim())return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/accounting-lock/unlock`,{
        method:"POST",
        body:JSON.stringify({reason:unlockReason.trim()}),
      });
      setMessage("Accounting period unlocked. The action was recorded in the audit log.");
      setUnlockReason("");setConfirmUnlock(false);
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  return <>
    <div className="page-head">
      <div>
        <h1>Transaction Locking</h1>
        <p>OWNER and ACCOUNTANT can lock accounting dates. Only OWNER can unlock a previously locked period.</p>
      </div>
    </div>

    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}

    <div className="grid locking-grid">
      <div className="card">
        <div className="muted">Current accounting status</div>
        <div className="metric">{current?`Locked through ${current}`:"Open"}</div>
        <p className="muted">Posting, reversal, re-dating and other accounting changes on or before the locked-through date are rejected by both the API and PostgreSQL.</p>
      </div>

      {mayLock&&<div className="card form">
        <div>
          <h3 style={{margin:"0 0 4px"}}>Lock transactions</h3>
          <p className="muted" style={{margin:0}}>Close accounting through a date after review or month-end.</p>
        </div>
        <div className="field">
          <label>Lock through</label>
          <input type="date" value={date} onChange={e=>setDate(e.target.value)}/>
        </div>
        <div className="field">
          <label>Reason</label>
          <textarea rows={3} value={lockReason} onChange={e=>setLockReason(e.target.value)} placeholder="Example: August 2026 month-end review completed"/>
        </div>
        <div className="actions">
          <button disabled={busy||!date||!lockReason.trim()} onClick={lock}>{busy?"Working…":"Lock transactions"}</button>
        </div>
      </div>}

      {mayUnlock&&<div className="card form">
        <div>
          <h3 style={{margin:"0 0 4px"}}>Owner unlock</h3>
          <p className="muted" style={{margin:0}}>Unlocking reopens accounting history. A separate reason is required and audited.</p>
        </div>
        <div className="field">
          <label>Unlock reason</label>
          <textarea rows={3} value={unlockReason} onChange={e=>setUnlockReason(e.target.value)} placeholder="Explain why the locked period must be reopened"/>
        </div>
        <div className="actions">
          <button className="danger" disabled={busy||!current||!unlockReason.trim()} onClick={()=>setConfirmUnlock(true)}>Review unlock</button>
        </div>
      </div>}
    </div>

    {!mayLock&&<div className="alert">Your {entity?.Role??"VIEWER"} role can view the lock state but cannot close accounting periods.</div>}
    {mayLock&&!mayUnlock&&<div className="alert">ACCOUNTANT may advance the lock but only OWNER may reopen a locked period.</div>}

    <ConfirmDialog
      open={mayUnlock&&confirmUnlock}
      title="Reopen locked accounting period?"
      description={current
        ? `This will remove the lock through ${current}. Historical transactions in that period may become mutable again according to normal permissions. The unlock will be permanently audited.`
        : "There is no active accounting lock."}
      confirmLabel="Unlock accounting period"
      danger
      busy={busy}
      onCancel={()=>{if(!busy)setConfirmUnlock(false)}}
      onConfirm={unlock}
    >
      <div className="alert" style={{marginTop:16,marginBottom:0}}>
        <strong>Entity:</strong> {entity?.Name??"—"}<br/>
        <strong>Unlock reason:</strong> {unlockReason||"—"}
      </div>
    </ConfirmDialog>
  </>;
}
