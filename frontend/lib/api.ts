const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function currentDevUser() {
  if (typeof window === "undefined") return process.env.NEXT_PUBLIC_DEV_USER_ULID ?? "";
  return localStorage.getItem("myankafe-finance-dev-user") ?? process.env.NEXT_PUBLIC_DEV_USER_ULID ?? "";
}

export function setDevUser(v: string) {
  localStorage.setItem("myankafe-finance-dev-user", v);
}

export function clearDevUser() {
  localStorage.removeItem("myankafe-finance-dev-user");
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = currentDevUser();
  const headers = new Headers(init.headers);
  if (init.body != null && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer dev:${token}`);

  const res = await fetch(`${base}${path}`, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  let body: any = {};
  if (text) {
    try { body = JSON.parse(text); }
    catch { body = { error: text }; }
  }

  if (!res.ok) throw new ApiError(res.status, body.error ?? `HTTP ${res.status}`);
  return body as T;
}

export type AuthUser = {
  public_id: string;
  username: string;
  display_name: string;
};

export async function login(username: string, password: string) {
  return api<{user: AuthUser; session_expires_at: string; session_token?: string}>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export async function logout() {
  return api<void>("/auth/logout", { method: "POST", body: "{}" });
}

export async function me() {
  return api<{user: AuthUser}>("/auth/me");
}
