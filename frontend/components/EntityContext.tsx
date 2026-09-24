"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { Entity } from "./types";

type Value = {
  entities: Entity[];
  entity?: Entity;
  setEntityID: (id: string) => void;
  error: string;
  reload: () => void;
};

const Ctx = createContext<Value | null>(null);

export function EntityProvider({ children }: { children: React.ReactNode }) {
  const [entities, setEntities] = useState<Entity[]>([]);
  const [entityID, setEntityIDState] = useState("");
  const [error, setError] = useState("");

  const load = () => {
    setError("");
    api<{ items: Entity[] }>("/entities")
      .then((x) => {
        setEntities(x.items);
        setEntityIDState((current) => current || x.items[0]?.PublicID || "");
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 401 && typeof window !== "undefined") {
          window.location.assign("/login");
          return;
        }
        setError(e instanceof Error ? e.message : String(e));
      });
  };

  useEffect(load, []);

  const entity = useMemo(() => entities.find((x) => x.PublicID === entityID), [entities, entityID]);

  return (
    <Ctx.Provider value={{ entities, entity, setEntityID: setEntityIDState, error, reload: load }}>
      {children}
    </Ctx.Provider>
  );
}

export function useEntity() {
  const v = useContext(Ctx);
  if (!v) throw new Error("EntityProvider missing");
  return v;
}
