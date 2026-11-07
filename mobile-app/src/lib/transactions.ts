export const TRANSACTION_TYPES = [
  "INCOME",
  "EXPENSE",
  "ACCOUNT_TRANSFER",
  "INTER_ENTITY",
  "MANUAL_JOURNAL",
  "ADJUSTMENT",
  "REVERSAL",
  "OPENING_BALANCE",
  "OPENING_BALANCE_ADJUSTMENT",
] as const;

export const TRANSACTION_STATUSES = ["DRAFT", "POSTED", "VOIDED"] as const;

export type TransactionFilters = {
  q?: string;
  status?: string;
  type?: string;
  from?: string;
  to?: string;
  financialAccountId?: string;
  limit?: number;
  offset?: number;
};

export function transactionQuery(filters: TransactionFilters): string {
  const params = new URLSearchParams();
  if (filters.q) params.set("q", filters.q);
  if (filters.status) params.set("status", filters.status);
  if (filters.type) params.set("type", filters.type);
  if (filters.from) params.set("from", filters.from);
  if (filters.to) params.set("to", filters.to);
  if (filters.financialAccountId) params.set("financial_account_id", filters.financialAccountId);
  params.set("limit", String(filters.limit ?? 50));
  params.set("offset", String(filters.offset ?? 0));
  return params.toString();
}

export function nextTransactionOffset(lastCount: number, hasMore: boolean, previousOffset: number): number | undefined {
  if (!hasMore || lastCount <= 0) return undefined;
  return previousOffset + lastCount;
}

export function mergeTransactionRows<T>(pages: T[][], key: (item: T) => string): T[] {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const page of pages) {
    for (const item of page) {
      const id = key(item);
      if (seen.has(id)) continue;
      seen.add(id);
      out.push(item);
    }
  }
  return out;
}

export function statusLabel(status: string): string {
  if (status === "DRAFT") return "Draft";
  if (status === "POSTED") return "Posted";
  if (status === "VOIDED") return "Voided";
  return status;
}
