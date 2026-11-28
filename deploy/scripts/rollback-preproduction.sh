#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
case "${1:-}" in
  ""|--dry-run) ;;
  *) echo "Usage: $0 [--dry-run]" >&2; exit 2 ;;
esac
source "$ROOT/deploy/scripts/load-env-defaults.sh"
load_env_defaults "$ROOT/.env"
node "$ROOT/scripts/preproduction-preflight.mjs" --mode rollback >/dev/null
export P10_API_IMAGE="$P10_PREVIOUS_API_IMAGE"
export P10_ADMIN_IMAGE="$P10_PREVIOUS_ADMIN_IMAGE"
export P10_COLLECTOR_IMAGE="$P10_PREVIOUS_COLLECTOR_IMAGE"
bash "$ROOT/deploy/scripts/deploy-preproduction.sh" "$@"
if [[ "${1:-}" != --dry-run ]]; then
  echo "[preproduction-rollback] previous application image set is ready; database and volumes retained"
fi
