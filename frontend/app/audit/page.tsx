"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Event={id:string;occurred_at:string;action:string;resource_type?:string;resource_id?:string;outcome:string;source:string;actor?:{display_name?:string}};

export default function Audit(){
  const {entity}=useEntity();
  const [items,setItems]=useState<Event[]>([]);
  const [error,setError]=useState("");
  useEffect(()=>{if(entity)api<{items:Event[]}>(`/entities/${entity.PublicID}/audit-events?limit=200`).then(x=>setItems(x.items)).catch(e=>setError(e.message))},[entity]);
  return <>
    <div className="page-head"><div><h1>Audit Log</h1><p>Append-only business and request activity. Viewing this page is itself audited.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    <div className="table-wrap"><table><thead><tr><th>Time</th><th>Action</th><th>Outcome</th><th>Resource</th><th>Actor</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{new Date(x.occurred_at).toLocaleString()}</td><td>{x.action}</td><td>{x.outcome}</td><td>{x.resource_type??"—"} {x.resource_id??""}</td><td>{x.actor?.display_name??"System"}</td></tr>)}</tbody></table></div>
  </>;
}
