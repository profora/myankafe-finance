import type { EntityRole } from "@/components/types";

export function canOperateLedger(role?:EntityRole){
  return role==="OWNER"||role==="ADMIN"||role==="ACCOUNTANT"||role==="BOOKKEEPER";
}
export function canConfigureAccounting(role?:EntityRole){
  return role==="OWNER"||role==="ADMIN"||role==="ACCOUNTANT";
}
export function canCorrectPostedAccounting(role?:EntityRole){
  return role==="OWNER"||role==="ACCOUNTANT";
}
export function canEditOpeningBalances(role?:EntityRole){
  return role==="OWNER"||role==="ACCOUNTANT";
}
export function canViewAudit(role?:EntityRole){
  return role==="OWNER"||role==="ADMIN"||role==="ACCOUNTANT";
}
export function canLockAccounting(role?:EntityRole){
  return role==="OWNER"||role==="ACCOUNTANT";
}
export function canUnlockAccounting(role?:EntityRole){
  return role==="OWNER";
}
export function canManageEntitySettings(role?:EntityRole){
  return role==="OWNER"||role==="ADMIN";
}
export function canManageInterEntitySetup(role?:EntityRole){
  return role==="OWNER"||role==="ADMIN";
}
export function canManagePlatformUsers(role?:EntityRole){
  return role==="OWNER";
}
