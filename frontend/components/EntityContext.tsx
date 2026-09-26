"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { Entity } from "./types";

type Value = {
  entities: Entity[];
  entity?: Entity;
  platformOwner: boolean;
  setEntityID: (id: string) => void;
  error: string;
  loading: boolean;
  reload: () => void;
};

const Ctx = createContext<Value | null>(null);

export function EntityProvider({ children }: { children: React.ReactNode }) {
  const [entities, setEntities] = useState<Entity[]>([]);
  const [platformOwner, setPlatformOwner] = useState(false);
  const [entityID, setEntityIDState] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const [me, x] = await Promise.all([
        api<{ user: { platform_owner?: boolean } }>("/auth/me"),
        api<{ items: Entity[] }>("/entities"),
      ]);
      setPlatformOwner(Boolean(me.user.platform_owner));
      setEntities(x.items);
      setEntityIDState((current) => x.items.some(item=>item.PublicID===current) ? current : (x.items[0]?.PublicID || ""));
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

  useEffect(()=>{void load()}, []);

  const entity = useMemo(() => entities.find((x) => x.PublicID === entityID), [entities, entityID]);

  return (
    <Ctx.Provider value={{ entities, entity, platformOwner, setEntityID: setEntityIDState, error, loading, reload: ()=>{void load()} }}>
      {children}
    </Ctx.Provider>
  );
}

export function useEntity() {
  const v = useContext(Ctx);
  if (!v) throw new Error("EntityProvider missing");
  return v;
}
