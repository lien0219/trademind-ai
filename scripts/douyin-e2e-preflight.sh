#!/usr/bin/env bash
# Douyin E2E preflight — config, health, production-preflight, runtime-status.
# Exit 3 + stderr "blocked_by_real_credentials" when App Key/Secret or authorized shop missing.
set -euo pipefail

API_BASE="${TRADEMIND_API_BASE:-http://127.0.0.1:8080}"
API_V1="${API_BASE%/}/api/v1"
ACCOUNT="${TRADEMIND_ADMIN_ACCOUNT:-}"
PASSWORD="${TRADEMIND_ADMIN_PASSWORD:-}"
LIVE_TEST="${DOUYIN_E2E_LIVE_TEST:-false}"
REPORT_DIR="${DOUYIN_E2E_REPORT_DIR:-./tmp/douyin-e2e}"

mkdir -p "$REPORT_DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$REPORT_DIR/preflight-${TS}.json"

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

echo "[douyin-e2e-preflight] health $API_BASE/health"
HEALTH="$(curl -sfS "${API_BASE%/}/health" || true)"
if [ -z "$HEALTH" ]; then
  echo "error: API unreachable at $API_BASE" >&2
  exit 1
fi

if [ -z "$ACCOUNT" ] || [ -z "$PASSWORD" ]; then
  echo "blocked_by_real_credentials" >&2
  echo "hint: set TRADEMIND_ADMIN_ACCOUNT and TRADEMIND_ADMIN_PASSWORD (Douyin creds live in DB settings)" >&2
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

PREFLIGHT_BODY='{}'
if [ "$LIVE_TEST" = "true" ] || [ "$LIVE_TEST" = "1" ]; then
  PREFLIGHT_BODY='{"liveTest":true}'
fi

PREFLIGHT="$(curl_json POST "$API_V1/platform/douyin/production-preflight" "$PREFLIGHT_BODY")"
LATEST="$(curl_json GET "$API_V1/platform/douyin/production-preflight/latest")"
RUNTIME="$(curl_json GET "$API_V1/platform/douyin/runtime-status")"

if echo "$PREFLIGHT" | grep -q '"blockedByRealCredentials"[[:space:]]*:[[:space:]]*true'; then
  printf '%s\n' "{\"health\":$HEALTH,\"preflight\":$PREFLIGHT,\"latest\":$LATEST,\"runtime\":$RUNTIME}" > "$OUT"
  echo "report: $OUT"
  blocked_exit
fi

printf '%s\n' "{\"health\":$HEALTH,\"preflight\":$PREFLIGHT,\"latest\":$LATEST,\"runtime\":$RUNTIME}" > "$OUT"
echo "report: $OUT"
echo "ok: preflight completed (see blockedByRealCredentials=false in $OUT)"
