#!/usr/bin/env bash
# Douyin E2E write chain — platform draft / stock sync probes (destructive on test shop).
# Requires ALLOW_DOUYIN_WRITE_TEST=true and real credentials; otherwise exit 3 or 4.
set -euo pipefail

API_BASE="${TRADEMIND_API_BASE:-http://127.0.0.1:8080}"
API_V1="${API_BASE%/}/api/v1"
ACCOUNT="${TRADEMIND_ADMIN_ACCOUNT:-}"
PASSWORD="${TRADEMIND_ADMIN_PASSWORD:-}"
ALLOW_WRITE="${ALLOW_DOUYIN_WRITE_TEST:-}"
PRODUCT_ID="${DOUYIN_E2E_PRODUCT_ID:-}"
SHOP_ID="${DOUYIN_E2E_SHOP_ID:-}"
REPORT_DIR="${DOUYIN_E2E_REPORT_DIR:-./tmp/douyin-e2e}"

mkdir -p "$REPORT_DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$REPORT_DIR/write-${TS}.json"

blocked_exit() {
  echo "blocked_by_real_credentials" >&2
  exit 3
}

curl_json() {
  local method="$1" url="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sfS -X "$method" "$url" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${TOKEN:-}" \
      -d "$body"
  else
    curl -sfS -X "$method" "$url" \
      -H "Authorization: Bearer ${TOKEN:-}"
  fi
}

if [ -z "$ACCOUNT" ] || [ -z "$PASSWORD" ]; then
  blocked_exit
fi

if [ "$ALLOW_WRITE" != "true" ] && [ "$ALLOW_WRITE" != "1" ]; then
  echo "error: set ALLOW_DOUYIN_WRITE_TEST=true to run write probes" >&2
  exit 4
fi

LOGIN_RESP="$(curl -sfS -X POST "$API_V1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"account\":\"$ACCOUNT\",\"password\":\"$PASSWORD\"}")"
TOKEN="$(echo "$LOGIN_RESP" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
if [ -z "$TOKEN" ]; then
  echo "error: login failed" >&2
  exit 1
fi

PREFLIGHT="$(curl_json POST "$API_V1/platform/douyin/production-preflight" '{"liveTest":true}')"
if echo "$PREFLIGHT" | grep -q '"blockedByRealCredentials"[[:space:]]*:[[:space:]]*true'; then
  blocked_exit
fi

RESULT='{"skipped":[]}'
if [ -z "$PRODUCT_ID" ] || [ -z "$SHOP_ID" ]; then
  RESULT='{"skipped":["create-draft","sync-inventory"],"reason":"set DOUYIN_E2E_PRODUCT_ID and DOUYIN_E2E_SHOP_ID for write probes"}'
else
  VALIDATE="$(curl_json POST "$API_V1/products/${PRODUCT_ID}/platform-configs/douyin_shop/validate" "{}")"
  IMG_STATUS="$(curl_json GET "$API_V1/products/${PRODUCT_ID}/platform-configs/douyin_shop/images/status")"
  RESULT=$(printf '{"validate":%s,"imageStatus":%s,"note":"manual create-draft/sync-inventory via admin when ready"}' "$VALIDATE" "$IMG_STATUS")
fi

printf '%s\n' "{\"preflight\":$PREFLIGHT,\"writeProbes\":$RESULT}" > "$OUT"
echo "report: $OUT"
echo "ok: write E2E scaffold completed (see DOUYIN_E2E_CHECKLIST.md for full write steps)"
