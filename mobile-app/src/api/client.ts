import { ApiError } from "./errors";

export type FetchLike = (input: string, init?: RequestInit) => Promise<{
  ok: boolean;
  status: number;
  text: () => Promise<string>;
  headers: { get(name: string): string | null };
}>;

export type ClientDeps = {
  baseUrl: string;
  getToken: () => Promise<string | null>;
  onUnauthorized: () => void | Promise<void>;
  fetchImpl?: FetchLike;
};

function serverText(payload: unknown): string {
  if (payload && typeof payload === "object") {
    const obj = payload as Record<string, unknown>;
    if (typeof obj.error === "string") return obj.error;
    if (typeof obj.message === "string") return obj.message;
  }
  return "";
}

function safeAccountingMessage(raw: string, fallback: string): string {
  const text = raw.trim();
  if (!text) return fallback;
  if (/pq:|sql|panic|stack trace|password_hash|session_token/i.test(text)) return fallback;
  return text.length > 300 ? `${text.slice(0, 300)}…` : text;
}

export function messageFrom(payload: unknown, status: number): string {
  const server = serverText(payload);
  if (status === 401) {
    if (/current password/i.test(server)) return "Current password is incorrect.";
    return "Invalid username or password.";
  }
  if (status === 429) return "Too many attempts. Wait a moment and try again.";
  if (status === 403) return "You do not have permission for that action.";
  if (status === 409) return safeAccountingMessage(server, "That action conflicted with one already in progress. Try again.");
  if (status === 400 || status === 422) return safeAccountingMessage(server, "Finance rejected that entry. Check the fields and try again.");
  if (status >= 500) return "Finance is unavailable. Try again.";
  return safeAccountingMessage(server, "Request failed.");
}

export function createClient(deps: ClientDeps) {
  const fetchImpl = deps.fetchImpl ?? (globalThis.fetch as unknown as FetchLike);
  const base = deps.baseUrl.replace(/\/$/, "");

  async function request<T>(path: string, init: RequestInit & { auth?: boolean; idempotencyKey?: string; endSessionOn401?: boolean } = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("Accept", "application/json");
    headers.set("X-Requested-With", "MyanKafe-Finance");
    const body = init.body;
    if (body && typeof body === "string" && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (init.idempotencyKey) headers.set("Idempotency-Key", init.idempotencyKey);
    if (init.auth !== false) {
      const token = await deps.getToken();
      if (token) headers.set("Authorization", `Bearer ${token}`);
    }
    let response: Awaited<ReturnType<FetchLike>>;
    try {
      response = await fetchImpl(`${base}${path}`, { ...init, headers });
    } catch {
      throw new ApiError("Network unavailable. Check your connection and try again.", 0, "network");
    }
    const text = await response.text();
    const payload = text ? safeJson(text) : null;
    if (response.status === 401 && init.auth !== false && init.endSessionOn401 !== false) {
      await deps.onUnauthorized();
      throw new ApiError("Sign in again to continue.", 401);
    }
    if (!response.ok) throw new ApiError(messageFrom(payload, response.status), response.status);
    return payload as T;
  }

  return {
    request,
    login(username: string, password: string) {
      return request<{ session_token?: string; session_expires_at?: string; user: { public_id: string; username: string; display_name: string } }>(
        "/auth/login",
        {
          method: "POST",
          auth: false,
          headers: { "X-Session-Transport": "bearer" },
          body: JSON.stringify({ username, password }),
        },
      );
    },
  };
}

function safeJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return { message: text };
  }
}
