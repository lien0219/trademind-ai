#!/usr/bin/env bash
# Phase R1 — Demo dataset seed (wrapper; runs PowerShell script on Windows or documents manual steps).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API_BASE="${1:-http://127.0.0.1:8080}"
OUT_FILE="${2:-docs/demo-dataset.json}"

if command -v pwsh >/dev/null 2>&1; then
  pwsh -File "$REPO_ROOT/scripts/seed-demo-data.ps1" -ApiBase "$API_BASE" -OutFile "$OUT_FILE"
elif command -v powershell >/dev/null 2>&1; then
  powershell -ExecutionPolicy Bypass -File "$REPO_ROOT/scripts/seed-demo-data.ps1" -ApiBase "$API_BASE" -OutFile "$OUT_FILE"
else
  echo "PowerShell required to run seed-demo-data.ps1" >&2
  exit 1
fi
