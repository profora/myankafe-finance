"use client";

import { useEffect, useRef } from "react";

type Props={
  open:boolean;
  title:string;
  description:string;
  confirmLabel:string;
  danger?:boolean;
  busy?:boolean;
  children?:React.ReactNode;
  onCancel:()=>void;
  onConfirm:()=>void;
};

export default function ConfirmDialog({open,title,description,confirmLabel,danger,busy,children,onCancel,onConfirm}:Props){
  const ref=useRef<HTMLDialogElement>(null);

  useEffect(()=>{
    const dialog=ref.current;
    if(!dialog)return;
    if(open&&!dialog.open)dialog.showModal();
    if(!open&&dialog.open)dialog.close();
  },[open]);

  return <dialog ref={ref} className="dialog" onCancel={e=>{e.preventDefault();if(!busy)onCancel()}}>
    <div className="dialog-card">
      <h2>{title}</h2>
      <p className="muted">{description}</p>
      {children}
      <div className="actions dialog-actions">
        <button className="secondary" disabled={busy} onClick={onCancel}>Cancel</button>
        <button className={danger?"danger":""} disabled={busy} onClick={onConfirm}>{busy?"Working…":confirmLabel}</button>
      </div>
    </div>
  </dialog>;
}
