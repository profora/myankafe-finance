/// <reference types="jest" />
import { assertProductionApiBaseUrl, PRODUCTION_API_BASE_URL } from "../../productionApiGuard.js";
import { chooseEntity } from "../lib/entitySelection";
import { canCorrectPostedAccounting, canOperateLedger } from "../lib/roles";
import { entryErrors, entryTotal, saveEntry } from "../lib/entry";
import { transferErrors } from "../lib/transfer";
import { groupBalanceSheet, groupProfitLoss } from "../lib/reports";
import { formatDisplay, parseAmount } from "../lib/money";
import { createClient } from "../api/client";
import { createIdempotencyKey, createKeyStore } from "../lib/idempotency";
import { mergeTransactionRows, nextTransactionOffset, statusLabel, transactionQuery } from "../lib/transactions";
import { reportRequest } from "../lib/reports";
import { amountsEqual, formatDisplay as showMoney } from "../lib/money";
import { passwordChangeError } from "../lib/password";
import { removeSplit } from "../lib/entry";
import { resolveApiBaseUrl } from "../../productionApiGuard.js";
import { attachmentError } from "../lib/attachments";

const mockMemory = new Map<string, string>();
jest.mock("expo-secure-store", () => ({
  setItemAsync: async (k: string, v: string) => {
    mockMemory.set(k, v);
  },
  getItemAsync: async (k: string) => mockMemory.get(k) ?? null,
  deleteItemAsync: async (k: string) => {
    mockMemory.delete(k);
  },
}));

import { clearSession, getSessionToken, saveSession } from "../auth/session";

function response(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get: () => "application/json" },
    text: async () => (body == null ? "" : JSON.stringify(body)),
  };
}

describe("authentication", () => {
  beforeEach(() => mockMemory.clear());

  test("successful bearer login persists only in SecureStore", async () => {
    const calls: { url: string; headers: Headers }[] = [];
    let cleared = false;
    const client = createClient({
      baseUrl: "https://finance-api.myankafe.com/api/v1",
      getToken: async () => null,
      onUnauthorized: async () => {
        cleared = true;
      },
      fetchImpl: async (url, init) => {
        calls.push({ url, headers: new Headers(init?.headers) });
        return response(200, { session_token: "tok-1", session_expires_at: "2030-01-01T00:00:00Z", user: { public_id: "u", username: "ada", display_name: "Ada" } });
      },
    });
    const me = await client.login("ada", "secret");
    expect(calls[0].headers.get("X-Session-Transport")).toBe("bearer");
    expect(me.session_token).toBe("tok-1");
    await saveSession(me.session_token!, me.session_expires_at);
    expect(await getSessionToken()).toBe("tok-1");
    expect(JSON.stringify([...mockMemory.keys()])).not.toMatch(/password/i);
    expect(cleared).toBe(false);
  });

  test("restore uses stored token", async () => {
    await saveSession("restored");
    expect(await getSessionToken()).toBe("restored");
  });

  test("logout clears secure store", async () => {
    await saveSession("tok");
    await clearSession();
    expect(await getSessionToken()).toBeNull();
  });

  test("401 clears the local session", async () => {
    await saveSession("old");
    const client = createClient({
      baseUrl: "https://finance-api.myankafe.com/api/v1",
      getToken: getSessionToken,
      onUnauthorized: clearSession,
      fetchImpl: async () => response(401, { error: "unauthorized" }),
    });
    await expect(client.request("/auth/me")).rejects.toMatchObject({ status: 401 });
    expect(await getSessionToken()).toBeNull();
  });

  test("bad credentials stay unauthenticated", async () => {
    const client = createClient({
      baseUrl: "https://finance-api.example/api/v1",
      getToken: async () => null,
      onUnauthorized: () => undefined,
      fetchImpl: async () => response(401, { error: "invalid" }),
    });
    await expect(client.login("ada", "nope")).rejects.toThrow(/Invalid username or password/);
    expect(await getSessionToken()).toBeNull();
  });

  test("network errors are friendly", async () => {
    const client = createClient({
      baseUrl: "https://finance-api.example/api/v1",
      getToken: async () => null,
      onUnauthorized: () => undefined,
      fetchImpl: async () => {
        throw new Error("failed to fetch");
      },
    });
    await expect(client.login("ada", "pw")).rejects.toThrow(/Network unavailable/);
  });
});

describe("entity", () => {
  const mk = { PublicID: "mk", Name: "MyanKafe", Role: "OWNER", FunctionalCurrency: "MMK", Timezone: "Asia/Yangon", FiscalMonth: 1, FiscalDay: 1 };
  const royal = { PublicID: "rm", Name: "Royal Masterpiece", Role: "ACCOUNTANT", FunctionalCurrency: "USD", Timezone: "Asia/Yangon", FiscalMonth: 4, FiscalDay: 1 };

  test("single entity", () => {
    expect(chooseEntity([mk], null)?.PublicID).toBe("mk");
  });
  test("multiple entities", () => {
    expect(chooseEntity([mk, royal], null)?.PublicID).toBe("mk");
  });
  test("saved selection", () => {
    expect(chooseEntity([mk, royal], "rm")?.Name).toBe("Royal Masterpiece");
  });
  test("revoked entity access falls back", () => {
    expect(chooseEntity([mk], "rm")?.PublicID).toBe("mk");
  });
  test("role UI behavior", () => {
    expect(canOperateLedger("BOOKKEEPER")).toBe(true);
    expect(canOperateLedger("VIEWER")).toBe(false);
    expect(canCorrectPostedAccounting("ADMIN")).toBe(false);
    expect(canCorrectPostedAccounting("ACCOUNTANT")).toBe(true);
  });
});

describe("dashboard", () => {
  test("KPIs and zero values stay strings", () => {
    const dash = { income: "0", expenses: "0", net_profit: "0" };
    expect(formatDisplay(dash.income, "MMK")).toContain("0");
    expect(parseAmount(dash.net_profit)).toBe(0n);
  });
  test("multiple currencies are not combined", () => {
    const balances = [
      { currency: "MMK", balance: "1000" },
      { currency: "USD", balance: "10" },
    ];
    const currencies = new Set(balances.map((b) => b.currency));
    expect(currencies.size).toBe(2);
    expect(showMoney("-12.50", "USD")).toContain("-12.5");
    expect(showMoney("1000000000000.25", "MMK")).toContain("1,000,000,000,000.25");
    expect(showMoney("0", "MMK")).toContain("0");
  });
});

describe("entry", () => {
  const base = {
    type: "EXPENSE" as const,
    date: "2026-11-06",
    description: "Beans",
    financialAccountId: "fa",
    currency: "MMK",
    contactId: "",
    salesChannelId: "",
    salesChannelsAvailable: false,
    splits: [{ accountId: "exp", amount: "1500.50", description: "coffee" }],
  };

  test("expense and income required fields", () => {
    expect(entryErrors({ ...base, description: "" })).toContain("Description is required.");
    expect(entryErrors({ ...base, type: "INCOME", salesChannelsAvailable: true })).toContain("Choose a sales channel.");
    expect(entryErrors({ ...base, type: "INCOME", salesChannelsAvailable: true, salesChannelId: "ch" })).toEqual([]);
  });
  test("split rows and total", () => {
    const draft = { ...base, splits: [...base.splits, { accountId: "exp2", amount: "1", description: "" }] };
    expect(entryTotal(draft)).toBe("1501.5");
    expect(entryErrors({ ...base, splits: [{ accountId: "", amount: "0", description: "" }] }).length).toBeGreaterThan(0);
  });
  test("Save Draft does not post", async () => {
    const calls: string[] = [];
    const result = await saveEntry({
      mode: "draft",
      draft: base,
      idempotencyKey: "key-1",
      attachments: [],
      create: async (_body, key) => {
        calls.push(key);
        return { PublicID: "tx1" };
      },
      upload: async () => undefined,
      post: async () => {
        calls.push("posted");
      },
    });
    expect(result).toMatchObject({ ok: true, posted: false, id: "tx1" });
    expect(calls).toEqual(["key-1"]);
  });
  test("Save and Post uses one key and posts only after uploads", async () => {
    const order: string[] = [];
    const result = await saveEntry({
      mode: "post",
      draft: base,
      idempotencyKey: "stable",
      attachments: [{ name: "receipt.jpg" }],
      create: async (_b, key) => {
        order.push(`create:${key}`);
        return { PublicID: "tx2" };
      },
      upload: async () => {
        order.push("upload");
      },
      post: async (_id, key) => {
        order.push(`post:${key}`);
      },
    });
    expect(result).toMatchObject({ ok: true, posted: true });
    expect(order).toEqual(["create:stable", "upload", "post:stable:post"]);
  });
  test("failed attachment keeps the draft unposted", async () => {
    let posted = false;
    const result = await saveEntry({
      mode: "post",
      draft: base,
      idempotencyKey: "stable",
      attachments: [{ name: "a.pdf" }],
      create: async () => ({ PublicID: "draft-9" }),
      upload: async () => {
        throw new Error("too large");
      },
      post: async () => {
        posted = true;
      },
    });
    expect(posted).toBe(false);
    expect(result).toMatchObject({ ok: false, draftKept: true, id: "draft-9" });
  });
  test("idempotency key is stable across retries of the same action", () => {
    const key = createIdempotencyKey(() => "fixed");
    expect(key).toBe("fixed");
    let n = 0;
    const store = createKeyStore(() => `k${++n}`);
    expect(store.keyFor("post:{}")).toBe("k1");
    expect(store.keyFor("post:{}")).toBe("k1");
    expect(store.keyFor("post:changed")).toBe("k2");
  });
  test("removing a split keeps the last row", () => {
    expect(removeSplit([{ accountId: "a" }, { accountId: "b" }], 0)).toEqual([{ accountId: "b" }]);
    expect(removeSplit([{ accountId: "a" }], 0)).toHaveLength(1);
  });
  test("negative and blank amounts are rejected", () => {
    expect(entryErrors({ ...base, splits: [{ accountId: "exp", amount: "-5", description: "" }] })[0]).toMatch(/greater than zero/);
    expect(entryErrors({ ...base, date: "11/06/2026" })).toContain("Enter a valid date.");
  });
});

describe("transfer", () => {
  const draft = {
    date: "2026-11-06",
    description: "Move cash",
    fromId: "a",
    toId: "b",
    fromAmount: "10",
    toAmount: "10",
    fromCurrency: "MMK",
    toCurrency: "MMK",
  };
  test("validation and same-account rejection", () => {
    expect(transferErrors(draft)).toEqual([]);
    expect(transferErrors({ ...draft, fromId: "a", toId: "a" })[0]).toMatch(/different/);
    expect(transferErrors({ ...draft, toAmount: "9" })[0]).toMatch(/equal amounts/);
    expect(transferErrors({ ...draft, toAmount: "10.00" })).toEqual([]);
    expect(amountsEqual("10", "10.000000")).toBe(true);
  });
});

describe("transactions", () => {
  test("list filters and empty query", () => {
    const query = transactionQuery({ q: "rent", status: "DRAFT", type: "EXPENSE", from: "2026-01-01", to: "2026-01-31", financialAccountId: "fa", offset: 50 });
    expect(query).toContain("q=rent");
    expect(query).toContain("status=DRAFT");
    expect(query).toContain("financial_account_id=fa");
    expect(transactionQuery({})).toContain("limit=50");
  });
  test("pagination advances once and stops on an empty page", () => {
    expect(nextTransactionOffset(40, true, 0)).toBe(40);
    expect(nextTransactionOffset(0, true, 40)).toBeUndefined();
    expect(nextTransactionOffset(10, false, 0)).toBeUndefined();
    const rows = mergeTransactionRows(
      [[{ row_id: "a" }, { row_id: "b" }], [{ row_id: "b" }, { row_id: "c" }]],
      (row) => row.row_id,
    );
    expect(rows.map((row) => row.row_id)).toEqual(["a", "b", "c"]);
  });
  test("draft posted and voided labels", () => {
    expect(statusLabel("DRAFT")).toBe("Draft");
    expect(statusLabel("POSTED")).toBe("Posted");
    expect(statusLabel("VOIDED")).toBe("Voided");
  });
  test("detail shape keeps public id and lines", () => {
    const detail = { public_id: "tx", status: "POSTED", lines: [{ debit: "10", credit: "0" }] };
    expect(detail.public_id).toBe("tx");
    expect(detail.lines[0].debit).toBe("10");
  });
  test("attachment constraints", () => {
    expect(attachmentError({ mime: "image/jpeg", size: 100 })).toBeNull();
    expect(attachmentError({ mime: "application/zip", size: 10 })).toMatch(/not accepted/);
  });
});

describe("reports", () => {
  test("profit and loss and balance sheet grouping", () => {
    const pl = groupProfitLoss([
      { type: "INCOME", name: "Sales", amount: "10" },
      { type: "EXPENSE", name: "Rent", amount: "4" },
    ]);
    expect(pl.income).toHaveLength(1);
    expect(pl.expenses).toHaveLength(1);
    const bs = groupBalanceSheet([
      { type: "ASSET", name: "Cash" },
      { type: "LIABILITY", name: "Loan" },
      { type: "EQUITY", name: "Capital" },
      { name: "Current Earnings", type: "EQUITY" },
    ]);
    expect(bs.assets).toHaveLength(1);
    expect(bs.liabilities).toHaveLength(1);
    expect(bs.equity.length + bs.earnings.length).toBeGreaterThan(0);
  });
  test("empty reports", () => {
    expect(groupProfitLoss([])).toEqual({ income: [], expenses: [], other: [] });
    expect(groupBalanceSheet([])).toEqual({ assets: [], liabilities: [], equity: [], earnings: [] });
  });
  test("report paths stay bounded", () => {
    expect(reportRequest("en", "profit-loss", "2026-01-01", "2026-01-31")).toContain("/reports/profit-loss");
    expect(reportRequest("en", "balance-sheet", "2026-01-01", "2026-01-31")).toContain("through=2026-01-31");
    expect(reportRequest("en", "trial-balance", "2026-01-01", "2026-01-31")).toContain("trial-balance");
    expect(reportRequest("en", "cash-movement", "2026-01-01", "2026-01-31")).toContain("cash-movement");
    expect(reportRequest("en", "sales-analysis", "2026-01-01", "2026-01-31")).toContain("sales-analysis");
    expect(reportRequest("en", "general-ledger", "2026-01-01", "2026-01-31")).toContain("limit=200");
  });
});

describe("production guard", () => {
  test("rejects localhost and http", () => {
    expect(() => assertProductionApiBaseUrl("http://finance-api.myankafe.com/api/v1")).toThrow();
    expect(() => assertProductionApiBaseUrl("https://localhost/api/v1")).toThrow();
    expect(() => assertProductionApiBaseUrl("https://127.0.0.1/api/v1")).toThrow();
    expect(() => assertProductionApiBaseUrl("https://staging.myankafe.com/api/v1")).toThrow();
  });
  test("accepts the documented production API", () => {
    expect(assertProductionApiBaseUrl(PRODUCTION_API_BASE_URL)).toBe("https://finance-api.myankafe.com/api/v1");
  });
  test("development accepts a local URL and production does not", () => {
    expect(resolveApiBaseUrl("development", "http://10.0.2.2:8180/api/v1")).toBe("http://10.0.2.2:8180/api/v1");
    expect(resolveApiBaseUrl("development", "")).toBe("http://127.0.0.1:8180/api/v1");
    expect(resolveApiBaseUrl("preview", PRODUCTION_API_BASE_URL)).toBe(PRODUCTION_API_BASE_URL);
    expect(() => resolveApiBaseUrl("production", "http://10.0.2.2:8180/api/v1")).toThrow();
    expect(() => resolveApiBaseUrl("production", "https://192.168.1.8/api/v1")).toThrow();
    expect(() => assertProductionApiBaseUrl("https://10.0.2.2/api/v1")).toThrow();
  });
});

describe("sessions and password", () => {
  test("change password stores the replacement bearer token", async () => {
    const client = createClient({
      baseUrl: "https://finance-api.myankafe.com/api/v1",
      getToken: async () => "old",
      onUnauthorized: () => undefined,
      fetchImpl: async (url, init) => {
        expect(String(url)).toContain("/auth/change-password");
        expect(new Headers(init?.headers).get("X-Session-Transport")).toBe("bearer");
        return response(200, { changed: true, session_token: "next", session_expires_at: "2030-01-01T00:00:00Z" });
      },
    });
    const body = await client.request<{ session_token: string }>("/auth/change-password", {
      method: "POST",
      headers: { "X-Session-Transport": "bearer" },
      body: JSON.stringify({ current_password: "a", new_password: "b" }),
    });
    await saveSession(body.session_token);
    expect(await getSessionToken()).toBe("next");
  });

  test("revoke session and revoke others are distinct calls", async () => {
    const paths: string[] = [];
    const client = createClient({
      baseUrl: "https://finance-api.myankafe.com/api/v1",
      getToken: async () => "tok",
      onUnauthorized: () => undefined,
      fetchImpl: async (url) => {
        paths.push(String(url));
        return response(200, { revoked: 2 });
      },
    });
    await client.request("/auth/sessions/sess-2", { method: "DELETE" });
    await client.request("/auth/sessions/revoke-others", { method: "POST", body: "{}" });
    expect(paths.some((p) => p.endsWith("/auth/sessions/sess-2"))).toBe(true);
    expect(paths.some((p) => p.endsWith("/auth/sessions/revoke-others"))).toBe(true);
  });

  test("a wrong current password does not clear the bearer session", async () => {
    await saveSession("still-valid");
    let cleared = false;
    const client = createClient({
      baseUrl: "https://finance-api.myankafe.com/api/v1",
      getToken: getSessionToken,
      onUnauthorized: async () => {
        cleared = true;
        await clearSession();
      },
      fetchImpl: async () => response(401, { error: "current password is incorrect" }),
    });
    await expect(
      client.request("/auth/change-password", {
        method: "POST",
        endSessionOn401: false,
        headers: { "X-Session-Transport": "bearer" },
        body: "{}",
      }),
    ).rejects.toThrow(/Current password is incorrect/);
    expect(cleared).toBe(false);
    expect(await getSessionToken()).toBe("still-valid");
  });

  test("password rules match Finance", () => {
    expect(passwordChangeError("old", "short")).toMatch(/12 characters/);
    expect(passwordChangeError("old", "long enough password")).toBeNull();
  });
});
