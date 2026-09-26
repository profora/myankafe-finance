"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function InterEntityRedirect(){
  const router=useRouter();
  useEffect(()=>{router.replace("/pay-for-another-entity")},[router]);
  return <p className="muted">Opening Pay for Another Entity…</p>;
}
