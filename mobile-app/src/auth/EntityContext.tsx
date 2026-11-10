import { createContext, ReactNode, useContext, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api/runtime";
import { chooseEntity, EntityAccess } from "../lib/entitySelection";
import { loadEntityPreference, saveEntityPreference } from "./entityPreference";
import { useAuth } from "./AuthContext";

type EntityValue = {
  entities: EntityAccess[];
  entity: EntityAccess | null;
  loading: boolean;
  select: (publicId: string) => Promise<void>;
};

const EntityContext = createContext<EntityValue | null>(null);

export function EntityProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [entity, setEntity] = useState<EntityAccess | null>(null);
  const query = useQuery({
    queryKey: ["entities", user?.public_id],
    enabled: Boolean(user),
    queryFn: () => api().request<{ items: EntityAccess[] }>("/entities"),
  });

  useEffect(() => {
    if (!user) {
      setEntity(null);
      return;
    }
    const items = query.data?.items ?? [];
    if (!items.length) return;
    (async () => {
      const saved = await loadEntityPreference();
      const next = chooseEntity(items, saved);
      setEntity(next);
      if (next) await saveEntityPreference(next.PublicID);
    })();
  }, [user, query.data]);

  return (
    <EntityContext.Provider
      value={{
        entities: query.data?.items ?? [],
        entity,
        loading: Boolean(user) && query.isLoading,
        select: async (publicId) => {
          const next = (query.data?.items ?? []).find((item) => item.PublicID === publicId) ?? null;
          setEntity(next);
          if (next) await saveEntityPreference(next.PublicID);
        },
      }}
    >
      {children}
    </EntityContext.Provider>
  );
}

export function useEntity() {
  const value = useContext(EntityContext);
  if (!value) throw new Error("EntityProvider missing");
  return value;
}
