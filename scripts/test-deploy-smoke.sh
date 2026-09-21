#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:${TEST_APP_PORT:-8091}}"
COOKIE_JAR="$(mktemp)"
LOGIN_HTML="$(mktemp)"
MANAGER_HTML="$(mktemp)"
HEADERS="$(mktemp)"
cleanup() {
  rm -f "$COOKIE_JAR" "$LOGIN_HTML" "$MANAGER_HTML" "$HEADERS"
}
trap cleanup EXIT INT TERM

csrf_token() {
  awk '$6 == "pabetoop_csrf" { print $7; exit }' "$COOKIE_JAR"
}

require_contains() {
  haystack="$1"
  needle="$2"
  if ! grep -q "$needle" "$haystack"; then
    echo "Smoke test failed: expected $needle in $haystack" >&2
    exit 1
  fi
}

curl -fsS "$BASE_URL/_healthz" >/dev/null
curl -fsS "$BASE_URL/assets/manifest.webmanifest" | grep -q '"start_url": "/portal"'
curl -fsS "$BASE_URL/sw.js" | grep -q 'isSensitiveRoute'

curl -fsS -c "$COOKIE_JAR" "$BASE_URL/login" -o "$LOGIN_HTML"
CSRF="$(csrf_token)"
test -n "$CSRF"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/login" \
  --data-urlencode 'identifier=manager@example.test' \
  --data-urlencode 'password=ChangeMe123!' \
  --data-urlencode "csrf_token=$CSRF"
require_contains "$HEADERS" 'Location: /manager'

curl -fsS -b "$COOKIE_JAR" "$BASE_URL/manager" -o "$MANAGER_HTML"
require_contains "$MANAGER_HTML" 'ARA-180'
require_contains "$MANAGER_HTML" 'ثبت‌نام'
require_contains "$MANAGER_HTML" 'صف پیگیری'
require_contains "$MANAGER_HTML" 'نونهالان پایه'

PLAYER_ID="$(sed -n 's/.*<option value="\([^"]*\)">.*ARA-001<\/option>.*/\1/p' "$MANAGER_HTML" | head -n 1)"
PLAN_ID="$(sed -n 's/.*<option value="\([^"]*\)">نونهالان پایه.*/\1/p' "$MANAGER_HTML" | head -n 1)"
REMINDER_ID="$(sed -n 's/.*\/manager\/reminders\/\([^/\"]*\)\/complete.*/\1/p' "$MANAGER_HTML" | head -n 1)"
test -n "$PLAYER_ID"
test -n "$PLAN_ID"
test -n "$REMINDER_ID"

CSRF="$(csrf_token)"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/manager/subscriptions" \
  --data-urlencode "player_id=$PLAYER_ID" \
  --data-urlencode "plan_id=$PLAN_ID" \
  --data-urlencode 'starts_on=2026-08-21' \
  --data-urlencode "csrf_token=$CSRF"
require_contains "$HEADERS" 'Location: /manager'

CSRF="$(csrf_token)"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/manager/players/$PLAYER_ID/status" \
  --data-urlencode 'status=active' \
  --data-urlencode "csrf_token=$CSRF"
require_contains "$HEADERS" 'Location: /manager'

CSRF="$(csrf_token)"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/manager/reminders/$REMINDER_ID/complete" \
  --data-urlencode "csrf_token=$CSRF"
require_contains "$HEADERS" 'Location: /manager'

curl -fsS -c "$COOKIE_JAR" "$BASE_URL/login" -o "$LOGIN_HTML"
CSRF="$(csrf_token)"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/login" \
  --data-urlencode 'identifier=09170000001' \
  --data-urlencode 'password=ChangeMe123!' \
  --data-urlencode "csrf_token=$CSRF"
require_contains "$HEADERS" 'Location: /portal'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/portal" -o "$LOGIN_HTML"
require_contains "$LOGIN_HTML" 'پرداخت'
require_contains "$LOGIN_HTML" 'ARA-001'

INVOICE_ID="$(sed -n 's/.*href="\/pay\/\([^"]*\)".*/\1/p' "$LOGIN_HTML" | head -n 1)"
test -n "$INVOICE_ID"
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" "$BASE_URL/pay/$INVOICE_ID" -o "$LOGIN_HTML"
require_contains "$LOGIN_HTML" 'مبلغ قابل پرداخت'
CSRF="$(csrf_token)"
curl -sS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$HEADERS" -o /dev/null \
  -X POST "$BASE_URL/pay/$INVOICE_ID/start" \
  --data-urlencode "csrf_token=$CSRF"
PAYMENT_PATH="$(awk '/^Location:/ {gsub("\\r", "", $2); print $2; exit}' "$HEADERS")"
case "$PAYMENT_PATH" in
  /pay/test/*) ;;
  *) echo "Smoke test failed: expected test payment redirect, got $PAYMENT_PATH" >&2; exit 1 ;;
esac
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" "$BASE_URL$PAYMENT_PATH" -o "$LOGIN_HTML"
require_contains "$LOGIN_HTML" 'تأیید پرداخت آزمایشی'
CSRF="$(csrf_token)"
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
  -X POST "$BASE_URL$PAYMENT_PATH/complete" \
  --data-urlencode "csrf_token=$CSRF" -o "$LOGIN_HTML"
require_contains "$LOGIN_HTML" 'پرداخت آزمایشی ثبت شد'

echo "Docker test-deployment smoke test passed: $BASE_URL"
