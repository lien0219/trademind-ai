#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env"
source "$ROOT/deploy/scripts/load-env-defaults.sh"
load_env_defaults "$ENV_FILE"
COMPOSE=(docker compose --project-name trademind-preproduction --env-file "$ENV_FILE" -f "$ROOT/deploy/preproduction/compose.yml")
node "$ROOT/scripts/preproduction-preflight.mjs" --mode restore >/dev/null

ARTIFACT="${1:?usage: restore-preproduction.sh <verified.dump>}"
TARGET_DB="${P10_RESTORE_DATABASE_NAME:?set a new preproduction restore database name}"
[[ "$TARGET_DB" =~ ^trademind_(preproduction|staging)_restore_[a-z0-9_]+$ ]] || { echo "invalid restore database name" >&2; exit 1; }
[[ "$TARGET_DB" != "$DB_NAME" ]] || { echo "restore target must not be the live preproduction database" >&2; exit 1; }
# Compare the selected dump with the expected checksum. The sidecar can
# contain a path from the host that created the backup.
if ! read -r EXPECTED_CHECKSUM _ < "$ARTIFACT.sha256" || [[ ! "$EXPECTED_CHECKSUM" =~ ^[0-9a-f]{64}$ ]]; then
  echo "[preproduction-restore] missing or invalid SHA-256 sidecar" >&2
  exit 1
fi
ACTUAL_CHECKSUM="$(sha256sum < "$ARTIFACT")"
if [[ "${ACTUAL_CHECKSUM%% *}" != "$EXPECTED_CHECKSUM" ]]; then
  echo "[preproduction-restore] backup checksum mismatch" >&2
  exit 1
fi
"${COMPOSE[@]}" exec -T postgres createdb -U "$DB_USER" "$TARGET_DB"
"${COMPOSE[@]}" exec -T postgres pg_restore -U "$DB_USER" -d "$TARGET_DB" --single-transaction --exit-on-error < "$ARTIFACT"
echo "[preproduction-restore] restored to isolated database $TARGET_DB"
