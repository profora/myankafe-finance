"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import type { Account } from "@/components/types";

type LedgerLine = {
  date:string;
  journal_id:string;
  transaction_id?:string|null;
  journal_description:string;
  description:string;
  debit:string;
  credit:string;
  running_balance:string;
  currency:string;
};

function firstOfYear(){
  const d=new Date();
  return `${d.getFullYear()}-01-01`;
}

export default function AccountLedgerPage(){
  const {entity}=useEntity();
  const params=useParams<{id:string}>();
  const accountID=Array.isArray(params.id)?params.id[0]:params.id;
  const [accounts,setAccounts]=useState<Account[]>([]);
  const [items,setItems]=useState<LedgerLine[]>([]);
  const [from,setFrom]=useState(firstOfYear);
  const [to,setTo]=useState(()=>new Date().toISOString().slice(0,10));
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(false);

  const account=useMemo(()=>accounts.find(x=>x.PublicID===accountID),[accounts,accountID]);

  async function load(){
    if(!entity||!accountID)return;
    setLoading(true);setError("");
    try{
      const [coa,ledger]=await Promise.all([
        api<{items:Account[]}>(`/entities/${entity.PublicID}/accounts`),
        api<{items:LedgerLine[]}>(`/entities/${entity.PublicID}/reports/account-ledger?account_id=${encodeURIComponent(accountID)}&from=${from}&to=${to}&limit=2000`)
      ]);
      setAccounts(coa.items);setItems(ledger.items);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{load()},[entity,accountID]);

  return <>
    <div className="page-head">
      <div>
        <div className="muted"><Link href="/accounts">Chart of Accounts</Link> / Ledger</div>
        <h1>{account ? `${account.Code} · ${account.Name}` : "Account Ledger"}</h1>
        <p>Running balance includes activity before the selected start date.</p>
      </div>
    </div>

    {error&&<div className="alert error">{error}</div>}

    <div className="card form" style={{marginBottom:16}}>
      <div className="form-grid">
        <div className="field"><label>From</label><input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></div>
        <div className="field"><label>To</label><input type="date" value={to} onChange={e=>setTo(e.target.value)}/></div>
      </div>
      <div className="actions">
        <button disabled={loading||!from||!to||from>to} onClick={load}>{loading?"Loading…":"Apply range"}</button>
        <span className="muted">{items.length} ledger lines</span>
      </div>
    </div>

    <div className="table-wrap">
      {loading && !items.length ? <div className="empty">Loading ledger…</div> :
      items.length ? <table>
        <thead><tr><th>Date</th><th>Description</th><th>Debit</th><th>Credit</th><th>Running balance</th><th>Journal</th></tr></thead>
        <tbody>{items.map((x,n)=><tr key={x.journal_id+"-"+n}>
          <td>{x.date}</td>
          <td>{x.description||x.journal_description}</td>
          <td>{Number(x.debit).toLocaleString()}</td>
          <td>{Number(x.credit).toLocaleString()}</td>
          <td><strong>{Number(x.running_balance).toLocaleString()} {x.currency}</strong></td>
          <td><code>{x.journal_id.slice(0,8)}…</code></td>
        </tr>)}</tbody>
      </table> : <div className="empty">No posted ledger activity in this date range.</div>}
    </div>
  </>;
}
