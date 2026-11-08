#!/usr/bin/env bash
# Orchestrate Maestro UI flows against the disposable Finance API on the CI runner.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MOBILE="$ROOT/mobile-app"
API="${FINANCE_API:?}"
export PATH="${PATH}:${HOME}/.maestro/bin"

: "${BOOKKEEPER_PASSWORD:?}"
: "${BOOK_NEXT_PASSWORD:?}"
: "${VIEWER_PASSWORD:?}"
: "${OWNER_PASSWORD:?}"

mkdir -p /tmp/finance-e2e-artifacts
REPORT_DIR=/tmp/finance-e2e-artifacts

run_flow() {
  local file="$1"
  local base rc
  base="$(basename "$file" .yaml)"
  echo "=== Maestro flow: $file ==="
  set +e
  maestro test \
    --format junit \
    --output "$REPORT_DIR/${base}.xml" \
    -e BOOKKEEPER_PASSWORD="$BOOKKEEPER_PASSWORD" \
    -e BOOK_NEXT_PASSWORD="$BOOK_NEXT_PASSWORD" \
    -e VIEWER_PASSWORD="$VIEWER_PASSWORD" \
    "$MOBILE/maestro/$file"
  rc=$?
  set -e
  if [[ "$rc" -ne 0 ]]; then
    adb exec-out screencap -p >"$REPORT_DIR/${base}-fail.png" || true
    timeout 20s adb logcat -d -t 200 >"$REPORT_DIR/${base}-fail.logcat.txt" || true
    adb shell uiautomator dump /sdcard/finance-e2e-ui.xml >/dev/null 2>&1 || true
    adb pull /sdcard/finance-e2e-ui.xml "$REPORT_DIR/${base}-fail-ui.xml" >/dev/null 2>&1 || true
  fi
  return "$rc"
}

adb shell pm list packages | grep com.myankafe.finance
adb shell pm grant com.myankafe.finance android.permission.POST_NOTIFICATIONS || true
adb shell pm grant com.myankafe.finance android.permission.CAMERA || true
adb shell pm grant com.myankafe.finance android.permission.READ_MEDIA_IMAGES || true
adb shell pm grant com.myankafe.finance android.permission.READ_EXTERNAL_STORAGE || true
# Maestro scrolls are full-screen swipes; gesture nav treats them as Back and
# pops New Entry back to the transactions list. Force 3-button navigation.
adb shell settings put secure navigation_mode 0 || true
adb shell cmd overlay enable com.android.internal.systemui.navbar.threebutton || true
adb shell settings put global force_desktop_mode_on_external_displays 0 || true

# Cold-start once so Expo splash/JS settle before Maestro assertions.
adb shell am force-stop com.myankafe.finance || true
adb shell monkey -p com.myankafe.finance -c android.intent.category.LAUNCHER 1 || true
sleep 12
adb exec-out screencap -p > "$REPORT_DIR/boot-before-maestro.png" || true
timeout 20s adb logcat -d -t 200 > "$REPORT_DIR/boot-before-maestro.logcat.txt" || true
adb shell am force-stop com.myankafe.finance || true

run_flow 01_login_invalid.yaml
run_flow 02_login_and_home.yaml
run_flow 03_session_restore.yaml
run_flow 04_entity_switch.yaml
run_flow 05_expense_draft_edit_post.yaml
run_flow 06_income_attachment_post.yaml

echo "Stopping object-store stub for attachment failure"
if [[ -f /tmp/finance-e2e-s3.pid ]]; then
  kill "$(cat /tmp/finance-e2e-s3.pid)" || true
  rm -f /tmp/finance-e2e-s3.pid
  sleep 1
fi
run_flow 07_attachment_failure.yaml

echo "Restarting object-store stub"
python3 "$MOBILE/ci/s3_stub.py" >/tmp/finance-e2e-s3.log 2>&1 &
sleep 1

run_flow 08_split_expense.yaml
run_flow 09_transfer.yaml
run_flow 10_transactions.yaml
run_flow 11_reports.yaml
run_flow 12_account_ledger.yaml

echo "Creating a second disposable bookkeeper session for revoke UI"
OLD_TOKEN="$(
  python3 - <<'PY'
import json, os, urllib.request
req = urllib.request.Request(
    os.environ["FINANCE_API"].rstrip("/") + "/auth/login",
    data=json.dumps({"username": "ci-bookkeeper", "password": os.environ["BOOKKEEPER_PASSWORD"]}).encode(),
    headers={"Content-Type": "application/json", "Accept": "application/json", "X-Session-Transport": "bearer", "User-Agent": "finance-e2e-other-session"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=30) as resp:
    body = json.load(resp)
print(body["session_token"])
PY
)"
printf '%s' "$OLD_TOKEN" >/tmp/finance-e2e-old-book-token
run_flow 13_sessions.yaml

python3 - <<'PY'
import json, os, urllib.error, urllib.request
token = open("/tmp/finance-e2e-old-book-token", encoding="utf-8").read().strip()
req = urllib.request.Request(
    os.environ["FINANCE_API"].rstrip("/") + "/auth/me",
    headers={"Authorization": f"Bearer {token}", "Accept": "application/json"},
    method="GET",
)
try:
    urllib.request.urlopen(req, timeout=30)
    raise SystemExit("FAIL revoked session still accepted")
except urllib.error.HTTPError as err:
    if err.code != 401:
        raise SystemExit(f"FAIL expected 401 for revoked session, got {err.code}")
print("PASS revoked other session rejected by API")
PY

# Capture current bookkeeper token before password change for old-bearer check.
python3 - <<'PY'
import json, os, urllib.request
req = urllib.request.Request(
    os.environ["FINANCE_API"].rstrip("/") + "/auth/login",
    data=json.dumps({"username": "ci-bookkeeper", "password": os.environ["BOOKKEEPER_PASSWORD"]}).encode(),
    headers={"Content-Type": "application/json", "Accept": "application/json", "X-Session-Transport": "bearer", "User-Agent": "finance-e2e-pre-password"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=30) as resp:
    body = json.load(resp)
open("/tmp/finance-e2e-pre-password-token", "w", encoding="utf-8").write(body["session_token"])
print("PASS captured pre-password-change bearer")
PY

run_flow 14_password_change.yaml

python3 - <<'PY'
import json, os, urllib.error, urllib.request
api = os.environ["FINANCE_API"].rstrip("/")
old = open("/tmp/finance-e2e-pre-password-token", encoding="utf-8").read().strip()
req = urllib.request.Request(api + "/auth/me", headers={"Authorization": f"Bearer {old}", "Accept": "application/json"}, method="GET")
try:
    urllib.request.urlopen(req, timeout=30)
    raise SystemExit("FAIL old bearer still accepted after password change")
except urllib.error.HTTPError as err:
    if err.code != 401:
        raise SystemExit(f"FAIL expected 401 for old bearer, got {err.code}")
print("PASS old bearer invalid after password change")

req = urllib.request.Request(
    api + "/auth/login",
    data=json.dumps({"username": "ci-bookkeeper", "password": os.environ["BOOK_NEXT_PASSWORD"]}).encode(),
    headers={"Content-Type": "application/json", "Accept": "application/json", "X-Session-Transport": "bearer"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=30) as resp:
    body = json.load(resp)
assert body.get("session_token"), body
print("PASS login with replacement password")
PY

run_flow 15_role_viewer.yaml

# Viewer direct mutation must 403
python3 - <<'PY'
import json, os, urllib.error, urllib.request
api = os.environ["FINANCE_API"].rstrip("/")
seed = json.load(open(os.environ.get("SEED_OUT", "/tmp/finance-e2e-seed.json"), encoding="utf-8"))
req = urllib.request.Request(
    api + "/auth/login",
    data=json.dumps({"username": "ci-viewer", "password": os.environ["VIEWER_PASSWORD"]}).encode(),
    headers={"Content-Type": "application/json", "Accept": "application/json", "X-Session-Transport": "bearer"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=30) as resp:
    token = json.load(resp)["session_token"]
req = urllib.request.Request(
    api + f"/entities/{seed['cafe_id']}/transactions",
    data=json.dumps({"Type": "EXPENSE", "Date": "2026-11-06", "Description": "viewer probe", "FinancialAccountPublicID": seed["cash_fin_id"], "Currency": "MMK", "Splits": [{"AccountPublicID": seed["rent_id"], "Amount": "1", "Description": ""}]}).encode(),
    headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json", "Accept": "application/json", "Idempotency-Key": "viewer-probe"},
    method="POST",
)
try:
    urllib.request.urlopen(req, timeout=30)
    raise SystemExit("FAIL viewer mutation was allowed")
except urllib.error.HTTPError as err:
    if err.code != 403:
        raise SystemExit(f"FAIL expected viewer 403, got {err.code}")
print("PASS viewer direct API mutation forbidden")
PY

run_flow 16_offline.yaml

echo "MAESTRO_UI_DONE"
