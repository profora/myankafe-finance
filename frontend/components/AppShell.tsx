"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api } from "@/lib/api";
import { EntityProvider, useEntity } from "./EntityContext";
import { canCorrectPostedAccounting, canLockAccounting, canManageEntitySettings, canManagePlatformUsers, canOperateLedger, canViewAudit } from "@/lib/permissions";

const nav = [
  {href:"/",label:"Dashboard"},
  {href:"/transactions",label:"Transactions"},
  {href:"/transfers",label:"Transfers",show:canOperateLedger},
  {href:"/inter-entity",label:"Inter-Entity",show:canOperateLedger},
  {href:"/contacts",label:"Contacts"},
  {href:"/accounts",label:"Chart of Accounts"},
  {href:"/financial-accounts",label:"Cash / Bank"},
  {href:"/exchange-rates",label:"Exchange Rates"},
  {href:"/manual-journal",label:"Manual Journal",show:canCorrectPostedAccounting},
  {href:"/reports",label:"Reports"},
  {href:"/audit",label:"Audit Log",show:canViewAudit},
  {href:"/locking",label:"Transaction Locking",show:canLockAccounting},
  {href:"/settings/entities",label:"Entities",show:canManageEntitySettings},
  {href:"/settings/users",label:"Users & Access",show:canManagePlatformUsers},
  {href:"/settings/security",label:"Security"},
];

type Me={user:{public_id:string;username:string;display_name:string}};

function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { entities, entity, setEntityID, error } = useEntity();
  const [me,setMe]=useState<Me["user"]|null>(null);
  const [mobileNavOpen,setMobileNavOpen]=useState(false);

  useEffect(()=>{api<Me>("/auth/me").then(x=>setMe(x.user)).catch(()=>{})},[]);
  useEffect(()=>{setMobileNavOpen(false)},[pathname]);

  async function signOut(){
    try { await api("/auth/logout",{method:"POST",body:"{}"}); } catch {}
    window.location.assign("/login");
  }

  return (
    <div className="app">
      <aside className={`sidebar ${mobileNavOpen?"mobile-open":""}`}>
        <Link className="brand platform-brand" href="/" aria-label="MyanKafe Finance home" onClick={()=>setMobileNavOpen(false)}>
          <img className="sidebar-brand-logo" src="/brand/chieftain-logo.webp" alt="Chieftain Chin Coffee"/>
          <span className="sidebar-brand-text">
            <strong>MyanKafe</strong>
            <span>Finance</span>
          </span>
        </Link>
        <nav>
          {nav.filter(item=>!item.show||item.show(entity?.Role)).map(({href,label}) => (
            <Link key={href} className={pathname === href || (href!=="/"&&pathname.startsWith(href+"/")) ? "active" : ""} href={href} onClick={()=>setMobileNavOpen(false)}>{label}</Link>
          ))}
        </nav>
        <div className="sidebar-note">
          Double-entry ledger<br />UUIDv7 internal · ULID public
        </div>
      </aside>
      {mobileNavOpen&&<button className="mobile-nav-overlay" aria-label="Close navigation" onClick={()=>setMobileNavOpen(false)}/>}
      <main className="main">
        <header className="topbar">
          <div className="topbar-entity">
            <button className="secondary mobile-nav-button" aria-label="Open navigation" onClick={()=>setMobileNavOpen(true)}>☰</button>
            <div>
              <div className="eyebrow">Entity</div>
              <select value={entity?.PublicID ?? ""} onChange={(e) => setEntityID(e.target.value)}>
                {entities.map((x) => <option key={x.PublicID} value={x.PublicID}>{x.Name}</option>)}
              </select>
            </div>
          </div>
          <div className="actions">
            <div className="top-meta">
              {me&&<strong>{me.display_name}</strong>}
              {me&&entity&&" · "}
              {entity ? `${entity.Role} · ${entity.FunctionalCurrency} · FY ${entity.FiscalMonth}/${entity.FiscalDay}` : "No entity"}
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
