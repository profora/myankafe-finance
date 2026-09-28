"use client";

import { useEffect, useId, useRef } from "react";
import type { Account } from "@/components/types";
import { changeDraftType, isAccountType, parentOptions, type AccountDraft } from "@/lib/accountTree";

const accountTypes = ["ASSET", "LIABILITY", "EQUITY", "INCOME", "EXPENSE"] as const;

type Props = {
  draft: AccountDraft | null;
  accounts: Account[];
  busy: boolean;
  dirty: boolean;
  onChange: (draft: AccountDraft) => void;
  onClose: () => void;
  onSubmit: () => void;
};

export default function AccountModal({ draft, accounts, busy, dirty, onChange, onClose, onSubmit }: Props) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleID = useId();
  const nameRef = useRef<HTMLInputElement>(null);
  const codeRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (draft && !dialog.open) dialog.showModal();
    if (!draft && dialog.open) dialog.close();
  }, [draft]);

  useEffect(() => {
    if (!draft) return;
    const field = draft.mode === "create" ? codeRef.current : nameRef.current;
    field?.focus();
  }, [draft?.mode, draft?.publicID]);

  function requestClose() {
    if (busy) return;
    if (dirty && !window.confirm("Discard unsaved account changes?")) return;
    onClose();
  }

  const options = draft ? parentOptions(accounts, draft.type, draft.mode === "edit" ? draft.publicID : "") : [];
  const codeValid = draft ? /^\d{4}$/.test(draft.code) : false;

  return <dialog ref={ref} className="dialog account-dialog" aria-labelledby={titleID} aria-busy={busy || undefined} onCancel={event => { event.preventDefault(); requestClose(); }}>
    {draft && <form className="dialog-card" onSubmit={event => { event.preventDefault(); if (!busy) onSubmit(); }}>
      <h2 id={titleID}>{draft.mode === "create" ? "New account" : "Edit account"}</h2>
      <p className="muted">{draft.mode === "create" ? "Add a posting or header account to this entity." : "Name, parent, subtype, and status can change. Code, type, and posting identity stay fixed."}</p>
      {draft.error && <div className="alert error" role="alert">{draft.error}</div>}
      <div className="form-grid">
        <div className="field"><label htmlFor="account-code">Code</label><input id="account-code" ref={codeRef} inputMode="numeric" value={draft.code} readOnly={draft.mode === "edit"} disabled={draft.mode === "edit"} onChange={event => onChange({ ...draft, code: event.target.value, error: "" })} /></div>
        <div className="field"><label htmlFor="account-name">Name</label><input id="account-name" ref={nameRef} value={draft.name} onChange={event => onChange({ ...draft, name: event.target.value, error: "" })} /></div>
        <div className="field"><label htmlFor="account-type">Type</label><select id="account-type" value={draft.type} disabled={draft.mode === "edit"} onChange={event => { if (isAccountType(event.target.value)) onChange(changeDraftType(draft, event.target.value, accounts)); }}>{accountTypes.map(type => <option key={type}>{type}</option>)}</select></div>
        <div className="field"><label htmlFor="account-parent">Parent account</label><select id="account-parent" value={draft.parentPublicID} disabled={draft.hierarchyLocked} onChange={event => onChange({ ...draft, parentPublicID: event.target.value, error: "" })}>
          <option value="">None</option>
          {options.map(account => <option key={account.PublicID} value={account.PublicID}>{account.Code} · {account.Name}</option>)}
        </select></div>
        <div className="field"><label htmlFor="account-subtype">Subtype</label><input id="account-subtype" value={draft.subtype} onChange={event => onChange({ ...draft, subtype: event.target.value, error: "" })} /></div>
        <div className="field"><label htmlFor="account-kind">Account kind</label><select id="account-kind" value={draft.postable ? "POSTING" : "HEADER"} disabled={draft.mode === "edit"} onChange={event => onChange({ ...draft, postable: event.target.value === "POSTING", error: "" })}>
          <option value="POSTING">Posting account</option>
          <option value="HEADER">Header account</option>
        </select></div>
        {draft.mode === "edit" && <div className="field"><label htmlFor="account-status">Status</label><select id="account-status" value={draft.active ? "ACTIVE" : "INACTIVE"} onChange={event => onChange({ ...draft, active: event.target.value === "ACTIVE", error: "" })}>
          <option value="ACTIVE">Active</option>
          <option value="INACTIVE">Inactive</option>
        </select></div>}
      </div>
      {draft.hierarchyLocked && <p className="muted">This system account keeps its chart position.</p>}
      {draft.mode === "create" && draft.code !== "" && !codeValid && <p className="muted">Code must be exactly four digits.</p>}
      <div className="actions dialog-actions">
        <button type="button" className="secondary" disabled={busy} onClick={requestClose}>Cancel</button>
        <button type="submit" disabled={busy || !draft.name.trim() || !codeValid}>{busy ? "Saving…" : draft.mode === "create" ? "Create account" : "Save changes"}</button>
      </div>
    </form>}
  </dialog>;
}
