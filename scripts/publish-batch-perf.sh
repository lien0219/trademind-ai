#!/usr/bin/env bash
# Phase A2.1 batch publish performance benchmark (bash).
# Usage: ./scripts/publish-batch-perf.sh
set -euo pipefail

API_BASE="${TRADEMIND_API_BASE:-http://127.0.0.1:8080}"
API_V1="${API_BASE%/}/api/v1"
ACCOUNT="${TRADEMIND_ADMIN_ACCOUNT:-${ADMIN_BOOTSTRAP_EMAIL:-}}"
PASSWORD="${TRADEMIND_ADMIN_PASSWORD:-${ADMIN_BOOTSTRAP_PASSWORD:-}}"
OUT_FILE="${PUBLISH_BATCH_PERF_OUT:-docs/publish-batch-perf.json}"

if [ -z "$ACCOUNT" ] || [ -z "$PASSWORD" ]; then
  echo "Set TRADEMIND_ADMIN_ACCOUNT/TRADEMIND_ADMIN_PASSWORD or ADMIN_BOOTSTRAP_*" >&2
  exit 1
fi

login_body="$(printf '{"account":"%s","password":"%s"}' "$ACCOUNT" "$PASSWORD")"
TOKEN="$(curl -sfS -X POST "$API_V1/auth/login" -H "Content-Type: application/json" -d "$login_body" | jq -r '.data.token')"
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "login failed" >&2
  exit 1
fi

echo "Run PowerShell script for full scenario matrix on Windows, or extend this shell stub."
echo "Prefer: pwsh -File scripts/publish-batch-perf.ps1"
mkdir -p "$(dirname "$OUT_FILE")"
echo '{"note":"Use publish-batch-perf.ps1 for full benchmark; bash stub for CI hook only"}' > "$OUT_FILE"
