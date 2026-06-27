#!/usr/bin/env bash
# Phase R1.2-Auto — UI copywriting static scan.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_FILE="${1:-docs/COPYWRITING_AUDIT.auto.md}"
cd "$REPO_ROOT"
node scripts/check-ui-copy.mjs --strict --report "$OUT_FILE"
