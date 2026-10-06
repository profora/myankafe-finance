"use client";

import { daysInFiscalMonth, fiscalMonthNames, fiscalYearLabel } from "@/lib/fiscal";

export default function FiscalYearFields({
  month,
  day,
  onChange,
}: {
  month: number;
  day: number;
  onChange: (month: number, day: number) => void;
}) {
  const max = daysInFiscalMonth(month);
  const invalid = day < 1 || day > max;
  const label = invalid ? "" : fiscalYearLabel(month, day);

  return (
    <div className="field">
      <label>Fiscal year starts</label>
      <div className="form-grid">
        <select aria-label="Fiscal year start month" value={month} onChange={(e) => {
          const next = Number(e.target.value);
          const nextMax = daysInFiscalMonth(next);
          onChange(next, day > nextMax ? 0 : day);
        }}>
          {fiscalMonthNames.map((name, index) => <option key={name} value={index + 1}>{name}</option>)}
        </select>
        <select aria-label="Fiscal year start day" value={invalid ? "" : day} onChange={(e) => onChange(month, Number(e.target.value))}>
          {invalid && <option value="">Select a day</option>}
          {Array.from({ length: max }, (_, index) => index + 1).map((value) => <option key={value} value={value}>{value}</option>)}
        </select>
      </div>
      <div className="muted">{label ? `Fiscal year: ${label}` : "Choose a valid day. Invalid dates are not adjusted automatically."}</div>
    </div>
  );
}
