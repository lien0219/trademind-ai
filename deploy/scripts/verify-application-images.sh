#!/usr/bin/env bash
set -euo pipefail

# A valid digest may still point to the wrong service or a different release.
# Compare the labels of the actual, locally pulled platform images before up.
docker image inspect --format '{{json .Config.Labels}}' "$@" | python3 -c '
import json, re, sys
try:
    labels = [json.loads(line) for line in sys.stdin]
except ValueError:
    sys.exit("could not read production image labels")
services = ("Backend", "Admin", "Collector")
if len(labels) != len(services):
    sys.exit("expected labels for all three production images")
releases = set()
for service, item in zip(services, labels):
    if not isinstance(item, dict) or item.get("org.opencontainers.image.title") != "TradeMind " + service:
        sys.exit("incorrect production image for " + service)
    revision = item.get("org.opencontainers.image.revision", "")
    version = item.get("org.opencontainers.image.version", "")
    if not isinstance(revision, str) or not re.fullmatch(r"[0-9a-f]{40}", revision) or not isinstance(version, str) or not version.strip():
        sys.exit("missing or invalid release labels for " + service)
    releases.add((revision, version))
if len(releases) != 1:
    sys.exit("production images must share the same source revision and release version")
'
