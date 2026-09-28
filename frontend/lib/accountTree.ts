export type AccountType = "ASSET" | "LIABILITY" | "EQUITY" | "INCOME" | "EXPENSE";

export type TreeAccount = {
  PublicID: string;
  Code: string;
  Name: string;
  Type: AccountType;
  Subtype?: string | null;
  ParentPublicID?: string | null;
  Postable: boolean;
  Active: boolean;
  HierarchyLocked?: boolean;
};

export type FlatAccount = {
  account: TreeAccount;
  depth: number;
};

export type AccountDraft = {
  mode: "create" | "edit";
  publicID: string;
  code: string;
  name: string;
  type: AccountType;
  parentPublicID: string;
  subtype: string;
  postable: boolean;
  active: boolean;
  hierarchyLocked: boolean;
  error: string;
};

const types: AccountType[] = ["ASSET", "LIABILITY", "EQUITY", "INCOME", "EXPENSE"];

export function isAccountType(value: string): value is AccountType {
  return types.includes(value as AccountType);
}

function byCode(a: TreeAccount, b: TreeAccount) {
  return a.Code.localeCompare(b.Code);
}

function childrenByParent(accounts: TreeAccount[]) {
  const ids = new Set(accounts.map(account => account.PublicID));
  const children = new Map<string, TreeAccount[]>();
  const roots: TreeAccount[] = [];
  for (const account of accounts) {
    const parentID = account.ParentPublicID ?? "";
    if (parentID && parentID !== account.PublicID && ids.has(parentID)) {
      children.set(parentID, [...(children.get(parentID) ?? []), account]);
    } else {
      roots.push(account);
    }
  }
  roots.sort(byCode);
  for (const list of children.values()) list.sort(byCode);
  return { roots, children };
}

export function buildAccountTree(accounts: TreeAccount[]): FlatAccount[] {
  const { roots, children } = childrenByParent(accounts);
  const out: FlatAccount[] = [];
  const seen = new Set<string>();
  const walk = (account: TreeAccount, depth: number) => {
    if (seen.has(account.PublicID) || depth > 32) return;
    seen.add(account.PublicID);
    out.push({ account, depth });
    for (const child of children.get(account.PublicID) ?? []) walk(child, depth + 1);
  };
  for (const root of roots) walk(root, 0);
  for (const account of [...accounts].sort(byCode)) {
    if (!seen.has(account.PublicID)) walk(account, 0);
  }
  return out;
}

export function descendantIDs(accounts: TreeAccount[], rootID: string) {
  const { children } = childrenByParent(accounts);
  const out = new Set<string>();
  const walk = (id: string) => {
    for (const child of children.get(id) ?? []) {
      if (out.has(child.PublicID)) continue;
      out.add(child.PublicID);
      walk(child.PublicID);
    }
  };
  walk(rootID);
  return out;
}

export function parentOptions(accounts: TreeAccount[], type: string, excludeID = "") {
  const blocked = excludeID ? descendantIDs(accounts, excludeID) : new Set<string>();
  if (excludeID) blocked.add(excludeID);
  return accounts
    .filter(account => account.Active && !account.Postable && account.Type === type && !blocked.has(account.PublicID))
    .sort(byCode);
}

export function blankDraft(): AccountDraft {
  return {
    mode: "create",
    publicID: "",
    code: "",
    name: "",
    type: "EXPENSE",
    parentPublicID: "",
    subtype: "",
    postable: true,
    active: true,
    hierarchyLocked: false,
    error: "",
  };
}

export function addChildDraft(parent: TreeAccount): AccountDraft {
  return {
    ...blankDraft(),
    type: parent.Type,
    parentPublicID: parent.PublicID,
  };
}

export function editDraft(account: TreeAccount): AccountDraft {
  return {
    mode: "edit",
    publicID: account.PublicID,
    code: account.Code,
    name: account.Name,
    type: account.Type,
    parentPublicID: account.ParentPublicID ?? "",
    subtype: account.Subtype ?? "",
    postable: account.Postable,
    active: account.Active,
    hierarchyLocked: Boolean(account.HierarchyLocked),
    error: "",
  };
}

export function changeDraftType(draft: AccountDraft, type: AccountType, accounts: TreeAccount[]): AccountDraft {
  const options = parentOptions(accounts, type);
  const parentPublicID = options.some(account => account.PublicID === draft.parentPublicID) ? draft.parentPublicID : "";
  return { ...draft, type, parentPublicID, error: "" };
}

export function draftChanged(current: AccountDraft, initial: AccountDraft) {
  return current.code !== initial.code
    || current.name !== initial.name
    || current.type !== initial.type
    || current.parentPublicID !== initial.parentPublicID
    || current.subtype !== initial.subtype
    || current.postable !== initial.postable
    || current.active !== initial.active;
}

export function afterAccountSave(draft: AccountDraft, error: string): AccountDraft | null {
  if (!error) return null;
  return { ...draft, error };
}

export function accountMutationControls(role?: string) {
  const allowed = role === "OWNER" || role === "ADMIN" || role === "ACCOUNTANT";
  return { create: allowed, edit: allowed, addChild: allowed };
}
