"use client";

import Link from "next/link";
import { useEffect, useId, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { dateInTimeZone } from "@/lib/date";
import { useEntity } from "@/components/EntityContext";
import ConfirmDialog from "@/components/ConfirmDialog";
import TransactionAttachments from "@/components/TransactionAttachments";
import DraftTransactionActions from "@/components/DraftTransactionActions";
import { canCorrectPostedAccounting, canOperateLedger } from "@/lib/permissions";

type JournalLine={
  line_no:number;
  account:{id:string;code:string;name:string};
  financial_account?:{id?:string|null;name?:string|null};
  description:string;
  transaction_currency:string;
  transaction_debit:string;
  transaction_credit:string;
  functional_currency:string;
  fx_rate:string;
  debit:string;
  credit:string;
  exchange_rate_id?:string|null;
};
type Journal={
  id:string;
  date:string;
  description:string;
  status:string;
  functional_currency:string;
  posted_at?:string|null;
  lines:JournalLine[];
};
type Detail={
  id:string;
  type:string;
  status:string;
  date:string;
  description:string;
  currency:string;
  total:string;
  contact?:{id?:string|null;name?:string|null};
  financial_account?:{id?:string|null;name?:string|null};
  original_transaction_id?:string|null;
  reversal_transaction_id?:string|null;
  posted_at?:string|null;
  voided_at?:string|null;
  void_reason?:string|null;
  splits:{line_no:number;account_id:string;account_code:string;account_name:string;amount:string;description:string}[];
  journals:Journal[];
};

export default function TransactionDetailPage(){
  const {entity,entities,setEntityID,loading:entitiesLoading}=useEntity();
  const mayOperate=canOperateLedger(entity?.Role);
  const mayReverse=canCorrectPostedAccounting(entity?.Role);
  const params=useParams<{id:string}>();
  const id=Array.isArray(params.id)?params.id[0]:params.id;
  const [detail,setDetail]=useState<Detail|null>(null);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(false);
  const [requestedEntity,setRequestedEntity]=useState<string|undefined>();
  const [reverseOpen,setReverseOpen]=useState(false);
  const [reverseBusy,setReverseBusy]=useState(false);
  const [reverseReason,setReverseReason]=useState("");
  const [reverseDate,setReverseDate]=useState("");
  const reversalDateID=useId();
  const reversalReasonID=useId();

  async function load(){
    if(!entity||!id)return;
    setLoading(true);setError("");
    try{
      setDetail(await api<Detail>(`/entities/${entity.PublicID}/transactions/${id}`));
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{
    setRequestedEntity(new URLSearchParams(window.location.search).get("entity")??"");
  },[]);

  useEffect(()=>{
    if(requestedEntity===undefined||entitiesLoading)return;
    if(requestedEntity && !entities.some(item=>item.PublicID===requestedEntity)){
      setError("That entity is not available to this account.");
      return;
    }
    if(requestedEntity && entity?.PublicID!==requestedEntity) setEntityID(requestedEntity);
  },[requestedEntity,entities,entitiesLoading,entity?.PublicID,setEntityID]);

  useEffect(()=>{
    if(requestedEntity===undefined)return;
    if(requestedEntity && entity?.PublicID!==requestedEntity)return;
    setDetail(null);
    void load();
  },[entity?.PublicID,id,requestedEntity]);

  const canReverse=Boolean(mayReverse&&detail&&detail.status==="POSTED"&&!detail.reversal_transaction_id);

  function openReverse(){
    setReverseReason("");
    setReverseDate(dateInTimeZone(entity?.Timezone??"Asia/Yangon"));
    setReverseOpen(true);
  }

  async function reverse(){
    if(!entity||!detail||!reverseDate||!reverseReason.trim())return;
    setReverseBusy(true);setError("");
    try{
      await api(`/entities/${entity.PublicID}/transactions/${detail.id}/reverse`,{method:"POST",body:JSON.stringify({reversal_date:reverseDate,reason:reverseReason.trim()})});
      setReverseOpen(false);setReverseReason("");
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setReverseBusy(false)}
  }

  if(loading&&!detail)return <div className="card detail-loading" role="status" aria-live="polite"><span className="skeleton skeleton-wide" aria-hidden="true"/><span className="skeleton skeleton-line" aria-hidden="true"/><span className="sr-only">Loading transaction…</span></div>;

  return <>
    <div className="page-head detail-page-head">
      <div>
        <div className="muted"><Link href="/transactions">Transactions</Link> / Detail</div>
        <h1>{detail?.description??"Transaction"}</h1>
        {detail&&<p>{detail.date} · {detail.type} · <span className={`badge ${detail.status}`}>{detail.status}</span></p>}
      </div>
      {canReverse&&<button type="button" className="danger" onClick={openReverse}>Reverse transaction</button>}
    </div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    {detail&&<>
      {mayOperate&&detail.status==="DRAFT"&&<div className="card draft-management-card">
        <div>
          <strong>Draft transaction</strong>
          <p className="muted">This transaction has no accounting effect until posted. You can edit or cancel it while its date remains in an open period.</p>
        </div>
        <DraftTransactionActions entity={entity!} draft={detail} onChanged={load}/>
      </div>}

      <div className="grid cards">
        <div className="card"><div className="muted">Amount</div><div className="metric">{Number(detail.total).toLocaleString()} {detail.currency}</div></div>
        <div className="card"><div className="muted">Financial account</div><div style={{fontWeight:750,marginTop:8}}>{detail.financial_account?.name??"—"}</div></div>
        <div className="card"><div className="muted">Contact</div><div style={{fontWeight:750,marginTop:8}}>{detail.contact?.name??"—"}</div></div>
        <div className="card"><div className="muted">Public ID</div><div style={{fontFamily:"monospace",fontSize:12,marginTop:8}}>{detail.id}</div></div>
      </div>

      <TransactionAttachments entityID={entity!.PublicID} transactionID={detail.id} canUpload={mayOperate} canManage={mayOperate}/>

      {(detail.original_transaction_id||detail.reversal_transaction_id||detail.void_reason)&&<div className="card" style={{marginTop:16}}>
        <h3>Correction history</h3>
        {detail.original_transaction_id&&<p>Reverses: <Link className="table-link" href={`/transactions/${detail.original_transaction_id}`}>{detail.original_transaction_id}</Link></p>}
        {detail.reversal_transaction_id&&<p>Reversal: <Link className="table-link" href={`/transactions/${detail.reversal_transaction_id}`}>{detail.reversal_transaction_id}</Link></p>}
        {detail.void_reason&&<p><strong>Reason:</strong> {detail.void_reason}</p>}
      </div>}

      {detail.splits.length>0&&<>
        <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Entry splits</h1></div></div>
        <div className="table-wrap"><table><thead><tr><th>#</th><th>Account</th><th>Description</th><th>Amount</th></tr></thead><tbody>{detail.splits.map(x=><tr key={x.line_no}><td>{x.line_no}</td><td><Link className="table-link" href={`/accounts/${x.account_id}/ledger`}>{x.account_code} · {x.account_name}</Link></td><td>{x.description||"—"}</td><td>{Number(x.amount).toLocaleString()} {detail.currency}</td></tr>)}</tbody></table></div>
      </>}

      <ConfirmDialog
        open={reverseOpen}
        title="Reverse posted transaction?"
        description="This does not edit or delete the original. A new opposite journal will be posted and the original remains in the audit trail."
        confirmLabel="Post reversal"
        danger
        busy={reverseBusy}
        confirmDisabled={!reverseDate||!reverseReason.trim()}
        onCancel={()=>{if(!reverseBusy){setReverseOpen(false);setReverseReason("")}}}
        onConfirm={reverse}
      >
        <div className="form" style={{marginTop:16}}>
          <div className="field"><label htmlFor={reversalDateID}>Reversal date</label><input id={reversalDateID} type="date" value={reverseDate} onChange={e=>setReverseDate(e.target.value)}/></div>
          <div className="field"><label htmlFor={reversalReasonID}>Reason</label><textarea id={reversalReasonID} autoFocus rows={3} value={reverseReason} onChange={e=>setReverseReason(e.target.value)} placeholder="Why is this transaction being reversed?"/></div>
          {!reverseReason.trim()&&<div className="muted">A reason is required for the audit trail.</div>}
        </div>
      </ConfirmDialog>

      {detail.journals.map(j=><section key={j.id}>
        <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Journal · {j.status}</h1><p>{j.date} · {j.id}</p></div></div>
        <div className="table-wrap"><table><thead><tr><th>#</th><th>Account</th><th>Description</th><th>Transaction debit</th><th>Transaction credit</th><th>FX</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{j.lines.map(x=><tr key={x.line_no}>
          <td>{x.line_no}</td>
          <td><Link className="table-link" href={`/accounts/${x.account.id}/ledger`}>{x.account.code} · {x.account.name}</Link>{x.financial_account?.name&&<div className="muted">{x.financial_account.name}</div>}</td>
          <td>{x.description||"—"}</td>
          <td>{Number(x.transaction_debit).toLocaleString()} {x.transaction_currency}</td>
          <td>{Number(x.transaction_credit).toLocaleString()} {x.transaction_currency}</td>
          <td>{x.fx_rate}</td>
          <td>{Number(x.debit).toLocaleString()} {x.functional_currency}</td>
          <td>{Number(x.credit).toLocaleString()} {x.functional_currency}</td>
        </tr>)}</tbody></table></div>
      </section>)}
    </>}
  </>;
}
