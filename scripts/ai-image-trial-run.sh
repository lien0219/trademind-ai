#!/usr/bin/env bash
# Phase A3.2.1 — Real Image Provider trial run for batch AI images (small sample).
# Usage: ./scripts/ai-image-trial-run.sh [API_BASE] [OUT_FILE]

set -euo pipefail

API_BASE="${1:-http://127.0.0.1:8080}"
OUT_FILE="${2:-docs/ai-image-trial-run.json}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if command -v pwsh >/dev/null 2>&1; then
  exec pwsh -ExecutionPolicy Bypass -File "$SCRIPT_DIR/ai-image-trial-run.ps1" -ApiBase "$API_BASE" -OutFile "$OUT_FILE"
fi

echo "PowerShell (pwsh) required for ai-image-trial-run on this platform." >&2
echo "Run: pwsh -File scripts/ai-image-trial-run.ps1" >&2
exit 1
