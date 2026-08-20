# P10 Pre-production Architecture

TradeMind uses the existing `staging` profile as P10 pre-production. No separate synonymous runtime profile is introduced.

## Repository Foundation

- `deploy/preproduction/compose.yml` defines isolated PostgreSQL, Redis, backend, Admin, network, and named data volumes under the fixed `trademind-preproduction` project.
- `.env.example` is the only non-secret configuration contract. The target host copies it to `.env`, sets `APP_ENV=staging`, and injects secret values and immutable image references at runtime.
- `.github/workflows/container-images.yml` publishes backend and Admin validation images from `main` into one GHCR package with service-prefixed tags, and publishes stable service tags only from a matching `v<version>` tag contained in `main`, then reports each multi-platform manifest digest. Pre-production `P10_API_IMAGE` and `P10_ADMIN_IMAGE` must use `image@sha256:<manifest-digest>`, never mutable tags.
- `scripts/preproduction-preflight.mjs` rejects missing, unknown, production, or non-isolated targets before startup, migration, backup, restore, rollback, or teardown.
- `/health/live` remains the process probe. `/health/ready` verifies database, Redis, and completed startup migrations; deployment scripts use bounded probes rather than fixed sleep acceptance.
- Startup migration uses the existing PostgreSQL advisory lock and AutoMigrate path. The preflight requires an explicit pre-production target before the backend is started.
- Backup produces a PostgreSQL custom-format artifact, SHA-256 sidecar, and non-secret metadata. Restore is limited to a newly named pre-production restore database and cannot target the live database.
- Application rollback requires explicit previous immutable images, a migration-compatibility assertion, and readiness verification. It never restores the database automatically.
- Teardown removes services but retains isolated volumes by default.

## Isolation Boundary

The contract requires distinct database, Redis, session namespace, public endpoint, and deployment identities for test, pre-production, and production. Cookie domains must be non-overlapping so a production parent-domain cookie cannot be sent to pre-production. The staging profile also requires a non-local storage mode and a credentialed CORS origin matching the pre-production Admin URL. All real Provider/network/read/write, mutation, background queue/worker, and automatic business retry switches remain disabled at L0.

Production restore remains disabled. No claim is made that every migration is reversible.

## External Reality

Current status: `not_provisioned`.

This workspace has no pre-production host, PostgreSQL, Redis, domain, or deployment credential evidence. Production resources must not be used as substitutes. Repository foundation can be validated locally, but P10 Batch 1 cannot complete until independent resources are provisioned and both deployment and teardown are rehearsed.
