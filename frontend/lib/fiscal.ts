export const fiscalMonthNames = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

const daysByMonth = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];

export function daysInFiscalMonth(month: number) {
  return daysByMonth[month - 1] ?? 0;
}

export function fiscalYearLabel(month: number, day: number) {
  if (month < 1 || month > 12 || day < 1 || day > daysInFiscalMonth(month)) return "";
  const start = new Date(Date.UTC(2001, month - 1, day));
  const end = new Date(start);
  end.setUTCFullYear(2002);
  end.setUTCDate(end.getUTCDate() - 1);
  const format = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
  return `${format.format(start)} → ${format.format(end)}`;
}
