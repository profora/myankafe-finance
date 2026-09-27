"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { ApiError, api } from "@/lib/api";
import {
  chooseAccessibleEntity,
  clearStoredEntityID,
  currentEntityQuery,
  entityResourceListPath,
  inaccessibleEntityMessage,
  readStoredEntityID,
  subscribeEntityQuery,
  writeStoredEntityID,
} from "@/lib/entitySelection";
import type { Entity } from "./types";

type Value = {
  entities: Entity[];
  entity?: Entity;
  entityQuery: string;
  platformOwner: boolean;
  setEntityID: (id: string) => void;
  error: string;
  loading: boolean;
  reload: () => void;
};

const Ctx = createContext<Value | null>(null);

export function EntityProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [entities, setEntities] = useState<Entity[]>([]);
  const [platformOwner, setPlatformOwner] = useState(false);
  const [entityID, setEntityIDState] = useState("");
  const [entityQuery, setEntityQuery] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [bootstrapped, setBootstrapped] = useState(false);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const [me, x] = await Promise.all([
        api<{ user: { platform_owner?: boolean } }>("/auth/me"),
        api<{ items: Entity[] }>("/entities"),
      ]);
      const items = x.items ?? [];
      const ids = items.map(item => item.PublicID);
      const query = currentEntityQuery();
      const stored = readStoredEntityID();
      const chosen = chooseAccessibleEntity(ids, [query, stored]);
      if (stored && !ids.includes(stored)) clearStoredEntityID();
      if (chosen) writeStoredEntityID(chosen);
      setPlatformOwner(Boolean(me.user.platform_owner));
      setEntities(items);
      setEntityIDState(chosen);
      setEntityQuery(query);
      setError(query && !ids.includes(query) ? inaccessibleEntityMessage : "");
      setBootstrapped(true);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401 && typeof window !== "undefined") {
        window.location.assign("/login");
        return;
      }
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const unsubscribe = subscribeEntityQuery(() => setEntityQuery(currentEntityQuery()));
    void load();
    return unsubscribe;
  }, []);

  useEffect(() => {
    if (!bootstrapped) return;
    if (!entityQuery) {
      setError(current => current === inaccessibleEntityMessage ? "" : current);
      return;
    }
    if (!entities.some(item => item.PublicID === entityQuery)) {
      setError(inaccessibleEntityMessage);
      return;
    }
    setError(current => current === inaccessibleEntityMessage ? "" : current);
    setEntityIDState(current => {
      if (current === entityQuery) return current;
      writeStoredEntityID(entityQuery);
      return entityQuery;
    });
  }, [entityQuery, entities, bootstrapped]);

  const entity = useMemo(() => entities.find(item => item.PublicID === entityID), [entities, entityID]);

  function setEntityID(id: string) {
    if (!entities.some(item => item.PublicID === id)) return;
    writeStoredEntityID(id);
    setError(current => current === inaccessibleEntityMessage ? "" : current);
    setEntityIDState(id);
    const list = entityResourceListPath(pathname);
    if (list) {
      router.push(list);
      return;
    }
    const params = new URLSearchParams(typeof window === "undefined" ? "" : window.location.search);
    if (params.get("entity") && params.get("entity") !== id) {
      params.set("entity", id);
      const query = params.toString();
      router.replace(query ? `${pathname}?${query}` : pathname);
    }
  }

  return (
    <Ctx.Provider value={{ entities, entity, entityQuery, platformOwner, setEntityID, error, loading, reload: () => { void load(); } }}>
      {children}
    </Ctx.Provider>
  );
}

export function useEntity() {
  const v = useContext(Ctx);
  if (!v) throw new Error("EntityProvider missing");
  return v;
}
