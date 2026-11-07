#!/usr/bin/env python3
"""Assert disposable Finance journals after Android UI acceptance."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

API = os.environ["FINANCE_API"].rstrip("/")
OWNER_USER = os.environ.get("OWNER_USERNAME", "ci-owner")
OWNER_PASSWORD = os.environ["OWNER_PASSWORD"]
SEED_PATH = os.environ.get("SEED_OUT", "/tmp/finance-e2e-seed.json")


def call(method: str, path: str, token: str | None = None, body=None, headers=None):
    data = None
    req_headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode()
        req_headers["Content-Type"] = "application/json"
    if token:
        req_headers["Authorization"] = f"Bearer {token}"
    if headers:
        req_headers.update(headers)
    req = urllib.request.Request(API + path, data=data, headers=req_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode() or "null"
            return resp.status, json.loads(raw)
    except urllib.error.HTTPError as err:
        raw = err.read().decode()
        try:
            parsed = json.loads(raw) if raw else None
        except json.JSONDecodeError:
            parsed = raw
        return err.code, parsed


def must(status, body, expected, label):
    if status != expected:
        print(f"FAIL {label}: HTTP {status} {body}", flush=True)
        sys.exit(1)
    print(f"PASS {label}: HTTP {status}", flush=True)
    return body


def amount(value: str) -> str:
    return f"{float(value):.6f}"


def main() -> None:
    with open(SEED_PATH, encoding="utf-8") as fh:
        seed = json.load(fh)
    cafe_id = seed["cafe_id"]

    status, body = call(
        "POST",
        "/auth/login",
        None,
        {"username": OWNER_USER, "password": OWNER_PASSWORD},
        {"X-Session-Transport": "bearer"},
    )
    body = must(status, body, 200, "owner login")
    token = body["session_token"]

    status, dash = call("GET", f"/entities/{cafe_id}/dashboard", token)
    dash = must(status, dash, 200, "dashboard")
    assert amount(dash["income"]) == "80000.000000", dash
    assert amount(dash["expenses"]) == "40000.000000", dash
    assert amount(dash["net_profit"]) == "40000.000000", dash
    print("PASS dashboard amounts", flush=True)

    status, balances = call("GET", "/financial-account-balances", token)
    balances = must(status, balances, 200, "balances")
    by_name = {item["name"]: item for item in balances.get("items", []) if item.get("entity_id") == cafe_id}
    assert amount(by_name["Cash Drawer"]["balance"]) == "32000.000000", by_name
    assert amount(by_name["Bank Account"]["balance"]) == "8000.000000", by_name
    print("PASS cash/bank balances", flush=True)

    status, tb = call(
        "GET",
        f"/entities/{cafe_id}/reports/trial-balance?from=2026-11-01&to=2026-11-30",
        token,
    )
    tb = must(status, tb, 200, "trial-balance")
    items = tb.get("items", tb)
    debits = sum(float(row["debits"]) for row in items)
    credits = sum(float(row["credits"]) for row in items)
    assert abs(debits - credits) < 0.000001, (debits, credits)
    assert abs(debits - 80000.0) < 0.000001, debits
    print(f"PASS trial balance debits={debits:.6f} credits={credits:.6f}", flush=True)

    status, txs = call("GET", f"/entities/{cafe_id}/transactions?limit=50", token)
    txs = must(status, txs, 200, "transactions")
    posted = [t for t in txs.get("items", []) if t.get("status") == "POSTED"]
    drafts = [t for t in txs.get("items", []) if t.get("status") == "DRAFT"]
    print(f"POSTED_COUNT {len(posted)} DRAFT_COUNT {len(drafts)}", flush=True)

    expected_lines = {
        ("6210", "25000.000000", "0.000000"),
        ("1110", "0.000000", "25000.000000"),
        ("6520", "10000.000000", "0.000000"),
        ("6220", "5000.000000", "0.000000"),
        ("1110", "0.000000", "15000.000000"),
        ("1110", "80000.000000", "0.000000"),
        ("4110", "0.000000", "80000.000000"),
        ("1121", "8000.000000", "0.000000"),
        ("1110", "0.000000", "8000.000000"),
    }
    seen = set()
    total_debit = 0.0
    total_credit = 0.0
    for tx in posted:
        status, detail = call("GET", f"/entities/{cafe_id}/transactions/{tx['id']}", token)
        detail = must(status, detail, 200, f"detail {tx['id']}")
        for journal in detail.get("journals") or []:
            for line in journal.get("lines") or []:
                code = line["account"]["code"]
                debit = amount(line["debit"])
                credit = amount(line["credit"])
                seen.add((code, debit, credit))
                total_debit += float(debit)
                total_credit += float(credit)
                print(f"JOURNAL {code} Dr {debit} Cr {credit}", flush=True)

    missing = expected_lines - seen
    if missing:
        print(f"FAIL missing journal lines: {missing}", flush=True)
        sys.exit(1)
    assert abs(total_debit - 128000.0) < 0.000001, total_debit
    assert abs(total_credit - 128000.0) < 0.000001, total_credit
    print(f"PASS journal totals debit={total_debit:.6f} credit={total_credit:.6f}", flush=True)
    print("ACCOUNTING_ASSERT_DONE", flush=True)


if __name__ == "__main__":
    main()
