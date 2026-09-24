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
