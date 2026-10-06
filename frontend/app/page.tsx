"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import { fiscalYearLabel } from "@/lib/fiscal";
import { canOperateLedger } from "@/lib/permissions";
import { withEntity } from "@/lib/entitySelection";

type DashboardData={
  cash_balances:{id:string;name:string;currency:string;balance:string}[];
  income:string;
  expenses:string;
  net_profit:string;
};

type CombinedDashboardData={
  reporting_currency:string;
  income:string;
  expenses:string;
  net_profit:string;
  entities:{
    entity_id:string;
    entity_name:string;
    functional_currency:string;
    income:string;
    expenses:string;
    net_profit:string;
    reporting_rate:string;
  }[];
};

type Currency={code:string;name:string;symbol:string;decimal_places:number;active:boolean};

export default function Dashboard() {
  const { entity, platformOwner, loading: entitiesLoading } = useEntity();
  const [data,setData]=useState<DashboardData>({cash_balances:[],income:"0",expenses:"0",net_profit:"0"});
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(true);
  const [combined,setCombined]=useState<CombinedDashboardData|null>(null);
  const [combinedLoading,setCombinedLoading]=useState(true);
  const [combinedError,setCombinedError]=useState("");
  const [currencies,setCurrencies]=useState<Currency[]>([]);
  const [reportingCurrency,setReportingCurrency]=useState("MMK");
  useEffect(()=>{
    if(entitiesLoading)return;
    if(!entity){setLoading(false);return;}
    let cancelled=false;
    setLoading(true);
    setError("");
    api<DashboardData>(`/entities/${entity.PublicID}/dashboard`)
      .then(result=>{if(!cancelled)setData(result)})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoading(false)});
    return ()=>{cancelled=true};
  },[entitiesLoading,entity?.PublicID]);
  useEffect(()=>{
    let cancelled=false;
    api<{items:Currency[]}>("/currencies?active=1")
      .then(result=>{
        if(cancelled)return;
        const items=result.items??[];
        setCurrencies(items);
        setReportingCurrency(current=>{
          if(items.some(x=>x.code===current))return current;
          return items.find(x=>x.code==="MMK")?.code??items[0]?.code??current;
        });
      })
      .catch(e=>{if(!cancelled)setCombinedError(e instanceof Error?e.message:String(e))});
    return()=>{cancelled=true};
  },[]);

  useEffect(()=>{
    if(!reportingCurrency)return;
    let cancelled=false;
    setCombinedLoading(true);
    setCombinedError("");
    setCombined(null);
    api<CombinedDashboardData>(`/dashboard/combined?currency=${encodeURIComponent(reportingCurrency)}`)
      .then(result=>{if(!cancelled)setCombined(result)})
      .catch(e=>{if(!cancelled)setCombinedError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setCombinedLoading(false)});
    return()=>{cancelled=true};
  },[reportingCurrency,entity?.PublicID]);

  return (
    <>
      <div className="page-head">
        <div><h1>Dashboard</h1><p>{entity?`Current-month overview for ${entity.Name}.`:"No entity is selected yet."}</p></div>
        {entity&&canOperateLedger(entity.Role)&&<Link className="button" href={withEntity("/transactions/new", entity.PublicID)}>New entry</Link>}
      </div>
      {error&&<div className="alert error" role="alert">{error}</div>}
      {!entitiesLoading&&!loading&&!entity&&<div className="alert" role="status">{platformOwner?"Create the first entity before recording any accounting.":"This account does not have an entity yet."}</div>}
      <div className="grid cards" aria-busy={loading}>
        <div className="card"><div className="muted">Income</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.income).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Expenses</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.expenses).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Net profit</div><div className="metric">{loading?<span className="skeleton skeleton-metric" aria-hidden="true"/>:<>{Number(data.net_profit).toLocaleString()} {entity?.FunctionalCurrency}</>}</div></div>
        <div className="card"><div className="muted">Fiscal year</div><div className="metric">{entity ? fiscalYearLabel(entity.FiscalMonth, entity.FiscalDay) || "—" : "—"}</div></div>
      </div>
      <div className="page-head dashboard-combined-head" style={{marginTop:28}}>
        <div><h1 style={{fontSize:20}}>All Entities{combined?` · ${combined.reporting_currency}`:""}</h1><p>Translated using each entity&apos;s latest stored reporting rate through the dashboard period.</p></div>
        {currencies.length>0&&<div className="field dashboard-reporting-currency">
          <label htmlFor="dashboard-reporting-currency">Reporting currency</label>
          <select id="dashboard-reporting-currency" value={reportingCurrency} disabled={combinedLoading} onChange={e=>setReportingCurrency(e.target.value)}>
            {currencies.map(x=><option key={x.code} value={x.code}>{x.code} · {x.name}</option>)}
          </select>
        </div>}
      </div>
      {combinedError&&<div className="alert error" role="alert">Combined dashboard: {combinedError}. Add the required stored exchange rate for each entity or choose another reporting currency.</div>}
      {combinedLoading?<div className="grid cards" role="status" aria-live="polite">
        <span className="sr-only">Loading combined dashboard…</span>
        {Array.from({length:3}).map((_,i)=><div className="card" key={i} aria-hidden="true"><div className="skeleton skeleton-line"/><div className="metric"><span className="skeleton skeleton-metric"/></div></div>)}
      </div>:combined&&<>
        <div className="grid cards dashboard-combined-kpis">
          <div className="card"><div className="muted">Combined income</div><div className="metric">{Number(combined.income).toLocaleString()} {combined.reporting_currency}</div></div>
          <div className="card"><div className="muted">Combined expenses</div><div className="metric">{Number(combined.expenses).toLocaleString()} {combined.reporting_currency}</div></div>
          <div className="card"><div className="muted">Combined net profit</div><div className="metric">{Number(combined.net_profit).toLocaleString()} {combined.reporting_currency}</div></div>
        </div>
        <div className="table-wrap" style={{marginTop:16}}><table><thead><tr><th>Entity</th><th>Functional currency</th><th>Income</th><th>Expenses</th><th>Net profit</th><th>Rate to {combined.reporting_currency}</th></tr></thead><tbody>{combined.entities.map(x=><tr key={x.entity_id}><td>{x.entity_name}</td><td>{x.functional_currency}</td><td>{Number(x.income).toLocaleString()}</td><td>{Number(x.expenses).toLocaleString()}</td><td>{Number(x.net_profit).toLocaleString()}</td><td>{Number(x.reporting_rate).toLocaleString(undefined,{maximumFractionDigits:6})}</td></tr>)}</tbody></table></div>
      </>}
      <FinancialAccounts/>
    </>
  );
}

const kindLabels:Record<string,string>={CASH:"Cash",BANK:"Bank",MOBILE_WALLET:"Mobile Wallet",CREDIT_CARD:"Credit Card",OTHER:"Other"};

function FinancialAccounts(){
  const [items,setItems]=useState<{entity_id:string;entity_name:string;id:string;name:string;kind:string;currency:string;active:boolean;ledger_account_id:string;balance:string}[]>([]);
  const [error,setError]=useState("");
  const [loading,setLoading]=useState(true);
  useEffect(()=>{
    let cancelled=false;
    api<{items:typeof items}>("/financial-account-balances")
      .then(result=>{if(!cancelled)setItems(result.items??[])})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))})
      .finally(()=>{if(!cancelled)setLoading(false)});
    return ()=>{cancelled=true};
  },[]);
  const groups=items.reduce<Record<string,{name:string;rows:typeof items}>>((acc,item)=>{
    acc[item.entity_id]??={name:item.entity_name,rows:[]};
    acc[item.entity_id].rows.push(item);
    return acc;
  },{});
  return <>
    <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Financial Accounts</h1><p>Cash, bank, wallet, and card balances for every entity you can access. Each balance stays in that account&apos;s currency.</p></div></div>
    {error&&<div className="alert error" role="alert">{error}</div>}
    <div className="table-wrap" aria-busy={loading}>
      <table>
        <thead><tr><th>Entity</th><th>Account</th><th>Type</th><th>Currency</th><th>Balance</th><th>Status</th></tr></thead>
        <tbody>
          {loading?Array.from({length:3}).map((_,i)=><tr key={`loading-${i}`} aria-hidden="true"><td><span className="skeleton skeleton-line"/></td><td><span className="skeleton skeleton-line"/></td><td><span className="skeleton skeleton-short"/></td><td><span className="skeleton skeleton-short"/></td><td><span className="skeleton skeleton-line"/></td><td><span className="skeleton skeleton-short"/></td></tr>):Object.entries(groups).flatMap(([,group])=>group.rows.map(row=><tr key={row.id}><td>{row.entity_name}</td><td><Link className="table-link" href={withEntity(`/accounts/${row.ledger_account_id}/ledger`, row.entity_id)}>{row.name}</Link></td><td>{kindLabels[row.kind]??row.kind}</td><td>{row.currency}</td><td>{Number(row.balance).toLocaleString(undefined,{maximumFractionDigits:2})} {row.currency}</td><td>{row.active?"Active":"Inactive"}</td></tr>))}
        </tbody>
      </table>
      {!loading&&items.length===0&&<div className="empty">No financial accounts are available.</div>}
    </div>
  </>;
}
