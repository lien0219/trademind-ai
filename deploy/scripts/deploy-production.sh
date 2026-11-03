#!/usr/bin/env bash
set -euo pipefail
umask 077

# Production is deployed from a single verified Release, never from local
# source builds or movable main/latest tags.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${TRADEMIND_PRODUCTION_ENV_FILE:-$ROOT/.env}"
COMPOSE_FILE="$ROOT/deploy/production/compose.yml"
DRY_RUN=false
case "${1:-}" in
  --dry-run) DRY_RUN=true ;;
  "") ;;
  *) echo "Usage: $0 [--dry-run]" >&2; exit 2 ;;
esac

if [[ ! -f "$ENV_FILE" ]]; then
  echo "[deploy-production] missing runtime env file: $ENV_FILE" >&2
  exit 1
fi
command -v docker >/dev/null || { echo "[deploy-production] docker missing" >&2; exit 1; }
command -v python3 >/dev/null || { echo "[deploy-production] python3 missing" >&2; exit 1; }

COMPOSE=(docker compose --project-name trademind-production --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
CONFIG="$("${COMPOSE[@]}" config --format json)"

# Verify all images resolve to immutable digests and the three application
# images are distinct. This validation occurs before any pull or mutation.
python3 -c '
import json, re, sys
try:
    services = json.loads(sys.argv[1])["services"]
except (ValueError, KeyError) as exc:
    sys.exit("invalid production compose config: " + str(exc))
pattern = re.compile(r"^ghcr[.]io/lien0219/trademind@sha256:[0-9a-f]{64}$")
images = [services[name].get("image", "") for name in ("backend", "admin", "collector")]
if not all(pattern.fullmatch(value) for value in images):
    sys.exit("backend/admin/collector must use GHCR sha256 manifest digests from ONE validated release")
if len(set(images)) != 3:
    sys.exit("backend/admin/collector must use distinct manifest digests")
for name in ("postgres", "redis"):
    if name not in services:
        sys.exit("production dependency missing: " + name)
' "$CONFIG"

echo "[deploy-production] compose and immutable image references validated"
if $DRY_RUN; then
  echo "[deploy-production] dry run complete: no images pulled or services changed"
  exit 0
fi
if [[ "${TRADEMIND_PRODUCTION_APPROVED:-}" != "yes" ]] ||
   [[ "${TRADEMIND_PRODUCTION_BACKUP_CONFIRMED:-}" != "yes" ]]; then
  echo "[deploy-production] requires explicit approval and verified off-host backup" >&2
  echo "Set TRADEMIND_PRODUCTION_APPROVED=yes and TRADEMIND_PRODUCTION_BACKUP_CONFIRMED=yes" >&2
  exit 1
fi

# Pull application digests only; never rebuild source or delete named volumes.
"${COMPOSE[@]}" pull backend admin collector
"${COMPOSE[@]}" up -d --wait --wait-timeout 240
"${COMPOSE[@]}" ps
echo "[deploy-production] healthy services; validate external traffic, migrations, workers and release evidence before acceptance"
