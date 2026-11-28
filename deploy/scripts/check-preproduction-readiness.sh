#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE=(docker compose --project-name trademind-preproduction --env-file "$ROOT/.env" -f "$ROOT/deploy/preproduction/compose.yml")
BACKEND_ONLY=false
case "${1:-}" in
  --backend-only) BACKEND_ONLY=true ;;
  "") ;;
  *) echo "Usage: $0 [--backend-only]" >&2; exit 2 ;;
esac

node "$ROOT/scripts/preproduction-preflight.mjs" --mode startup >/dev/null
if ! "${COMPOSE[@]}" exec -T backend curl -qfsS --noproxy '*' --max-time 5 http://127.0.0.1:8080/health/ready | node -e '
  try {
    const payload = JSON.parse(require("node:fs").readFileSync(0, "utf8"));
    const checks = payload.data?.checks || {};
    if (checks.database !== "ok" || checks.redis !== "ok" || checks.migrations !== "ok" || checks.appEnv !== "staging") process.exitCode = 1;
  } catch { process.exitCode = 1; }
'; then
  echo "[preproduction-readiness] API readiness failed" >&2
  exit 1
fi

# Send the shared token on stdin. This keeps the credential out of argv and logs.
if ! "${COMPOSE[@]}" exec -T backend sh -ec '
  endpoint="${COLLECTOR_BASE_URL}/v1/providers"
  status=$(curl -q -sS --noproxy "*" --max-time 5 -o /dev/null -w "%{http_code}" "$endpoint")
  [ "$status" = 401 ]
  printf "Authorization: Bearer %s\n" "$COLLECTOR_SERVICE_TOKEN" |
    curl -qfsS --noproxy "*" --max-time 5 --header @- -o /dev/null "$endpoint"
'; then
  echo "[preproduction-readiness] Collector reachability or service authentication failed" >&2
  exit 1
fi

if ! $BACKEND_ONLY; then
  if ! "${COMPOSE[@]}" exec -T admin sh -ec '
    wget -q -T 5 -O /dev/null http://127.0.0.1/
    wget -q -T 5 -O /dev/null http://127.0.0.1/api/v1/health
  '; then
    echo "[preproduction-readiness] Admin or API proxy check failed" >&2
    exit 1
  fi
  echo "[preproduction-readiness] Admin and API proxy are ready"
fi
echo "[preproduction-readiness] API dependencies, staging profile, and Collector authentication are ready"
