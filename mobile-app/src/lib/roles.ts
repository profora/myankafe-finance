export type EntityRole = "OWNER" | "ADMIN" | "ACCOUNTANT" | "BOOKKEEPER" | "VIEWER" | string;

export function canOperateLedger(role?: string | null): boolean {
  return role === "OWNER" || role === "ADMIN" || role === "ACCOUNTANT" || role === "BOOKKEEPER";
}

export function canCorrectPostedAccounting(role?: string | null): boolean {
  return role === "OWNER" || role === "ACCOUNTANT";
}
