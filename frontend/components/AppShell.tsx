"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { clearDevUser, logout } from "@/lib/api";
import { EntityProvider, useEntity } from "./EntityContext";

const nav = [
  ["/", "Dashboard"],
  ["/transactions", "Transactions"],
  ["/transactions/new", "New Entry"],
  ["/transfers", "Transfers"],
  ["/inter-entity", "Inter-Entity"],
  ["/contacts", "Contacts"],
  ["/accounts", "Chart of Accounts"],
  ["/financial-accounts", "Cash / Bank"],
  ["/exchange-rates", "Exchange Rates"],
  ["/manual-journal", "Manual Journal"],
  ["/reports", "Reports"],
  ["/audit", "Audit Log"],
  ["/locking", "Transaction Locking"],
  ["/settings/entities", "Entities"],
  ["/settings/users", "Users & Access"],
];

function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { entities, entity, setEntityID, error } = useEntity();

  async function signOut(){
    try { await logout(); } catch {}
    clearDevUser();
    window.location.assign("/login");
  }

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
          <div className="actions">
            <div className="top-meta">{entity ? `${entity.FunctionalCurrency} · FY ${entity.FiscalMonth}/${entity.FiscalDay}` : "No entity"}</div>
            <button className="secondary" onClick={signOut}>Sign out</button>
          </div>
        </header>
        {error && <div className="alert error">{error}</div>}
        <section className="content">{children}</section>
      </main>
    </div>
  );
}

export default function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  if (pathname === "/login" || pathname === "/dev-login") {
    return <main className="auth-shell">{children}</main>;
  }
  return <EntityProvider><Shell>{children}</Shell></EntityProvider>;
}
