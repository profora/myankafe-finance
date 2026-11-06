/** Decimal strings only. No floating-point accounting decisions. Scale matches Finance amounts (6 dp). */

const SCALE = 6n;
const BASE = 10n ** SCALE;

export function parseAmount(raw: string): bigint | null {
  const text = raw.trim();
  const match = /^(-)?(\d+)(?:\.(\d{1,6}))?$/.exec(text);
  if (!match) return null;
  const negative = Boolean(match[1]);
  const whole = BigInt(match[2]);
  const padded = (match[3] ?? "").padEnd(6, "0");
  const units = whole * BASE + BigInt(padded || "0");
  return negative ? -units : units;
}

export function formatAmount(units: bigint): string {
  const negative = units < 0n;
  const abs = negative ? -units : units;
  const whole = abs / BASE;
  const frac = (abs % BASE).toString().padStart(6, "0").replace(/0+$/, "");
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

export function sumAmountStrings(values: string[]): string | null {
  const total = addAmounts(values);
  return total == null ? null : formatAmount(total);
}

export function subtractAmountStrings(left: string, right: string): string | null {
  const a = parseAmount(left);
  const b = parseAmount(right);
  if (a == null || b == null) return null;
  return formatAmount(a - b);
}

export function amountsEqual(left: string, right: string): boolean {
  const a = parseAmount(left);
  const b = parseAmount(right);
  return a != null && b != null && a === b;
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
