"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

export type CurrencyOption={code:string;name:string;symbol:string;decimal_places:number;active:boolean};

export function useActiveCurrencies(){
  const [items,setItems]=useState<CurrencyOption[]>([]);
  const [error,setError]=useState("");
  useEffect(()=>{
    let cancelled=false;
    api<{items:CurrencyOption[]}>("/currencies?active=1")
      .then(result=>{if(!cancelled)setItems(result.items)})
      .catch(e=>{if(!cancelled)setError(e instanceof Error?e.message:String(e))});
    return()=>{cancelled=true};
  },[]);
  return {items,error};
}

export function currencyChoices(active:CurrencyOption[], selected:string[]){
  const seen=new Set(active.map(item=>item.code));
  const extra=selected.filter(code=>code&&!seen.has(code)).map(code=>({code,name:code,symbol:"",decimal_places:2,active:false}));
  return [...active,...extra];
}
