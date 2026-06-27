#!/usr/bin/env bash
# Phase R1.2-Auto — AI operation workbench perf wrapper.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API_BASE="${1:-http://127.0.0.1:8080}"
OUT_FILE="${2:-docs/ai-operation-workbench-perf.auto.json}"
if command -v pwsh >/dev/null 2>&1; then
  pwsh -File "$REPO_ROOT/scripts/ai-operation-workbench-perf.ps1" -ApiBase "$API_BASE" -OutFile "$OUT_FILE"
elif command -v powershell >/dev/null 2>&1; then
  powershell -ExecutionPolicy Bypass -File "$REPO_ROOT/scripts/ai-operation-workbench-perf.ps1" -ApiBase "$API_BASE" -OutFile "$OUT_FILE"
else
  echo "PowerShell required" >&2
  exit 1
fi
