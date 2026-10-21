#!/usr/bin/env bash
set -Eeuo pipefail

for variable in TRADEMIND_BACKEND_IMAGE TRADEMIND_ADMIN_IMAGE TRADEMIND_COLLECTOR_IMAGE; do
  value="${!variable:-}"
  if [[ ! "$value" =~ ^ghcr\.io/lien0219/trademind@sha256:[0-9a-f]{64}$ ]]; then
    echo "::error::$variable must be an immutable ghcr.io/lien0219/trademind@sha256:<manifest-digest> reference" >&2
    exit 1
  fi
done

printf '%s\n' "Production image references are immutable and use the unified GHCR package."
