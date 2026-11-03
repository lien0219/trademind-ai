#!/usr/bin/env bash
set -euo pipefail
umask 077

# Explicit application-image rollback only. The operator must obtain all
# three prior digests from a previously verified release. Never rolls back
# database schema or restores/deletes persistent volumes.
for name in BACKEND ADMIN COLLECTOR; do
  key="TRADEMIND_PREVIOUS_${name}_IMAGE"
  value="${!key:-}"
  if [[ ! "$value" =~ ^ghcr[.]io/lien0219/trademind@sha256:[[:xdigit:]]{64}$ ]]; then
    echo "[rollback-production] missing or invalid $key" >&2
    exit 1
  fi
  export "TRADEMIND_${name}_IMAGE=$value"
done

if [[ "${TRADEMIND_PRODUCTION_ROLLBACK_APPROVED:-}" != "yes" ]]; then
  echo "[rollback-production] requires explicit rollback approval" >&2
  exit 1
fi
echo "[rollback-production] rolling back application images only; database state is unchanged"
exec "$(dirname "${BASH_SOURCE[0]}")/deploy-production.sh" "$@"
