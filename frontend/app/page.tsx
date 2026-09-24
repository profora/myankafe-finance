"use client";

import Link from "next/link";
import { useEntity } from "@/components/EntityContext";

export default function Dashboard() {
  const { entity } = useEntity();
  return (
    <>
      <div className="page-head">
        <div>
          <h1>Dashboard</h1>
          <p>Finance overview for {entity?.Name ?? "the selected entity"}.</p>
        </div>
        <Link className="button" href="/transactions/new">New entry</Link>
      </div>
      <div className="grid cards">
        <div className="card"><div className="muted">Functional currency</div><div className="metric">{entity?.FunctionalCurrency ?? "—"}</div></div>
        <div className="card"><div className="muted">Fiscal year starts</div><div className="metric">{entity ? `${entity.FiscalMonth}/${entity.FiscalDay}` : "—"}</div></div>
        <div className="card"><div className="muted">Ledger mode</div><div className="metric">Double-entry</div></div>
        <div className="card"><div className="muted">Posting controls</div><div className="metric">Enabled</div></div>
      </div>
      <div className="card" style={{marginTop:16}}>
        <h3>V1 foundation</h3>
        <p className="muted">Create the Chart of Accounts and cash/bank accounts, then enter income or expenses. Posting creates balanced journal lines and respects the entity lock date.</p>
      </div>
    </>
  );
}
