const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export function currentDevUser() {
  if (typeof window === "undefined") return process.env.NEXT_PUBLIC_DEV_USER_ULID ?? "";
  return localStorage.getItem("myankafe-finance-dev-user") ?? process.env.NEXT_PUBLIC_DEV_USER_ULID ?? "";
}

export function setDevUser(v: string) {
  localStorage.setItem("myankafe-finance-dev-user", v);
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = currentDevUser();
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer dev:${token}`);

  const res = await fetch(`${base}${path}`, { ...init, headers, cache: "no-store" });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error ?? `HTTP ${res.status}`);
  return body as T;
}
