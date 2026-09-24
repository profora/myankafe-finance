"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";

type Row={id?:string;code?:string;name?:string;type?:string;amount?:string;debits?:string;credits?:string;balance?:string};
type GL={journal_id:string;date:string;account_code:string;account_name:string;description:string;debit:string;credit:string;currency:string};
type Cash={date:string;financial_account_id:string;name:string;currency:string;movement:string};
type Inter={counterparty_entity_id:string;counterparty_name:string;due_from:string;due_to:string};

export default function Reports(){
  const {entity}=useEntity();
  const [pl,setPL]=useState<Row[]>([]);
  const [tb,setTB]=useState<Row[]>([]);
  const [bs,setBS]=useState<Row[]>([]);
  const [earnings,setEarnings]=useState("0");
  const [gl,setGL]=useState<GL[]>([]);
  const [cash,setCash]=useState<Cash[]>([]);
  const [inter,setInter]=useState<Inter[]>([]);
  const [error,setError]=useState("");

  useEffect(()=>{
    if(!entity)return;
    Promise.all([
      api<{items:Row[]}>(`/entities/${entity.PublicID}/reports/profit-loss`),
      api<{items:Row[]}>(`/entities/${entity.PublicID}/reports/trial-balance`),
      api<{items:Row[];current_earnings:string}>(`/entities/${entity.PublicID}/reports/balance-sheet`),
      api<{items:GL[]}>(`/entities/${entity.PublicID}/reports/general-ledger?limit=200`),
      api<{items:Cash[]}>(`/entities/${entity.PublicID}/reports/cash-movement`),
      api<{items:Inter[]}>(`/entities/${entity.PublicID}/reports/inter-entity-balances`)
    ]).then(([p,t,b,g,c,i])=>{
      setPL(p.items);setTB(t.items);setBS(b.items);setEarnings(b.current_earnings);
      setGL(g.items);setCash(c.items);setInter(i.items);
    }).catch(e=>setError(e.message));
  },[entity]);

  return <>
    <div className="page-head"><div><h1>Reports</h1><p>Reports are generated from posted/reversed journal history only.</p></div></div>
    {error&&<div className="alert error">{error}</div>}

    <div className="grid" style={{gridTemplateColumns:"repeat(auto-fit,minmax(420px,1fr))"}}>
      <div><h3>Profit & Loss</h3><div className="table-wrap"><table><thead><tr><th>Account</th><th>Type</th><th>Amount</th></tr></thead><tbody>{pl.map((x,n)=><tr key={x.id??n}><td>{x.code} · {x.name}</td><td>{x.type}</td><td>{Number(x.amount||0).toLocaleString()}</td></tr>)}</tbody></table></div></div>
      <div><h3>Balance Sheet</h3><div className="table-wrap"><table><thead><tr><th>Account</th><th>Type</th><th>Balance</th></tr></thead><tbody>{bs.map((x,n)=><tr key={x.id??n}><td>{x.code} · {x.name}</td><td>{x.type}</td><td>{Number(x.balance||0).toLocaleString()}</td></tr>)}<tr><td><strong>Current earnings</strong></td><td>EQUITY</td><td><strong>{Number(earnings).toLocaleString()}</strong></td></tr></tbody></table></div></div>
      <div><h3>Trial Balance</h3><div className="table-wrap"><table><thead><tr><th>Account</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{tb.map((x,n)=><tr key={x.id??n}><td>{x.code} · {x.name}</td><td>{Number(x.debits||0).toLocaleString()}</td><td>{Number(x.credits||0).toLocaleString()}</td></tr>)}</tbody></table></div></div>
      <div><h3>Inter-Entity Balances</h3><div className="table-wrap"><table><thead><tr><th>Counterparty</th><th>Due from</th><th>Due to</th></tr></thead><tbody>{inter.map(x=><tr key={x.counterparty_entity_id}><td>{x.counterparty_name}</td><td>{Number(x.due_from).toLocaleString()}</td><td>{Number(x.due_to).toLocaleString()}</td></tr>)}</tbody></table></div></div>
    </div>

    <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>Cash Movement</h1></div></div>
    <div className="table-wrap"><table><thead><tr><th>Date</th><th>Account</th><th>Currency</th><th>Movement</th></tr></thead><tbody>{cash.map((x,n)=><tr key={x.financial_account_id+x.date+n}><td>{x.date}</td><td>{x.name}</td><td>{x.currency}</td><td>{Number(x.movement).toLocaleString()}</td></tr>)}</tbody></table></div>

    <div className="page-head" style={{marginTop:28}}><div><h1 style={{fontSize:20}}>General Ledger</h1><p>Most recent 200 lines.</p></div></div>
    <div className="table-wrap"><table><thead><tr><th>Date</th><th>Account</th><th>Description</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{gl.map((x,n)=><tr key={x.journal_id+n}><td>{x.date}</td><td>{x.account_code} · {x.account_name}</td><td>{x.description}</td><td>{Number(x.debit).toLocaleString()}</td><td>{Number(x.credit).toLocaleString()}</td></tr>)}</tbody></table></div>
  </>;
}
