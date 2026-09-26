"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api } from "@/lib/api";
import { EntityProvider, useEntity } from "./EntityContext";
import { canConfigureAccounting, canCorrectPostedAccounting, canLockAccounting, canManageEntitySettings, canOperateLedger, canViewAudit } from "@/lib/permissions";

const nav=[
  {href:"/",label:"Dashboard",group:"Overview"},
  {href:"/transactions",label:"Transactions",group:"Transactions"},
  {href:"/transfers",label:"Transfers",group:"Transactions",show:canOperateLedger},
  {href:"/inter-entity",label:"Inter-Entity",group:"Transactions",show:canOperateLedger},
  {href:"/manual-journal",label:"Manual Journal",group:"Transactions",show:canCorrectPostedAccounting},
  {href:"/accounts",label:"Chart of Accounts",group:"Accounting Setup"},
  {href:"/financial-accounts",label:"Financial Accounts",group:"Accounting Setup"},
  {href:"/exchange-rates",label:"Exchange Rates",group:"Accounting Setup"},
  {href:"/contacts",label:"Contacts",group:"Accounting Setup"},
  {href:"/reports",label:"Reports",group:"Reports & Control"},
  {href:"/audit",label:"Audit Log",group:"Reports & Control",show:canViewAudit},
  {href:"/locking",label:"Transaction Locking",group:"Reports & Control",show:canLockAccounting},
  {href:"/settings/entities",label:"Entities",group:"Settings",show:canManageEntitySettings},
  {href:"/settings/currencies",label:"Currencies",group:"Settings",ownerOnly:true},
  {href:"/settings/contact-types",label:"Contact Types",group:"Settings",show:canConfigureAccounting},
  {href:"/settings/users",label:"Users & Access",group:"Settings",ownerOnly:true},
  {href:"/settings/system",label:"System",group:"Settings",ownerOnly:true},
  {href:"/settings/security",label:"Security",group:"Settings"},
];

type Me={user:{public_id:string;username:string;display_name:string}};

function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { entities, entity, setEntityID, error, loading } = useEntity();
  const [me,setMe]=useState<Me["user"]|null>(null);
  const [mobileNavOpen,setMobileNavOpen]=useState(false);
  const ownerAnywhere=entities.some(x=>x.Role==="OWNER");

  useEffect(()=>{api<Me>("/auth/me").then(x=>setMe(x.user)).catch(()=>{})},[]);
  useEffect(()=>{setMobileNavOpen(false)},[pathname]);

  async function signOut(){
    try { await api("/auth/logout",{method:"POST",body:"{}"}); } catch {}
    window.location.assign("/login");
  }

  return (
    <div className="app">
      <a className="skip-link" href="#main-content">Skip to main content</a>
      <aside className={`sidebar ${mobileNavOpen?"mobile-open":""}`}>
        <Link className="brand platform-brand" href="/" aria-label="MyanKafe Finance home" onClick={()=>setMobileNavOpen(false)}>
          <img className="sidebar-brand-logo" src="/brand/logo-head.svg" alt="" width={52} height={46}/>
          <span className="sidebar-brand-text">
            <strong>MyanKafe</strong>
            <span>Finance</span>
          </span>
        </Link>
        <nav aria-label="Primary navigation">
          {["Overview","Transactions","Accounting Setup","Reports & Control","Settings"].map(group=>{
            const items=nav.filter(item=>{
              if(item.group!==group)return false;
              if(item.ownerOnly)return ownerAnywhere;
              return !item.show||item.show(entity?.Role);
            });
            if(items.length===0)return null;
            return <div className="nav-group" key={group}>
              <div className="nav-caption">{group}</div>
              {items.map(({href,label})=>{
                const active=pathname===href||(href!=="/"&&pathname.startsWith(href+"/"));
                return <Link key={href} aria-current={active?"page":undefined} className={active?"active":""} href={href} onClick={()=>setMobileNavOpen(false)}>{label}</Link>;
              })}
            </div>;
          })}
        </nav>
        <div className="sidebar-note">
          Double-entry ledger<br />UUIDv7 internal · ULID public
        </div>
      </aside>
      {mobileNavOpen&&<button type="button" className="mobile-nav-overlay" aria-label="Close navigation" onClick={()=>setMobileNavOpen(false)}/>}
      <main className="main">
        <header className="topbar">
          <div className="topbar-entity">
            <button type="button" className="secondary mobile-nav-button" aria-label="Open navigation" onClick={()=>setMobileNavOpen(true)}>☰</button>
            <div>
              <div className="eyebrow">Entity</div>
              <select aria-label="Current entity" aria-busy={loading} disabled={loading||entities.length===0} value={entity?.PublicID ?? ""} onChange={(e) => setEntityID(e.target.value)}>
                {loading&&entities.length===0&&<option value="">Loading entities…</option>}
                {!loading&&entities.length===0&&<option value="">No entities available</option>}
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
            <button type="button" className="secondary" onClick={signOut}>Sign out</button>
          </div>
        </header>
        {error && <div className="alert error" role="alert">{error}</div>}
        {!loading&&!error&&entities.length===0&&<div className="alert" role="status">No finance entities are available for this account.</div>}
        <section id="main-content" tabIndex={-1} className="content">{children}</section>
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
