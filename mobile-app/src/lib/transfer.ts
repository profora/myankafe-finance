import { amountsEqual, isPositiveAmount } from "./money";
import { isIsoDate } from "./dates";

export type TransferDraft = {
  date: string;
  description: string;
  fromId: string;
  toId: string;
  fromAmount: string;
  toAmount: string;
  fromCurrency: string;
  toCurrency: string;
};

export function transferErrors(draft: TransferDraft): string[] {
  const errors: string[] = [];
  if (!isIsoDate(draft.date)) errors.push("Enter a valid date.");
  if (!draft.description.trim()) errors.push("Description is required.");
  if (!draft.fromId || !draft.toId) errors.push("Choose both accounts.");
  if (draft.fromId && draft.fromId === draft.toId) errors.push("Source and destination must be different accounts.");
  if (!isPositiveAmount(draft.fromAmount) || !isPositiveAmount(draft.toAmount)) {
    errors.push("Amounts must be greater than zero.");
  }
  if (
    draft.fromCurrency &&
    draft.toCurrency &&
    draft.fromCurrency === draft.toCurrency &&
    isPositiveAmount(draft.fromAmount) &&
    isPositiveAmount(draft.toAmount) &&
    !amountsEqual(draft.fromAmount, draft.toAmount)
  ) {
    errors.push("Same-currency transfers must use equal amounts. Fee conversion stays on Finance web.");
  }
  return errors;
}

export function transferPayload(draft: TransferDraft) {
  return {
    Date: draft.date,
    Description: draft.description.trim(),
    FromFinancialAccountPublicID: draft.fromId,
    ToFinancialAccountPublicID: draft.toId,
    FromAmount: draft.fromAmount.trim(),
    ToAmount: draft.toAmount.trim(),
    FeeAmount: "",
    FeeExpenseAccountPublicID: "",
  };
}
