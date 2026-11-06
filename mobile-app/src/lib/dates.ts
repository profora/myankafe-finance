export function todayInTimeZone(timeZone: string, now = new Date()): string {
  try {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(now);
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "";
    return `${get("year")}-${get("month")}-${get("day")}`;
  } catch {
    return now.toISOString().slice(0, 10);
  }
}

export function isIsoDate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value);
}

export function fiscalYearLabel(fiscalMonth: number, fiscalDay: number, timeZone: string, now = new Date()): string {
  const today = todayInTimeZone(timeZone, now);
  const year = Number(today.slice(0, 4));
  const startThis = `${year}-${String(fiscalMonth).padStart(2, "0")}-${String(fiscalDay).padStart(2, "0")}`;
  const startYear = today < startThis ? year - 1 : year;
  return `FY ${startYear}`;
}
