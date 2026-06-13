#!/usr/bin/env bash
# Douyin E2E readonly chain — categories, task-center, operation logs (no platform writes).
# Exit 3 + "blocked_by_real_credentials" when credentials or authorized shop missing.
set -euo pipefail

API_BASE="${TRADEMIND_API_BASE:-http://127.0.0.1:8080}"
API_V1="${API_BASE%/}/api/v1"
ACCOUNT="${TRADEMIND_ADMIN_ACCOUNT:-}"
PASSWORD="${TRADEMIND_ADMIN_PASSWORD:-}"
REPORT_DIR="${DOUYIN_E2E_REPORT_DIR:-./tmp/douyin-e2e}"

mkdir -p "$REPORT_DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$REPORT_DIR/readonly-${TS}.json"

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

CAT_STATS="$(curl_json GET "$API_V1/platform/douyin/categories/stats")"
TC_SUMMARY="$(curl_json GET "$API_V1/task-center/summary")"
TC_FAILURES="$(curl_json GET "$API_V1/task-center/failures?page=1&pageSize=5&keyword=DOUYIN")"
OPLOGS="$(curl_json GET "$API_V1/operation-logs?page=1&pageSize=10&action=douyin")"
DASH="$(curl_json GET "$API_V1/dashboard/product-operations")"

printf '%s\n' "{\"preflight\":$PREFLIGHT,\"categoryStats\":$CAT_STATS,\"taskCenterSummary\":$TC_SUMMARY,\"recentDouyinFailures\":$TC_FAILURES,\"douyinOperationLogs\":$OPLOGS,\"dashboard\":$DASH}" > "$OUT"
echo "report: $OUT"
echo "ok: readonly E2E probes completed"
