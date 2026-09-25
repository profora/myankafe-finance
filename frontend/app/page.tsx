"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type DashboardData={
  cash_balances:{id:string;name:string;currency:string;balance:string}[];
  income:string;
  expenses:string;
  net_profit:string;
};

export default function Dashboard() {
  const { entity } = useEntity();
  const [data,setData]=useState<DashboardData>({cash_balances:[],income:"0",expenses:"0",net_profit:"0"});
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(true);
  const [combined,setCombined]=useState<{reporting_currency:string;income:string;expenses:string;net_profit:string;entities:{entity_id:string;entity_name:string;functional_currency:string;income:string;expenses:string;net_profit:string;reporting_rate:string}[]}|null>(null);
  useEffect(()=>{
    if(!entity)return;
    let cancelled=false;
    setLoading(true);
    setError("");
    api<DashboardData>(`/entities/${entity.PublicID}/dashboard`)
      .then(result=>{if(!cancelled)setData(result)})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoading(false)});
    return ()=>{cancelled=true};
  },[entity?.PublicID]);
  useEffect(()=>{api<any>("/dashboard/combined?currency=MMK").then(setCombined).catch(()=>{})},[entity]);

  return (
    <>
      <div className="page-head">
        <div><h1>Dashboard</h1><p>Current-month overview for {entity?.Name ?? "the selected entity"}.</p></div>
        <Link className="button" href="/transactions/new">New entry</Link>
      </div>
      {error&&<div className="alert error" role="alert">{error}</div>}
      <div className="grid cards" aria-busy={loading}>
        <div className="card"><div className="muted">Income</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.income).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Expenses</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.expenses).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Net profit</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.net_profit).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Fiscal year starts</div><div className="metric">{entity ? `${entity.FiscalMonth}/${entity.FiscalDay}` : "—"}</div></div>
      </div>
      {combined&&<><div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>All Entities · {combined.reporting_currency}</h1><p>Translated using each entity's latest stored reporting rate.</p></div></div>
      <div className="grid cards">
        <div className="card"><div className="muted">Combined income</div><div className="metric">{Number(combined.income).toLocaleString()} {combined.reporting_currency}</div></div>
        <div className="card"><div className="muted">Combined expenses</div><div className="metric">{Number(combined.expenses).toLocaleString()} {combined.reporting_currency}</div></div>
        <div className="card"><div className="muted">Combined net profit</div><div className="metric">{Number(combined.net_profit).toLocaleString()} {combined.reporting_currency}</div></div>
      </div>
      <div className="table-wrap" style={{marginTop:16}}><table><thead><tr><th>Entity</th><th>Functional currency</th><th>Income</th><th>Expenses</th><th>Net profit</th><th>Rate to {combined.reporting_currency}</th></tr></thead><tbody>{combined.entities.map(x=><tr key={x.entity_id}><td>{x.entity_name}</td><td>{x.functional_currency}</td><td>{Number(x.income).toLocaleString()}</td><td>{Number(x.expenses).toLocaleString()}</td><td>{Number(x.net_profit).toLocaleString()}</td><td>{x.reporting_rate}</td></tr>)}</tbody></table></div></>}
      <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Cash / bank balances</h1></div></div>
      <div className="table-wrap" aria-busy={loading}><table><thead><tr><th>Account</th><th>Currency</th><th>Balance</th></tr></thead><tbody>{loading?Array.from({length:3}).map((_,i)=><tr key={`loading-${i}`} aria-hidden="true"><td><span className="skeleton skeleton-line"/></td><td><span className="skeleton skeleton-short"/></td><td><span className="skeleton skeleton-line"/></td></tr>):data.cash_balances.map(x=><tr key={x.id}><td>{x.name}</td><td>{x.currency}</td><td>{Number(x.balance).toLocaleString()}</td></tr>)}</tbody></table>{loading&&<span className="sr-only" role="status">Loading dashboard balances…</span>}</div>
    </>
  );
}
