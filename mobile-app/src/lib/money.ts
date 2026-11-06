/** Decimal strings only. No floating-point accounting decisions. Scale is 4. */

const SCALE = 4n;
const BASE = 10n ** SCALE;

export function parseAmount(raw: string): bigint | null {
  const text = raw.trim();
  if (!/^\d+(\.\d{1,4})?$/.test(text)) return null;
  const [whole, frac = ""] = text.split(".");
  const padded = (frac + "0000").slice(0, 4);
  return BigInt(whole) * BASE + BigInt(padded);
}

export function formatAmount(units: bigint): string {
  const negative = units < 0n;
  const abs = negative ? -units : units;
  const whole = abs / BASE;
  const frac = (abs % BASE).toString().padStart(4, "0").replace(/0+$/, "");
  const body = frac ? `${whole.toString()}.${frac}` : whole.toString();
  return negative ? `-${body}` : body;
}

export function addAmounts(values: string[]): bigint | null {
  let total = 0n;
  for (const value of values) {
    const parsed = parseAmount(value);
    if (parsed == null) return null;
    total += parsed;
  }
  return total;
}

export function isPositiveAmount(raw: string): boolean {
  const parsed = parseAmount(raw);
  return parsed != null && parsed > 0n;
}

/** Display only. Does not decide posting. */
export function formatDisplay(raw: string, currency: string): string {
  const parsed = parseAmount(raw.trim());
  const shown = parsed == null ? raw : formatAmount(parsed);
  const grouped = shown.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return currency ? `${grouped} ${currency}` : grouped;
}
