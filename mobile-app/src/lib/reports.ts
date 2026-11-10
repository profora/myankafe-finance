export type PnLRow = { type?: string; account_type?: string; name?: string; code?: string; amount?: string; id?: string };

function rowType(row: PnLRow): string {
  return (row.type || row.account_type || "").toUpperCase();
}

export function groupProfitLoss(items: PnLRow[]) {
  const income = items.filter((r) => rowType(r) === "INCOME");
  const expenses = items.filter((r) => rowType(r) === "EXPENSE");
  return { income, expenses, other: items.filter((r) => !income.includes(r) && !expenses.includes(r)) };
}

export type BalanceRow = { type?: string; account_type?: string; name?: string; code?: string; amount?: string; id?: string };

export function groupBalanceSheet(items: BalanceRow[]) {
  const bucket = (type: string) => items.filter((r) => (r.type || r.account_type || "").toUpperCase() === type);
  const earnings = items.filter((r) => (r.name || "").toLowerCase().includes("current earnings"));
  return {
    assets: bucket("ASSET"),
    liabilities: bucket("LIABILITY"),
    equity: bucket("EQUITY").filter((r) => !earnings.includes(r)),
    earnings,
  };
}

export function reportRequest(entityId: string, report: string, from: string, to: string): string {
  const range = `from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
  if (report === "balance-sheet") return `/entities/${entityId}/reports/balance-sheet?through=${encodeURIComponent(to)}`;
  if (report === "trial-balance") return `/entities/${entityId}/reports/trial-balance?through=${encodeURIComponent(to)}`;
  if (report === "general-ledger") return `/entities/${entityId}/reports/general-ledger?${range}&limit=200`;
  return `/entities/${entityId}/reports/${report}?${range}`;
}
