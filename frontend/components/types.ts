export type EntityRole = "OWNER" | "ADMIN" | "ACCOUNTANT" | "BOOKKEEPER" | "VIEWER";

export type Entity = {
  ID: string;
  PublicID: string;
  Code: string;
  Name: string;
  Type: string;
  FunctionalCurrency: string;
  Timezone: string;
  Role: EntityRole;
  FiscalMonth: number;
  FiscalDay: number;
};

export type Account = {
  PublicID: string;
  Code: string;
  Name: string;
  Type: "ASSET" | "LIABILITY" | "EQUITY" | "INCOME" | "EXPENSE";
  Subtype?: string | null;
  Postable: boolean;
  Active: boolean;
};

export type FinancialAccount = {
  PublicID: string;
  Code: string;
  Name: string;
  Kind: string;
  Currency: string;
  AccountPublicID: string;
  Institution?: string | null;
  Reference?: string | null;
  Active: boolean;
};

export type Transaction = {
  PublicID: string;
  Type: string;
  Status: "DRAFT" | "POSTED" | "VOIDED";
  Date: string;
  Description: string;
  Currency: string;
  Total: string;
};
