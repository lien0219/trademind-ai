# P10 Pre-production Architecture

TradeMind uses the existing `staging` profile as P10 pre-production. No separate synonymous runtime profile is introduced.

## Repository Foundation

- `deploy/preproduction/compose.yml` defines PostgreSQL, Redis, backend, Admin and Collector under the fixed `trademind-preproduction` project. The Collector browser profile and storage state use named volumes owned by that project.
- `.env.example` is the only non-secret configuration contract. The target host copies it to `.env`, sets `APP_ENV=staging`, and injects secret values and immutable image references at runtime.
- `.github/workflows/container-images.yml` publishes backend, Admin and Collector images to the same GHCR package. Set `P10_API_IMAGE`, `P10_ADMIN_IMAGE` and `P10_COLLECTOR_IMAGE` to the `image@sha256:<manifest-digest>` references from one Release. The deploy script verifies service names, source revision and release version after pull.
- `scripts/preproduction-preflight.mjs` rejects missing, unknown, production, or non-isolated targets before startup, migration, backup, restore, rollback, or teardown.
- `/health/live` probes the process. `/health/ready` checks database, Redis and startup migrations. The deployment probes Admin's internal API proxy and the backend-to-Collector service token on the Compose network.
- Startup migration uses the existing PostgreSQL advisory lock and AutoMigrate path. The preflight requires an explicit pre-production target before the backend is started.
- Backup publishes a PostgreSQL custom-format dump, a SHA-256 sidecar and metadata. Restore verifies the selected dump before creating a separate pre-production restore database.
- Application rollback requires all three previous immutable images, migration compatibility and readiness verification. PostgreSQL recovery uses the separate restore entry point.
- Teardown removes services but retains isolated volumes by default.

## Isolation Boundary

The contract requires distinct database, Redis, session namespace, public endpoint, and deployment identities for test, pre-production, and production. Cookie domains must be non-overlapping so a production parent-domain cookie cannot be sent to pre-production. The staging profile also requires a non-local storage mode and a credentialed CORS origin matching the pre-production Admin URL. L0 uses fixture and mock modes with real platform access, platform mutations, P10 Workers and business retries disabled.

Backend, Collector, PostgreSQL and Redis use the internal network. Admin also joins a dedicated ingress bridge to publish its loopback port for the host HTTPS reverse proxy. Adopt a verified restore point before an upgrade that changes the schema.

## External Reality

Record the independent host, database and queue identities, HTTPS origin, managed credentials and target-host rehearsal in the release work order. Local Docker rehearsals exercise the same deployment scripts. Target environment acceptance also covers the actual ingress, backup destination and operational alerts.
