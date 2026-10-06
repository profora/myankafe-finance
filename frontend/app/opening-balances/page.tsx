"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import { canConfigureAccounting, canEditOpeningBalances } from "@/lib/permissions";

type Balance={direction:string;amount:string};
type SavedLine={
  account_public_id:string;
  account_code:string;
  account_name:string;
  account_type:string;
  financial_account_id:string;
  currency:string;
  direction:string;
  amount:string;
  fx_rate_to_functional:string;
};
type Setup={
  accounting_start_date?:string|null;
  editable:boolean;
  view_only:boolean;
  locked_through?:string|null;
  lock_message?:string;
  lines:SavedLine[];
  accounts:{id:string;code:string;name:string;type:string}[];
  financial_accounts:{id:string;name:string;currency:string;account_id:string;account_code:string;account_name:string;account_type:string}[];
  opening_balance_equity:Balance;
  opening_balance_adjustment:Balance;
  initial_opening_entry:boolean;
  functional_currency:string;
};
type Row={
  key:string;
  accountId:string;
  financialAccountId:string;
  accountLabel:string;
  financialLabel:string;
  type:string;
  currency:string;
  debit:string;
  credit:string;
};
type Preview={
  unchanged?:boolean;
  initial?:boolean;
  user_debits?:string;
  user_credits?:string;
  resulting_debits?:string;
  resulting_credits?:string;
  difference?:string;
  functional_currency?:string;
  plug?:{account:string;direction:string;amount:string};
  fx_details?:{currency:string;amount:string;direction:string;rate:string;functional_amount:string}[];
};

function money(direction:string, amount:string){
  if(!direction||!amount||amount.startsWith("0.000000")||amount==="0")return "0";
  return `${amount} ${direction==="CREDIT"?"Cr":"Dr"}`;
}

export default function OpeningBalances(){
  const {entity,reload}=useEntity();
  const mayEdit=canEditOpeningBalances(entity?.Role);
  const maySetStart=canConfigureAccounting(entity?.Role);
  const [setup,setSetup]=useState<Setup|null>(null);
  const [rows,setRows]=useState<Row[]>([]);
  const [startDate,setStartDate]=useState("");
  const [preview,setPreview]=useState<Preview|null>(null);
  const [error,setError]=useState("");
  const [message,setMessage]=useState("");
  const [busy,setBusy]=useState(false);

  function linesFrom(current:Row[]){
    return current.flatMap(row=>{
      const debit=row.debit.trim();
      const credit=row.credit.trim();
      if(debit&&credit)throw new Error(`${row.accountLabel} cannot contain both a debit and a credit.`);
      if(!debit&&!credit)return [];
      return [{
        AccountPublicID:row.accountId,
        FinancialAccountPublicID:row.financialAccountId,
        Direction:debit?"DEBIT":"CREDIT",
        Amount:debit||credit,
      }];
    });
  }

  async function load(){
    if(!entity)return;
    setError("");
    const data=await api<Setup>(`/entities/${entity.PublicID}/opening-balances`);
    setSetup(data);
    setStartDate(data.accounting_start_date??"");
    const saved=data.lines??[];
    const next:Row[]=[];
    for(const account of data.accounts??[]){
      const line=saved.find(item=>!item.financial_account_id&&item.account_public_id===account.id);
      next.push({
        key:account.id,accountId:account.id,financialAccountId:"",
        accountLabel:`${account.code} ${account.name}`,financialLabel:"—",type:account.type,
        currency:data.functional_currency,debit:line?.direction==="DEBIT"?line.amount:"",credit:line?.direction==="CREDIT"?line.amount:"",
      });
    }
    for(const account of data.financial_accounts??[]){
      const line=saved.find(item=>item.financial_account_id===account.id);
      next.push({
        key:account.id,accountId:account.account_id,financialAccountId:account.id,
        accountLabel:`${account.account_code} ${account.account_name}`,financialLabel:account.name,type:account.account_type,
        currency:account.currency,debit:line?.direction==="DEBIT"?line.amount:"",credit:line?.direction==="CREDIT"?line.amount:"",
      });
    }
    setRows(next);
  }

  useEffect(()=>{setPreview(null);setMessage("");if(entity)load().catch(e=>setError(e instanceof Error?e.message:String(e)));},[entity?.PublicID]);

  function update(key:string, field:"debit"|"credit", value:string){
    setPreview(null);
    setRows(current=>current.map(row=>row.key===key?{...row,[field]:value}:row));
  }

  async function saveStart(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      await api(`/entities/${entity.PublicID}/accounting-start-date`,{method:"PUT",body:JSON.stringify({AccountingStartDate:startDate})});
      setMessage("Accounting start date saved.");
      reload();
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function runPreview(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      const result=await api<Preview>(`/entities/${entity.PublicID}/opening-balances/preview`,{method:"POST",body:JSON.stringify({Lines:linesFrom(rows)})});
      setPreview(result);
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  async function save(){
    if(!entity)return;
    setBusy(true);setError("");setMessage("");
    try{
      const result=await api<Setup&{unchanged?:boolean}>(`/entities/${entity.PublicID}/opening-balances`,{method:"PUT",body:JSON.stringify({Lines:linesFrom(rows)})});
      setMessage(result.unchanged?"Opening balances were already saved. No adjustment was created.":"Opening balances saved.");
      setPreview(null);
      await load();
    }catch(e){setError(e instanceof Error?e.message:String(e))}
    finally{setBusy(false)}
  }

  const locked=!setup?.editable;
  const canChange=Boolean(mayEdit&&setup?.editable);

  return <>
    <div className="page-head"><div><h1>Opening Balances</h1><p>The current balance sheet position as of the accounting start date. Posted journals stay unchanged; later edits add adjustment journals.</p></div></div>
    {error&&<div className="alert error">{error}</div>}
    {message&&<div className="alert success">{message}</div>}
    {!entity&&<div className="alert">Select an entity to view opening balances.</div>}
    {entity&&setup&&!setup.accounting_start_date&&<>
      <div className="alert">Set the accounting start date before entering accounting transactions.</div>
      {maySetStart&&<div className="card form">
        <div className="field"><label htmlFor="opening-start-date">Accounting start date</label><input id="opening-start-date" type="date" value={startDate} onChange={e=>setStartDate(e.target.value)}/></div>
        <button disabled={busy||!startDate} onClick={saveStart}>{busy?"Saving…":"Save accounting start date"}</button>
      </div>}
    </>}
    {entity&&setup?.accounting_start_date&&<>
      <div className="card" style={{marginBottom:16}}>
        <p><strong>Accounting start date:</strong> {setup.accounting_start_date}</p>
        {setup.lock_message&&<div className="alert">{setup.lock_message}</div>}
        {setup.view_only&&<div className="alert">Opening balances are read only for this role.</div>}
      </div>
      <div className="table-wrap"><table>
        <thead><tr><th>Account</th><th>Financial account</th><th>Currency</th><th>Debit</th><th>Credit</th></tr></thead>
        <tbody>
          {rows.length===0&&<tr><td colSpan={5}>No balance-sheet accounts are available yet.</td></tr>}
          {rows.map(row=><tr key={row.key}>
            <td>{row.accountLabel}</td>
            <td>{row.financialLabel}</td>
            <td>{row.currency}</td>
            <td><input aria-label={`${row.accountLabel} debit`} inputMode="decimal" disabled={!canChange} value={row.debit} onChange={e=>update(row.key,"debit",e.target.value)}/></td>
            <td><input aria-label={`${row.accountLabel} credit`} inputMode="decimal" disabled={!canChange} value={row.credit} onChange={e=>update(row.key,"credit",e.target.value)}/></td>
          </tr>)}
        </tbody>
      </table></div>
      <div className="card" style={{marginTop:16}}>
        <h3>System balancing accounts</h3>
        <p>Initial Opening Balance Equity (3990) absorbs the difference on the first save. Opening Balance Adjustment (3980) absorbs later differences. They are not edited in the grid.</p>
        <p>Initial Opening Balance Equity: {money(setup.opening_balance_equity?.direction, setup.opening_balance_equity?.amount)}</p>
        <p>Opening Balance Adjustments: {money(setup.opening_balance_adjustment?.direction, setup.opening_balance_adjustment?.amount)}</p>
      </div>
      {preview&&<div className="card" style={{marginTop:16}}>
        <h3>Preview</h3>
        {preview.unchanged&&<p>No change. Saving will not create an adjustment journal.</p>}
        {!preview.unchanged&&<>
          <p>Total user-entered debits: {preview.user_debits} {preview.functional_currency}</p>
          <p>Total user-entered credits: {preview.user_credits} {preview.functional_currency}</p>
          <p>{preview.initial?"Initial Opening Balance Equity":"Opening Balance Adjustment"}: {preview.plug?.direction?`${preview.plug.amount} ${preview.plug.direction}`:"0"}</p>
          {(preview.fx_details??[]).map((fx,index)=><p key={index}>FX: {fx.amount} {fx.currency} {fx.direction} at {fx.rate} = {fx.functional_amount} {preview.functional_currency}</p>)}
          <p>Resulting debits: {preview.resulting_debits}</p>
          <p>Resulting credits: {preview.resulting_credits}</p>
          <p>Difference: {preview.difference}</p>
        </>}
      </div>}
      {canChange&&<div style={{display:"flex",gap:8,marginTop:16,flexWrap:"wrap"}}>
        <button disabled={busy||locked} onClick={runPreview}>Preview</button>
        <button disabled={busy||locked} onClick={save}>{busy?"Saving…":"Save opening balances"}</button>
      </div>}
    </>}
  </>;
}
