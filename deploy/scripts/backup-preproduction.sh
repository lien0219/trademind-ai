#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENV_FILE="$ROOT/.env"
source "$ROOT/deploy/scripts/load-env-defaults.sh"
load_env_defaults "$ENV_FILE"
COMPOSE=(docker compose --project-name trademind-preproduction --env-file "$ENV_FILE" -f "$ROOT/deploy/preproduction/compose.yml")
node "$ROOT/scripts/preproduction-preflight.mjs" --mode backup >/dev/null

OUT_DIR="${P10_PREPRODUCTION_BACKUP_DIR:?set a protected preproduction backup directory}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$OUT_DIR"
WORK_FILE="$(mktemp "$OUT_DIR/.trademind-preproduction-$STAMP-XXXXXX")"
trap 'rm -f -- "$WORK_FILE"' EXIT
WORK_NAME="${WORK_FILE##*/}"
ARTIFACT="$OUT_DIR/${WORK_NAME#.}.dump"
"${COMPOSE[@]}" exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" --format=custom > "$WORK_FILE"
CHECKSUM="$(sha256sum < "$WORK_FILE")"
printf '%s  %s\n' "${CHECKSUM%% *}" "${ARTIFACT##*/}" > "$ARTIFACT.sha256"
printf '{"schemaVersion":1,"environment":"preproduction","createdAt":"%s","artifact":"%s","checksum":"sha256"}\n' "$STAMP" "${ARTIFACT##*/}" > "$ARTIFACT.metadata.json"
mv -- "$WORK_FILE" "$ARTIFACT"
echo "[preproduction-backup] artifact, checksum, and metadata created"
