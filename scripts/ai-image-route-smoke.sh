#!/usr/bin/env bash
# AI product image route smoke test — verifies /health and /api/v1/products/ai-images/* are registered (not 404).
# Usage: ./scripts/ai-image-route-smoke.sh [API_BASE] [OUT_FILE]

set -euo pipefail

API_BASE="${1:-http://127.0.0.1:8080}"
OUT_FILE="${2:-docs/ai-image-route-smoke.json}"

http_status() {
  local method="$1" url="$2"
  curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" || echo "000"
}

echo "AI image route smoke test against $API_BASE"

HEALTH_JSON=$(curl -s "$API_BASE/health")
HEALTH_TS=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('timestamp',''))" 2>/dev/null || echo "")
HEALTH_STATUS=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('status',''))" 2>/dev/null || echo "")
APP_ENV=$(echo "$HEALTH_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('appEnv',''))" 2>/dev/null || echo "")

if [ -z "$HEALTH_TS" ]; then
  echo "Health check failed" >&2
  exit 1
fi

declare -a ROUTES=(
  "GET|/health"
  "GET|/api/v1/products/ai-images/batches"
  "POST|/api/v1/products/ai-images/batches/check"
  "POST|/api/v1/products/ai-images/batches"
  "GET|/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001"
  "POST|/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/retry-failed"
  "POST|/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/cancel-pending"
  "POST|/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/apply-selected"
  "POST|/api/v1/products/ai-images/batches/00000000-0000-0000-0000-000000000001/undo-applied"
  "POST|/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/regenerate"
  "POST|/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/apply"
  "POST|/api/v1/products/ai-images/items/00000000-0000-0000-0000-000000000001/reject"
)

FAILED=0
ROUTES_JSON="["
FIRST=1
for entry in "${ROUTES[@]}"; do
  METHOD="${entry%%|*}"
  PATH_PART="${entry#*|}"
  URL="$API_BASE$PATH_PART"
  STATUS=$(http_status "$METHOD" "$URL")
  OK="true"
  if [ "$STATUS" = "404" ]; then
    OK="false"
    FAILED=$((FAILED + 1))
  fi
  NOTE="status_$STATUS"
  if [ "$STATUS" = "401" ] || [ "$STATUS" = "403" ]; then NOTE="auth_required_ok"; fi
  if [ "$STATUS" = "200" ]; then NOTE="ok"; fi
  printf "  %-6s %-80s -> %s %s\n" "$METHOD" "$PATH_PART" "$STATUS" "$( [ "$OK" = "true" ] && echo PASS || echo FAIL )"
  if [ "$FIRST" -eq 0 ]; then ROUTES_JSON+=","; fi
  FIRST=0
  ROUTES_JSON+="{\"method\":\"$METHOD\",\"path\":\"$PATH_PART\",\"statusCode\":$STATUS,\"not404\":$OK,\"note\":\"$NOTE\"}"
done
ROUTES_JSON+="]"

mkdir -p "$(dirname "$OUT_FILE")"
PASSED="false"
[ "$FAILED" -eq 0 ] && PASSED="true"

cat > "$OUT_FILE" <<EOF
{
  "generatedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "apiBase": "$API_BASE",
  "healthTimestamp": "$HEALTH_TS",
  "healthStatus": "$HEALTH_STATUS",
  "appEnv": "$APP_ENV",
  "routeCount": ${#ROUTES[@]},
  "failed404Count": $FAILED,
  "passed": $PASSED,
  "routes": $ROUTES_JSON
}
EOF

echo ""
echo "Health timestamp: $HEALTH_TS"
echo "Wrote $OUT_FILE"
if [ "$FAILED" -gt 0 ]; then
  echo "$FAILED route(s) returned 404" >&2
  exit 2
fi
echo "All routes registered (no 404). Smoke test PASSED."
exit 0
