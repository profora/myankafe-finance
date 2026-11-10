export type EntityAccess = {
  PublicID: string;
  Name: string;
  Role: string;
  FunctionalCurrency: string;
  Timezone: string;
  FiscalMonth: number;
  FiscalDay: number;
};

export function chooseEntity(entities: EntityAccess[], savedPublicId: string | null): EntityAccess | null {
  if (entities.length === 0) return null;
  const saved = savedPublicId ? entities.find((e) => e.PublicID === savedPublicId) : undefined;
  return saved ?? entities[0];
}
