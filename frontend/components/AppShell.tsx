"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api } from "@/lib/api";
import { EntityProvider, useEntity } from "./EntityContext";

const nav = [
  ["/", "Dashboard"],
  ["/transactions", "Transactions"],
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
  ["/settings/security", "Security"],
];

type Me={user:{public_id:string;username:string;display_name:string}};

function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { entities, entity, setEntityID, error } = useEntity();
  const [me,setMe]=useState<Me["user"]|null>(null);

  useEffect(()=>{api<Me>("/auth/me").then(x=>setMe(x.user)).catch(()=>{})},[]);

  async function signOut(){
    try { await api("/auth/logout",{method:"POST",body:"{}"}); } catch {}
    window.location.assign("/login");
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <Link className="brand brand-lockup" href="/" aria-label="MyanKafe Finance home">
          <img className="brand-mark" src="/brand/logo-head.svg" alt=""/>
          <span className="brand-wordmark-wrap">
            <img className="brand-wordmark" src="/brand/logo-text.svg" alt="MyanKafe"/>
            <small>Finance</small>
          </span>
        </Link>
        <nav>
          {nav.map(([href, label]) => (
            <Link key={href} className={pathname === href || (href!=="/"&&pathname.startsWith(href+"/")) ? "active" : ""} href={href}>{label}</Link>
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
            <div className="top-meta">
              {me&&<strong>{me.display_name}</strong>}
              {me&&entity&&" · "}
              {entity ? `${entity.FunctionalCurrency} · FY ${entity.FiscalMonth}/${entity.FiscalDay}` : "No entity"}
            </div>
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
