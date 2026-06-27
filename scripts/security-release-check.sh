#!/usr/bin/env bash
# Phase R1.2-Auto — Security release check wrapper.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_FILE="${1:-docs/SECURITY_RELEASE_CHECK.auto.md}"
if command -v pwsh >/dev/null 2>&1; then
  pwsh -File "$REPO_ROOT/scripts/security-release-check.ps1" -OutFile "$OUT_FILE"
elif command -v powershell >/dev/null 2>&1; then
  powershell -ExecutionPolicy Bypass -File "$REPO_ROOT/scripts/security-release-check.ps1" -OutFile "$OUT_FILE"
else
  echo "PowerShell required for security-release-check.ps1" >&2
  exit 1
fi
