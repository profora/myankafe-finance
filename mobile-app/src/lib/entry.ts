import { addAmounts, formatAmount, isPositiveAmount } from "./money";
import { isIsoDate } from "./dates";

export type SplitDraft = { accountId: string; amount: string; description: string };

export type EntryDraft = {
  type: "INCOME" | "EXPENSE";
  date: string;
  description: string;
  financialAccountId: string;
  currency: string;
  contactId: string;
  salesChannelId: string;
  salesChannelsAvailable: boolean;
  splits: SplitDraft[];
};

export function entryErrors(draft: EntryDraft): string[] {
  const errors: string[] = [];
  if (!isIsoDate(draft.date)) errors.push("Enter a valid date.");
  if (!draft.description.trim()) errors.push("Description is required.");
  if (!draft.financialAccountId) errors.push("Choose a financial account.");
  if (draft.splits.length === 0) errors.push("Add at least one split.");
  draft.splits.forEach((split, index) => {
    if (!split.accountId) errors.push(`Split ${index + 1} needs an account.`);
    if (!isPositiveAmount(split.amount)) errors.push(`Split ${index + 1} amount must be greater than zero.`);
  });
  if (draft.type === "INCOME" && draft.salesChannelsAvailable && !draft.salesChannelId) {
    errors.push("Choose a sales channel.");
  }
  return errors;
}

export function entryTotal(draft: EntryDraft): string | null {
  const sum = addAmounts(draft.splits.map((s) => s.amount));
  if (sum == null || sum <= 0n) return null;
  return formatAmount(sum);
}

export function entryPayload(draft: EntryDraft) {
  return {
    Type: draft.type,
    Date: draft.date,
    Description: draft.description.trim(),
    FinancialAccountPublicID: draft.financialAccountId,
    Currency: draft.currency,
    ContactPublicID: draft.contactId,
    SalesChannelPublicID: draft.type === "INCOME" ? draft.salesChannelId : "",
    Splits: draft.splits.map((s) => ({
      AccountPublicID: s.accountId,
      Amount: s.amount.trim(),
      Description: s.description.trim(),
    })),
  };
}

export type SaveMode = "draft" | "post";

export type SaveResult =
  | { ok: true; posted: boolean; id: string }
  | { ok: false; draftKept: true; id: string; message: string }
  | { ok: false; draftKept: false; message: string };

export async function saveEntry(args: {
  mode: SaveMode;
  draft: EntryDraft;
  idempotencyKey: string;
  attachments: { name: string }[];
  create: (body: unknown, key: string) => Promise<{ PublicID?: string; id?: string }>;
  upload: (id: string, files: { name: string }[]) => Promise<void>;
  post: (id: string, key: string) => Promise<void>;
}): Promise<SaveResult> {
  const errors = entryErrors(args.draft);
  if (errors.length) return { ok: false, draftKept: false, message: errors[0] };
  let created: { PublicID?: string; id?: string };
  try {
    created = await args.create(entryPayload(args.draft), args.idempotencyKey);
  } catch (err) {
    return { ok: false, draftKept: false, message: err instanceof Error ? err.message : "Could not save the draft." };
  }
  const id = created.PublicID || created.id || "";
  if (!id) return { ok: false, draftKept: false, message: "Finance did not return the draft id." };
  if (args.attachments.length) {
    try {
      await args.upload(id, args.attachments);
    } catch (err) {
      const detail = err instanceof Error ? err.message : "upload failed";
      return {
        ok: false,
        draftKept: true,
        id,
        message: `Draft ${id} was created, but attachment upload failed. It was not posted. ${detail}`,
      };
    }
  }
  if (args.mode === "post") {
    try {
      await args.post(id, `${args.idempotencyKey}:post`);
    } catch (err) {
      return {
        ok: false,
        draftKept: true,
        id,
        message: err instanceof Error ? err.message : "Draft was saved but posting failed.",
      };
    }
  }
  return { ok: true, posted: args.mode === "post", id };
}

export function removeSplit<T>(splits: T[], index: number): T[] {
  if (splits.length <= 1) return splits;
  return splits.filter((_, i) => i !== index);
}
