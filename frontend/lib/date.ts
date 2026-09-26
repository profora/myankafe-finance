export function dateInTimeZone(timeZone = "Asia/Yangon", date = new Date()) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);

  const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
  return `${values.year}-${values.month}-${values.day}`;
}

export function firstDayOfYearInTimeZone(timeZone = "Asia/Yangon", date = new Date()) {
  return `${dateInTimeZone(timeZone, date).slice(0, 4)}-01-01`;
}

export function fiscalYearStartInTimeZone(
  timeZone = "Asia/Yangon",
  fiscalMonth = 1,
  fiscalDay = 1,
  date = new Date(),
) {
  const today = dateInTimeZone(timeZone, date);
  const year = Number(today.slice(0, 4));
  const month = String(fiscalMonth).padStart(2, "0");
  const day = String(fiscalDay).padStart(2, "0");
  const candidate = `${year}-${month}-${day}`;
  return today >= candidate ? candidate : `${year - 1}-${month}-${day}`;
}
