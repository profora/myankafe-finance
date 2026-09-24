"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { EntityProvider, useEntity } from "./EntityContext";

const nav = [
  ["/", "Dashboard"],
  ["/transactions", "Transactions"],
  ["/transactions/new", "New Entry"],
  ["/accounts", "Chart of Accounts"],
  ["/financial-accounts", "Cash / Bank"],
  ["/locking", "Transaction Locking"],
];

function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { entities, entity, setEntityID, error } = useEntity();

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">MyanKafe <span>Finance</span></div>
        <nav>
          {nav.map(([href, label]) => (
            <Link key={href} className={pathname === href ? "active" : ""} href={href}>{label}</Link>
          ))}
        </nav>
        <div className="sidebar-note">
          Double-entry ledger<br />UUIDv7 internal · ULID public
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div>
            <div className="eyebrow">Entity</div>
            <select value={entity?.PublicID ?? ""} onChange={(e) => setEntityID(e.target.value)}>
              {entities.map((x) => <option key={x.PublicID} value={x.PublicID}>{x.Name}</option>)}
            </select>
          </div>
          <div className="top-meta">{entity ? `${entity.FunctionalCurrency} · FY ${entity.FiscalMonth}/${entity.FiscalDay}` : "No entity"}</div>
        </header>
        {error && <div className="alert error">{error}. Configure the development user first.</div>}
        <section className="content">{children}</section>
      </main>
    </div>
  );
}

export default function AppShell({ children }: { children: React.ReactNode }) {
  return <EntityProvider><Shell>{children}</Shell></EntityProvider>;
}
