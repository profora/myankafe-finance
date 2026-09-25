"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Event={
  id:string;
  occurred_at:string;
  action:string;
  resource_type?:string|null;
  resource_id?:string|null;
  outcome:string;
  source:string;
  reason?:string|null;
  before?:unknown;
  after?:unknown;
  metadata?:unknown;
  ip_address?:string|null;
  user_agent?:string|null;
  request_id?:string|null;
  actor?:{id?:string|null;display_name?:string|null};
};
type Result={items:Event[];count:number;has_more:boolean};

const pageSize=100;

function pretty(value:unknown){
  if(value==null)return "";
  try{return JSON.stringify(value,null,2)}catch{return String(value)}
}

export default function Audit(){
  const {entity}=useEntity();
  const [items,setItems]=useState<Event[]>([]);
  const [count,setCount]=useState(0);
  const [hasMore,setHasMore]=useState(false);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(false);
  const [search,setSearch]=useState("");
  const [debounced,setDebounced]=useState("");
  const [action,setAction]=useState("");
  const [outcome,setOutcome]=useState("");
  const [from,setFrom]=useState("");
  const [to,setTo]=useState("");

  useEffect(()=>{
    const timer=setTimeout(()=>setDebounced(search.trim()),300);
    return()=>clearTimeout(timer);
  },[search]);

  const query=useMemo(()=>{
    const p=new URLSearchParams();
    if(debounced)p.set("q",debounced);
    if(action.trim())p.set("action",action.trim());
    if(outcome)p.set("outcome",outcome);
    if(from)p.set("from",from);
    if(to)p.set("to",to);
    p.set("limit",String(pageSize));
    return p;
  },[debounced,action,outcome,from,to]);

  async function load(reset=true){
    if(!entity)return;
    setLoading(true);setError("");
    const offset=reset?0:items.length;
    const p=new URLSearchParams(query);
    p.set("offset",String(offset));
    try{
      const result=await api<Result>(`/entities/${entity.PublicID}/audit-events?${p.toString()}`);
      setItems(current=>reset?result.items:[...current,...result.items]);
      setCount(result.count);
      setHasMore(result.has_more);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setLoading(false)}
  }

  useEffect(()=>{load(true)},[entity?.PublicID,query.toString()]);

  function clear(){
    setSearch("");setAction("");setOutcome("");setFrom("");setTo("");
  }

  return <>
    <div className="page-head">
      <div><h1>Audit Log</h1><p>Append-only business, security and request activity. Viewing this page is itself audited.</p></div>
    </div>
    {error&&<div className="alert error">{error}</div>}

    <div className="card transaction-filters" style={{marginBottom:16}}>
      <div className="transaction-filter-grid audit-filter-grid">
        <div className="field transaction-search"><label>Search</label><input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Action, resource ULID, actor, request ID or reason"/></div>
        <div className="field"><label>Action contains</label><input value={action} onChange={e=>setAction(e.target.value)} placeholder="e.g. TRANSACTION"/></div>
        <div className="field"><label>Outcome</label><select value={outcome} onChange={e=>setOutcome(e.target.value)}><option value="">All</option><option>SUCCESS</option><option>FAILED</option><option>DENIED</option></select></div>
        <div className="field"><label>From</label><input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></div>
        <div className="field"><label>To</label><input type="date" value={to} onChange={e=>setTo(e.target.value)}/></div>
        <div className="actions transaction-filter-actions"><button className="secondary" onClick={clear}>Clear filters</button>{loading&&<span className="muted">Refreshing…</span>}</div>
      </div>
    </div>

    <div className="list-summary"><span className="muted">{count.toLocaleString()} matching audit event{count===1?"":"s"}</span></div>

    <div className="table-wrap">
      {items.length?<table className="audit-table">
        <thead><tr><th>Time</th><th>Action</th><th>Outcome</th><th>Resource</th><th>Actor</th><th>Source</th><th>Details</th></tr></thead>
        <tbody>{items.map(x=><tr key={x.id}>
          <td>{new Date(x.occurred_at).toLocaleString()}</td>
          <td><strong>{x.action}</strong>{x.reason&&<div className="muted">{x.reason}</div>}</td>
          <td><span className={`badge ${x.outcome==="SUCCESS"?"POSTED":x.outcome==="DENIED"?"VOIDED":""}`}>{x.outcome}</span></td>
          <td>{x.resource_type??"—"}{x.resource_id&&<div><code>{x.resource_id}</code></div>}</td>
          <td>{x.actor?.display_name??"System"}{x.ip_address&&<div className="muted">{x.ip_address}</div>}</td>
          <td>{x.source}</td>
          <td>
            <details className="audit-details">
              <summary>Inspect</summary>
              <div className="audit-detail-panel">
                {x.request_id&&<div><strong>Request:</strong> <code>{x.request_id}</code></div>}
                {x.user_agent&&<div><strong>User agent:</strong> <span className="muted">{x.user_agent}</span></div>}
                {x.before!=null&&<><strong>Before</strong><pre>{pretty(x.before)}</pre></>}
                {x.after!=null&&<><strong>After</strong><pre>{pretty(x.after)}</pre></>}
                {x.metadata!=null&&<><strong>Metadata</strong><pre>{pretty(x.metadata)}</pre></>}
                {x.before==null&&x.after==null&&x.metadata==null&&!x.request_id&&<span className="muted">No additional event payload.</span>}
              </div>
            </details>
          </td>
        </tr>)}</tbody>
      </table>:<div className="empty">{loading?"Loading audit events…":"No audit events match these filters."}</div>}
    </div>

    <div className="list-pagination">
      <span className="muted">Showing {items.length.toLocaleString()} of {count.toLocaleString()}</span>
      {hasMore&&<button className="secondary" disabled={loading} onClick={()=>load(false)}>{loading?"Loading…":"Load more"}</button>}
    </div>
  </>;
}
