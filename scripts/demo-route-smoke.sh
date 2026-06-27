#!/usr/bin/env bash
# Phase R1 — Unified demo route smoke test.
# Usage: ./scripts/demo-route-smoke.sh [API_BASE] [OUT_FILE]

set -euo pipefail

API_BASE="${1:-http://127.0.0.1:8080}"
OUT_FILE="${2:-docs/demo-route-smoke.json}"
API_V1="$API_BASE/api/v1"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [ -f "$REPO_ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  source <(grep -E '^[A-Z_]+=' "$REPO_ROOT/.env" | sed 's/\r$//')
  set +a
fi

ACCOUNT="${ADMIN_BOOTSTRAP_EMAIL:-}"
PASSWORD="${ADMIN_BOOTSTRAP_PASSWORD:-}"

http_status() {
  local method="$1" url="$2" token="${3:-}"
  if [ -n "$token" ]; then
    curl -s -o /dev/null -w "%{http_code}" -X "$method" -H "Authorization: Bearer $token" "$url" || echo "000"
  else
    curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" || echo "000"
  fi
}

echo "Demo route smoke test against $API_BASE"

HEALTH_JSON=$(curl -s "$API_BASE/health")
HEALTH_TS=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('timestamp',''))" 2>/dev/null || echo "")
HEALTH_STATUS=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('status',''))" 2>/dev/null || echo "")
APP_ENV=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('appEnv',''))" 2>/dev/null || echo "")

if [ -z "$HEALTH_TS" ]; then
  echo "Health check failed" >&2
  exit 1
fi

TOKEN=""
if [ -n "$ACCOUNT" ] && [ -n "$PASSWORD" ]; then
  TOKEN=$(curl -s -X POST "$API_V1/auth/login" -H "Content-Type: application/json" \
    -d "{\"account\":\"$ACCOUNT\",\"password\":\"$PASSWORD\"}" \
    | python3 -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('token',''))" 2>/dev/null || echo "")
fi

declare -a ROUTES=(
  "GET|/health|0"
  "GET|/api/v1/products|1"
  "GET|/api/v1/products/ai-text/batches|1"
  "GET|/api/v1/products/ai-images/batches|1"
  "GET|/api/v1/product-publish/batches|1"
  "GET|/api/v1/ai/operation-workbench/summary|1"
  "GET|/api/v1/ai/operation-workbench/todos?page=1&pageSize=50|1"
  "GET|/api/v1/task-center/failures|1"
)

FAILED=0
ROUTES_JSON="["
FIRST=1
for entry in "${ROUTES[@]}"; do
  IFS='|' read -r METHOD PATH_PART NEED_AUTH <<< "$entry"
  URL="$API_BASE$PATH_PART"
  USE_TOKEN=""
  [ "$NEED_AUTH" = "1" ] && USE_TOKEN="$TOKEN"
  STATUS=$(http_status "$METHOD" "$URL" "$USE_TOKEN")
  OK="true"
  NOTE="status_$STATUS"
  if [ "$STATUS" = "404" ]; then
    OK="false"
    FAILED=$((FAILED + 1))
  fi
  if [ "$NEED_AUTH" = "1" ] && [ -z "$TOKEN" ]; then
    NOTE="login_skipped"
  elif [ "$NEED_AUTH" = "1" ] && { [ "$STATUS" = "401" ] || [ "$STATUS" = "403" ]; }; then
    NOTE="auth_required_ok"
  elif [ "$NEED_AUTH" = "1" ] && [ "$STATUS" = "200" ]; then
    NOTE="authenticated_ok"
  elif [ "$NEED_AUTH" = "0" ] && [ "$STATUS" = "200" ]; then
    NOTE="ok"
  fi
  printf "  %-6s %-70s -> %s %s\n" "$METHOD" "$PATH_PART" "$STATUS" "$( [ "$OK" = "true" ] && echo PASS || echo FAIL )"
  if [ "$FIRST" -eq 0 ]; then ROUTES_JSON+=","; fi
  FIRST=0
  ROUTES_JSON+="{\"method\":\"$METHOD\",\"path\":\"$PATH_PART\",\"auth\":$([ "$NEED_AUTH" = "1" ] && echo true || echo false),\"statusCode\":$STATUS,\"not404\":$OK,\"note\":\"$NOTE\"}"
done
ROUTES_JSON+="]"

mkdir -p "$(dirname "$OUT_FILE")"
PASSED="false"
[ "$FAILED" -eq 0 ] && [ -n "$TOKEN" ] && PASSED="true"

cat > "$OUT_FILE" <<EOF
{
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "apiBase": "$API_BASE",
  "healthStatus": "$HEALTH_STATUS",
  "healthTimestamp": "$HEALTH_TS",
  "appEnv": "$APP_ENV",
  "loggedIn": $([ -n "$TOKEN" ] && echo true || echo false),
  "routeCount": ${#ROUTES[@]},
  "failed404Count": $FAILED,
  "passed": $PASSED,
  "routes": $ROUTES_JSON,
  "note": "task-center path is /api/v1/task-center/failures"
}
EOF

echo ""
echo "Wrote $OUT_FILE"
if [ "$FAILED" -gt 0 ]; then
  echo "$FAILED route(s) returned 404" >&2
  exit 2
fi
if [ -z "$TOKEN" ]; then
  echo "Login required for full smoke pass" >&2
  exit 3
fi
echo "Demo route smoke test PASSED."
exit 0
