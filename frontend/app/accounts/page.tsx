"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";
import { canConfigureAccounting } from "@/lib/permissions";
import { withEntity } from "@/lib/entitySelection";
import { accountMutationControls, addChildDraft, afterAccountSave, blankDraft, buildAccountTree, draftChanged, editDraft, type AccountDraft } from "@/lib/accountTree";
import AccountModal from "@/components/AccountModal";
import TableStateRows from "@/components/TableStateRows";

export default function Accounts() {
  const { entity } = useEntity();
  const mayConfigure = canConfigureAccounting(entity?.Role);
  const controls = accountMutationControls(entity?.Role);
  const [items, setItems] = useState<Account[]>([]);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState<AccountDraft | null>(null);
  const [initial, setInitial] = useState<AccountDraft | null>(null);
  const opener = useRef<HTMLButtonElement | null>(null);

  const load = async () => {
    if (!entity) return;
    setLoading(true); setError("");
    try {
      const result = await api<{ items: Account[] }>(`/entities/${entity.PublicID}/accounts`);
      setItems(result.items ?? []);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  };
  useEffect(() => {
    if (!entity) return;
    setItems([]);
    setDraft(null);
    void load();
  }, [entity?.PublicID]);

  function openDraft(next: AccountDraft, button: HTMLButtonElement) {
    opener.current = button;
    setDraft(next);
    setInitial(next);
    setError("");
    setMessage("");
  }

  function closeDraft() {
    setDraft(null);
    setInitial(null);
    opener.current?.focus();
  }

  async function save() {
    if (!entity || !draft) return;
    setBusy(true);
    setDraft({ ...draft, error: "" });
    try {
      if (draft.mode === "create") {
        await api(`/entities/${entity.PublicID}/accounts`, {
          method: "POST",
          body: JSON.stringify({
            Code: draft.code,
            Name: draft.name.trim(),
            Type: draft.type,
            Subtype: draft.subtype.trim(),
            ParentPublicID: draft.parentPublicID,
            Postable: draft.postable,
          }),
        });
        setMessage("Account created.");
      } else {
        await api(`/entities/${entity.PublicID}/accounts/${draft.publicID}`, {
          method: "PUT",
          body: JSON.stringify({
            Name: draft.name.trim(),
            Subtype: draft.subtype.trim(),
            ParentPublicID: draft.parentPublicID,
            Active: draft.active,
          }),
        });
        setMessage("Account updated.");
      }
      const next = afterAccountSave(draft, "");
      setDraft(next);
      setInitial(next);
      opener.current?.focus();
      await load();
    } catch (e) {
      const messageText = e instanceof Error ? e.message : String(e);
      setDraft(afterAccountSave(draft, messageText));
    } finally { setBusy(false); }
  }

  const tree = buildAccountTree(items);

  return <>
    <div className="page-head">
      <div><h1>Chart of Accounts</h1><p>Hierarchical accounts for {entity?.Name}.</p></div>
      {controls.create && <button type="button" onClick={event => openDraft(blankDraft(), event.currentTarget)}>+ New account</button>}
    </div>
    {error && <div className="alert error" role="alert">{error}</div>}
    {message && <div className="alert success" role="status" aria-live="polite">{message}</div>}
    {!mayConfigure && <div className="alert">Your {entity?.Role ?? "VIEWER"} role can view the Chart of Accounts and ledgers but cannot change account configuration.</div>}

    <AccountModal
      draft={draft}
      accounts={items}
      busy={busy}
      dirty={Boolean(draft && initial && draftChanged(draft, initial))}
      onChange={setDraft}
      onClose={closeDraft}
      onSubmit={() => { void save(); }}
    />

    <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Code</th><th>Name</th><th>Type</th><th>Subtype</th><th>Posting</th><th>Status</th><th></th></tr></thead><tbody>{tree.map(({ account, depth }) => <tr key={account.PublicID}>
      <td><Link className="table-link" href={withEntity(`/accounts/${account.PublicID}/ledger`, entity?.PublicID ?? "")}>{account.Code}</Link></td>
      <td><span className="account-name" style={{ paddingLeft: depth * 16 }}><Link className="table-link" href={withEntity(`/accounts/${account.PublicID}/ledger`, entity?.PublicID ?? "")}>{account.Name}</Link></span></td>
      <td>{account.Type}</td><td>{account.Subtype || "—"}</td><td>{account.Postable ? "Posting" : "Header"}</td>
      <td><span className={`badge ${account.Active ? "POSTED" : "VOIDED"}`}>{account.Active ? "ACTIVE" : "INACTIVE"}</span></td>
      <td>{controls.edit ? <div className="account-actions">
        <button type="button" className="secondary compact" onClick={event => openDraft(editDraft(account), event.currentTarget)}>Edit</button>
        {controls.addChild && !account.Postable && account.Active && <button type="button" className="secondary compact" onClick={event => openDraft(addChildDraft(account), event.currentTarget)}>Add child</button>}
      </div> : <span className="muted">Read-only</span>}</td>
    </tr>)}<TableStateRows loading={loading && items.length === 0} empty={!loading && items.length === 0} columns={7} emptyText="No accounts configured for this entity." /></tbody></table></div>
  </>;
}
