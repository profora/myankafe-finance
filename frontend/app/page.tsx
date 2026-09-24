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
  useEffect(()=>{if(entity)api<DashboardData>(`/entities/${entity.PublicID}/dashboard`).then(setData).catch(e=>setError(e.message))},[entity]);

  return (
    <>
      <div className="page-head">
        <div><h1>Dashboard</h1><p>Current-month overview for {entity?.Name ?? "the selected entity"}.</p></div>
        <Link className="button" href="/transactions/new">New entry</Link>
      </div>
      {error&&<div className="alert error">{error}</div>}
      <div className="grid cards">
        <div className="card"><div className="muted">Income</div><div className="metric">{Number(data.income).toLocaleString()} {entity?.FunctionalCurrency}</div></div>
        <div className="card"><div className="muted">Expenses</div><div className="metric">{Number(data.expenses).toLocaleString()} {entity?.FunctionalCurrency}</div></div>
        <div className="card"><div className="muted">Net profit</div><div className="metric">{Number(data.net_profit).toLocaleString()} {entity?.FunctionalCurrency}</div></div>
        <div className="card"><div className="muted">Fiscal year starts</div><div className="metric">{entity ? `${entity.FiscalMonth}/${entity.FiscalDay}` : "—"}</div></div>
      </div>
      <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Cash / bank balances</h1></div></div>
      <div className="table-wrap"><table><thead><tr><th>Account</th><th>Currency</th><th>Balance</th></tr></thead><tbody>{data.cash_balances.map(x=><tr key={x.id}><td>{x.name}</td><td>{x.currency}</td><td>{Number(x.balance).toLocaleString()}</td></tr>)}</tbody></table></div>
    </>
  );
}
