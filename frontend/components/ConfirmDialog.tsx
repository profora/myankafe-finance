"use client";

import { useEffect, useId, useRef } from "react";

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
  const titleID=useId();
  const descriptionID=useId();

  useEffect(()=>{
    const dialog=ref.current;
    if(!dialog)return;
    if(open&&!dialog.open)dialog.showModal();
    if(!open&&dialog.open)dialog.close();
  },[open]);

  return <dialog ref={ref} className="dialog" aria-labelledby={titleID} aria-describedby={descriptionID} aria-busy={busy||undefined} onCancel={e=>{e.preventDefault();if(!busy)onCancel()}}>
    <div className="dialog-card">
      <h2 id={titleID}>{title}</h2>
      <p id={descriptionID} className="muted">{description}</p>
      {children}
      <div className="actions dialog-actions">
        <button type="button" className="secondary" disabled={busy} onClick={onCancel}>Cancel</button>
        <button type="button" className={danger?"danger":""} disabled={busy} onClick={onConfirm}>{busy?"Working…":confirmLabel}</button>
      </div>
    </div>
  </dialog>;
}
