# Changelog

All notable changes to TradeMind are documented here.

## Unreleased

### ERP purchase returns (2026-08-22)

- Added tenant-scoped, receipt-bound purchase returns with cumulative over-return protection, revision-checked state transitions, action idempotency, separated approval/execution duties, and cancellation that releases receipt allocation.
- Completed returns atomically deduct warehouse available stock and update immutable movements, compatibility logs, and the `product_skus.stock` projection; insufficient stock rolls back the entire return.
- Added Admin list/detail/create workflows, permission-aware and readonly controls, audit labels, API contracts, PostgreSQL allocation-concurrency coverage, and five-viewport/write-safety regression coverage without enabling supplier settlement, real-platform inventory writes, automatic replenishment, or new workers.

### Order and inventory production hardening (2026-08-21)

- Enforced order-operation permission checks on every Admin order write endpoint, including order, line-item and shipment mutations.
- Prevented replacing order lines after a successful inventory reservation or deduction, and locked the line row during inventory application to serialize concurrent lifecycle updates.
- Stabilized warehouse-ledger regression assertions so movement verification does not depend on timestamp or UUID ordering.

### Container package consolidation (2026-08-21)

- Restricted automatic container image publication to image-related pushes on `main`, while retaining validated `v<version>` releases and manual runs that fail closed outside `main` or a version tag.
- Consolidated backend, Admin and Collector images under one `trademind` GHCR package with service-prefixed validation, version, SHA and latest tags, preserving separate multi-platform manifests and immutable deployment digests.

### ERP procurement foundation (2026-08-20)

- Added tenant-scoped warehouse and supplier master data, supplier-to-product SKU bindings, and separated view/manage permissions.
- Added revision-protected purchase-order submission, approval, cancellation and closure, plus transactional partial receipts with over-receipt rejection and payload-bound idempotency.
- Added warehouse stock balances and immutable purchase-receipt movements while preserving `product_skus.stock` as the compatibility authority until all legacy stock writers are migrated.
- Added API contracts, role-matrix regressions, procurement transaction tests, and the staged ERP architecture boundary without enabling real-platform inventory writes.
- Added the production-oriented Admin procurement workspace for warehouse and supplier maintenance, purchase-order creation and review, revision-checked state transitions, and idempotent partial receipt confirmation, with responsive and write-safety regression coverage.
- Migrated manual inventory adjustments to warehouse-selected, idempotent transactions that atomically update warehouse balances, immutable movements, compatibility logs, and the `product_skus.stock` aggregate projection.
- Added bounded historical-stock migration to the default or pending-allocation warehouse, reconciliation APIs and Admin workspace, and PostgreSQL concurrency coverage without enabling platform inventory writes, automatic replenishment, or new workers.
- Separated SKU metadata from inventory writes: create and update reject direct stock values, newly created SKUs start at zero, and all SKU mutation routes enforce product-write permission and tenant visibility.
- Migrated order inventory lifecycle to the warehouse ledger: payment/processing reserves `reserved`, shipment/fulfillment deducts `on_hand`, pre-shipment cancellation releases reservations, and post-deduction refund/cancel restores on-hand stock. Added tenant/warehouse effect binding, reserved movement snapshots, legacy-effect backfill, locked-order-line protection, platform-sync preservation, and responsive/write-safety regression coverage without enabling real platform inventory writes, automatic replenishment, or new workers.

### Database migration reliability (2026-08-15)

- Reconciled legacy and canonical PostgreSQL index names only when their definitions are otherwise identical, while keeping different definitions fail closed, so repeated `AutoMigrate` startup no longer fails on an equivalent duplicate inventory index.

### Naming consistency (2026-08-14)

- Normalized Go identifiers to standard initialisms such as `ID`, `SKU`, `URL`, `API`, `HTTP`, `JSON`, `UUID` and `OAuth` without changing JSON fields or API payloads.
- Renamed `ai_image_task_items` to `image_task_items` and removed the `p7` phase marker from nine production performance index names through transactional, fail-closed upgrades that preserve existing data.
- Replaced phase-numbered source paths with responsibility-based names across backend migrations/modules, Admin inventory sync, integration tests, pre-production scripts, and production documentation; compatibility environment variables, routes, and legacy schema inputs remain unchanged.
- Added affected quality checks for Go initialisms and lowercase, domain-oriented table, index, constraint, trigger and function names.

### Multi-architecture container publishing (2026-08-14)

- Added a single `deploy/IMAGE_VERSION` source and automated GHCR publishing for backend, Admin and Collector images.
- Published normalized branch, full-SHA and branch-version tags for validation builds; validated `v<version>` tags on `main` publish stable version tags and `latest`.
- Added Linux AMD64/ARM64 manifests, OCI metadata, SBOM, provenance and Compose selectors for local builds or prebuilt images.
- Kept image publication separate from deployment, traffic changes, database operations and real-platform activation.

### Application backup and restore retirement (2026-08-14)

- Removed application-level backup management and restore validation Admin pages, APIs, permissions, configuration, services, metrics, default alerts, dashboard and module-specific runbooks.
- Delegated automatic backups, encryption, retention, PITR, alerting and isolated restore drills to the cloud database and operations platform while retaining external disaster-recovery, database rollback and P10 pre-production release gates.
- Stopped managing legacy `backup_jobs`, `backup_verifications` and `restore_jobs` through `AutoMigrate`; existing tables and rows remain untouched pending an explicit retention or archival decision.

### Observability production hardening (2026-08-14)

- Added explicit trusted-proxy and metrics CIDR allowlists with staging/production fail-closed validation and exact Nginx protection for `/internal/metrics`.
- Replaced lifetime aggregate alert and SLO evaluation with bounded structured snapshots, window deltas, ratio sample guards, counter reset handling, histogram evaluation, and synchronized code-owned rule definitions that preserve administrator enablement choices.
- Expanded the protected overview contract and rebuilt the Admin observability center with operational statuses, retained-data refresh errors, responsive layout, and regression coverage. Backup and isolated restore metrics added during this work were subsequently removed with the application-level module retirement above.
- Removed legacy observability sub-endpoints that only returned fixed placeholder aggregates instead of live operational data.
- Kept real Prometheus/OTLP backends, external notification delivery, deployment, credentials, and production activation outside this code change and subject to target-environment human acceptance.

### Release recorder retirement (2026-08-13)

- Removed the development-only Admin page, `/api/v1/ops/releases*` endpoints, dedicated permissions and unused `RELEASE_*` configuration.
- Removed the self-reporting release state machine, unpopulated metrics, misleading default alerts, dashboard and module-specific runbooks while retaining external CI/CD, deployment scripts, release approval and application rollback procedures.
- Stopped managing legacy `release_runs`, `release_artifacts`, `release_steps` and `release_rollbacks` through `AutoMigrate`; existing rows remain untouched pending an explicit retention or archival decision.

### Disaster-recovery drill recorder retirement (2026-08-13)

- Removed the development-only Admin page, `/api/v1/ops/dr/*` endpoints, dedicated permissions, unused schedule configuration, unpopulated metrics and their misleading default alert.
- Retained application rollback and the disaster-recovery boundary. Application-level backup verification, isolated restore validation and WAL/PITR checks were subsequently delegated to the external platforms described above.
- Stopped managing the legacy `dr_drills` table through `AutoMigrate`; existing rows remain untouched pending an explicit retention or archival decision.

### Operation task center production hardening (2026-08-13)

- Added permission-aware task creation, restorable list filters and cursor history, detail Tab deep links, explicit stale/partial-error states, and responsive Admin workflows.
- Hardened draft, review, execution, retry, and cancellation actions with duplicate-submit guards, stable idempotency keys, bound revision/draft data, and retained form input after recoverable failures.
- Replaced per-row draft and attempt lookups with fixed-count batch queries and propagated related database failures instead of returning incomplete successful details.
- Added a default-off, operation-task-only Douyin platform-draft path with immutable canonical-hash snapshots, human approval, transactional execution/outbox records, pre-provider binding/runtime guards, and transactional local result persistence.
- Blocked legacy direct create, traditional publish, multi-target/batch and legacy retry bypasses before local writes; unknown platform results require manual read-only reconciliation and never trigger automatic recreation.
- Added one-tenant/one-shop database constraints, two-distinct-admin gray approval, five fail-closed kill switches, L3 startup validation for queue/reaper availability, and persistent unit, contract, backend, smoke, write-safety, and five-viewport regression coverage.
- Kept repository defaults at L0. This change does not include credentials, deployment, online publication, inventory mutation, automatic business retry, multi-shop rollout or production activation.

### AI customer service production hardening (2026-08-12)

- Fixed manual platform-message delivery to include the backend-required client message id.
- Added default-off, shop-scoped low-risk AI auto replies with tenant-scoped Redis workers, idempotent run records, rate and context guards, sensitive-commitment blocking, auditability, and no automatic business retry.
- Added Admin policy controls with explicit confirmation, readonly protection, recent run status, API contract coverage, and deployment documentation.
- Added database-backed run leases, reliable Redis reservation/ack recovery, multi-instance polling claims, final pre-send conversation checks, PostgreSQL uniqueness/index guards, and stale Admin request isolation.

### Native alert robots (2026-08-11)

- Added native Feishu custom-bot delivery with optional timestamp signing and native Enterprise WeChat group-robot delivery.
- Validate vendor response codes even on HTTP 2xx, retain masked audit targets, and keep production delivery HTTPS-only with bounded timeouts and response summaries.
- Exposed both implemented channels in the production Admin settings flow and retained encrypted webhook and signing-secret storage.

### Database schema naming (2026-08-10)

- Replaced phase-numbered `p9_*` and `p10_*` table names with stable inventory, SKU binding, platform credential, OAuth, and production-control domain names.
- Added a transactional, fail-closed PostgreSQL upgrade that renames existing tables and their indexes, constraints, triggers, and immutable-record function without copying or deleting data.
- Kept API routes, permissions, state machines, and the fail-closed P10 `L0` runtime boundary unchanged.

### Admin theme (2026-08-09)

- Added an icon-only, tooltip-labelled top-navigation light/dark theme switch with light mode as the default and local preference persistence.
- Applied Ant Design theme tokens across shared Admin chrome, login, dashboard, status surfaces, and responsive regression coverage.
- Moved the complete desktop brand and its sider toggle into the fixed top header, added permission-aware navigation search with a compact mobile entry, kept a compact mobile brand beside the menu trigger, removed the duplicate sidebar brand, and made the navigation drawer opaque above scrolled content.
- Made theme switching atomic and fully reversible for header, elevated, and portal surfaces, and safely centered mobile login and registration layouts.

### Production maintenance cleanup (2026-08-09)

- Removed historical phase gates, load-test harnesses, generated evidence, one-off acceptance scripts, and local Playwright/test outputs from the working tree.
- Removed residual P6/P7 verification commands, unreferenced backend placeholders, a one-off Admin codemod, and unused Admin/Collector symbols.
- Deduplicated the Admin brand image, aligned Collector Playwright dependencies, and added tracked documentation-path checks to CI.
- Kept GitHub Actions and their frontend, collector, backend, contract, architecture, PostgreSQL, Redis, and Admin E2E regression dependencies.
- Replaced the historical PostgreSQL phase wrapper with direct, isolated CI inventory integration commands.
- Adopted GitHub Actions for automated regression and human sign-off for product acceptance.
- Documented that a local test database is optional and is not recreated automatically; CI provisions isolated service containers.

## v0.1.0

- Initial TradeMind monorepo foundation with Go backend, React Admin, Node collector, PostgreSQL, Redis, Docker Compose, Provider abstractions, and open-source governance.
