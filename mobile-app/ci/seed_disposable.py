#!/usr/bin/env python3
"""Seed disposable Finance data for Android emulator UI acceptance."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

API = os.environ["FINANCE_API"].rstrip("/")
OWNER_USER = os.environ.get("OWNER_USERNAME", "ci-owner")
OWNER_PASSWORD = os.environ["OWNER_PASSWORD"]
VIEWER_PASSWORD = os.environ["VIEWER_PASSWORD"]
BOOK_PASSWORD = os.environ["BOOKKEEPER_PASSWORD"]
ADMIN_PASSWORD = os.environ["ADMIN_PASSWORD"]
OUT = os.environ.get("SEED_OUT", "/tmp/finance-e2e-seed.json")


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


def pub(obj):
    return obj.get("PublicID") or obj.get("id") or obj.get("public_id")


def login(username: str, password: str):
    status, body = call(
        "POST",
        "/auth/login",
        None,
        {"username": username, "password": password},
        {"X-Session-Transport": "bearer"},
    )
    token = body.get("session_token") if isinstance(body, dict) else None
    return status, token, body


def main() -> None:
    status, token, body = login(OWNER_USER, OWNER_PASSWORD)
    must(status, body, 200, "owner login")
    if not token:
        print("FAIL owner login: missing session_token", flush=True)
        sys.exit(1)

    status, entities = call("GET", "/entities", token)
    entities = must(status, entities, 200, "entities")
    items = entities.get("items") if isinstance(entities, dict) else entities
    by_code = {e["Code"]: e for e in items}
    cafe = by_code["MYANKAFE"]
    personal = by_code["PERSONAL"]
    cafe_id = pub(cafe)
    personal_id = pub(personal)

    for label, entity_id in (("cafe", cafe_id), ("personal", personal_id)):
        status, started = call(
            "PUT",
            f"/entities/{entity_id}/accounting-start-date",
            token,
            {"AccountingStartDate": "2026-11-01"},
        )
        must(status, started, 200, f"{label} accounting start date")

    status, accounts = call("GET", f"/entities/{cafe_id}/accounts", token)
    accounts = must(status, accounts, 200, "chart")
    chart = accounts.get("items") if isinstance(accounts, dict) else accounts
    by_code_acct = {a["Code"]: a for a in chart}
    cash_ledger = by_code_acct["1110"]
    bank_header = by_code_acct["1120"]
    income = by_code_acct["4110"]
    rent = by_code_acct["6210"]
    utilities = by_code_acct["6220"]
    supplies = by_code_acct["6520"]

    status, bank_ledger = call(
        "POST",
        f"/entities/{cafe_id}/accounts",
        token,
        {
            "Code": "1121",
            "Name": "Disposable Test Bank",
            "Type": "ASSET",
            "Subtype": "BANK",
            "ParentPublicID": pub(bank_header),
            "Postable": True,
        },
    )
    bank_ledger = must(status, bank_ledger, 201, "bank ledger")

    status, cash_fin = call(
        "POST",
        f"/entities/{cafe_id}/financial-accounts",
        token,
        {
            "Code": "CASH_DRAWER",
            "Name": "Cash Drawer",
            "Kind": "CASH",
            "Currency": "MMK",
            "AccountPublicID": pub(cash_ledger),
        },
    )
    cash_fin = must(status, cash_fin, 201, "cash financial account")

    status, bank_fin = call(
        "POST",
        f"/entities/{cafe_id}/financial-accounts",
        token,
        {
            "Code": "BANK_MAIN",
            "Name": "Bank Account",
            "Kind": "BANK",
            "Currency": "MMK",
            "AccountPublicID": pub(bank_ledger),
        },
    )
    bank_fin = must(status, bank_fin, 201, "bank financial account")

    status, channel = call(
        "POST",
        f"/entities/{cafe_id}/sales-channels",
        token,
        {"Code": "COUNTER", "Name": "COUNTER", "Active": True},
    )
    channel = must(status, channel, 201, "sales channel")

    users = {}
    for username, password, display, role in (
        ("ci-viewer", VIEWER_PASSWORD, "CI Viewer", "VIEWER"),
        ("ci-bookkeeper", BOOK_PASSWORD, "CI Bookkeeper", "BOOKKEEPER"),
        ("ci-admin", ADMIN_PASSWORD, "CI Admin", "ADMIN"),
    ):
        status, created = call(
            "POST",
            "/users",
            token,
            {"Username": username, "DisplayName": display, "Email": f"{username}@example.test", "Password": password},
        )
        created = must(status, created, 201, f"create {username}")
        status, granted = call(
            "PUT",
            f"/entities/{cafe_id}/users/role",
            token,
            {"user_id": pub(created), "role": role},
        )
        must(status, granted, 200, f"grant {role}")
        if username == "ci-bookkeeper":
            status, personal_role = call(
                "PUT",
                f"/entities/{personal_id}/users/role",
                token,
                {"user_id": pub(created), "role": "VIEWER"},
            )
            must(status, personal_role, 200, "grant bookkeeper PERSONAL VIEWER")
        users[username] = {"id": pub(created), "role": role}

    seed = {
        "api": API,
        "cafe_id": cafe_id,
        "personal_id": personal_id,
        "cash_fin_id": pub(cash_fin),
        "bank_fin_id": pub(bank_fin),
        "cash_ledger_id": pub(cash_ledger),
        "bank_ledger_id": pub(bank_ledger),
        "income_id": pub(income),
        "rent_id": pub(rent),
        "utilities_id": pub(utilities),
        "supplies_id": pub(supplies),
        "channel_id": pub(channel),
        "users": users,
    }
    with open(OUT, "w", encoding="utf-8") as fh:
        json.dump(seed, fh, indent=2)
    print(f"SEED_WRITTEN {OUT}", flush=True)


if __name__ == "__main__":
    main()
