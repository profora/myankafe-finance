export const entityStorageKey = "myankafe-finance.entity";

export const inaccessibleEntityMessage = "That entity is not available to this account.";

export function chooseAccessibleEntity(accessibleIDs: readonly string[], preferred: readonly string[]): string {
  for (const id of preferred) {
    if (id && accessibleIDs.includes(id)) return id;
  }
  return accessibleIDs[0] ?? "";
}

export function withEntity(path: string, entityID: string): string {
  if (!entityID) return path;
  const [base, query = ""] = path.split("?");
  const params = new URLSearchParams(query);
  params.set("entity", entityID);
  const encoded = params.toString();
  return encoded ? `${base}?${encoded}` : base;
}

export function entityResourceListPath(pathname: string): string | null {
  if (/^\/transactions\/(?!new$)[^/]+$/.test(pathname)) return "/transactions";
  if (/^\/accounts\/[^/]+\/ledger$/.test(pathname)) return "/accounts";
  return null;
}

export function readStoredEntityID(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(entityStorageKey) ?? "";
  } catch {
    return "";
  }
}

export function writeStoredEntityID(entityID: string) {
  if (typeof window === "undefined" || !entityID) return;
  try {
    window.localStorage.setItem(entityStorageKey, entityID);
  } catch {
    /* private mode or a full store should not block navigation */
  }
}

export function clearStoredEntityID() {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(entityStorageKey);
  } catch {
    /* ignore storage failures */
  }
}

export function currentEntityQuery(): string {
  if (typeof window === "undefined") return "";
  return new URLSearchParams(window.location.search).get("entity") ?? "";
}

export function subscribeEntityQuery(onChange: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  const originalPush = history.pushState.bind(history);
  const originalReplace = history.replaceState.bind(history);
  history.pushState = ((...args: Parameters<History["pushState"]>) => {
    originalPush(...args);
    onChange();
  }) as History["pushState"];
  history.replaceState = ((...args: Parameters<History["replaceState"]>) => {
    originalReplace(...args);
    onChange();
  }) as History["replaceState"];
  window.addEventListener("popstate", onChange);
  return () => {
    history.pushState = originalPush;
    history.replaceState = originalReplace;
    window.removeEventListener("popstate", onChange);
  };
}
