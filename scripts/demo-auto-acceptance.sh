#!/usr/bin/env bash
# Phase R1.2-Auto — Demo automated acceptance orchestrator.
# Usage: ./scripts/demo-auto-acceptance.sh [API_BASE]
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API_BASE="${1:-http://127.0.0.1:8080}"

run_ps1() {
  if command -v pwsh >/dev/null 2>&1; then
    pwsh -File "$@"
  elif command -v powershell >/dev/null 2>&1; then
    powershell -ExecutionPolicy Bypass -File "$@"
  else
    echo "PowerShell required for demo-auto-acceptance.ps1" >&2
    exit 1
  fi
}

run_ps1 "$REPO_ROOT/scripts/demo-auto-acceptance.ps1" -ApiBase "$API_BASE"
