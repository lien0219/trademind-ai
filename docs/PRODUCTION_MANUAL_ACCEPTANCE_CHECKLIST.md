# P10 Manual Acceptance Checklist

Status: **Feature Manual Acceptance Confirmed / Deployment Acceptance Pending**

Sections A-G validate repository-side P10 development without contacting Douyin or promoting runtime beyond L0. Section H is a separate, externally authorized L3 acceptance for the only supported real mutation: saving one reviewed product as a Douyin platform draft. Record the operator, date, environment, source HEAD, tenant, allowlisted shop, observed result and redacted evidence in the PR or release work order. Do not commit a completed checklist, report, screenshot or log artifact to the repository. Do not record credential values, OAuth state values, Authorization/Cookie headers, database/Redis URLs or raw Provider responses.

Automated regression is owned by GitHub Actions and does not complete this manual checklist, real PostgreSQL/runtime acceptance, performance acceptance or real-platform verification. The maintainer confirmed feature-level manual acceptance on 2026-08-21; the repository checklist below still tracks deployment, external infrastructure, credential and real-platform evidence, which must not be inferred from that feature sign-off.

## A. Credential

- [ ] Create a development-only offline credential for an authorized fixture shop; confirm the response contains metadata only.
- [ ] List credentials as the same tenant and verify credential ID, platform, shop, status, expiry, algorithm and revision.
- [ ] Attempt cross-tenant lookup/use and confirm denial without existence leakage.
- [ ] Rotate with the current revision; confirm a new active encrypted version and retired prior version.
- [ ] Retry rotation with a stale revision and confirm conflict.
- [ ] Revoke locally; confirm `status=revoked` and all later decrypt/use attempts are denied.
- [ ] Advance/use an expired fixture credential and confirm `status=expired` or credential-use denial.
- [ ] Inspect Admin, API, logs and audit evidence for absence of Token, ciphertext and secrets.

## B. Offline OAuth Foundation

- [ ] Start authorization using an exact allowlisted redirect and confirm `networkRequestExecuted=false`.
- [ ] Confirm the persisted state is random, hashed, expiring and bound to tenant, user, platform, shop and redirect.
- [ ] Complete the fixture callback once and confirm safe credential metadata creation.
- [ ] Replay the same state and confirm rejection.
- [ ] Use an expired state and confirm rejection.
- [ ] Use the state from another tenant/user and confirm rejection.
- [ ] Use a redirect not in the exact allowlist and confirm rejection.
- [ ] Confirm `oauth_authorization_started`, `oauth_callback_received` and credential lifecycle audit events contain no secrets.

## C. Provider

- [ ] Confirm `DouyinReadOnlyInventoryProvider` conforms to the exported P9 `InventoryProvider` interface.
- [ ] Confirm local publication pagination uses only the repository-confirmed `product.detail` operation.
- [ ] At L0, confirm no real Provider read/write route can reach Douyin. Separately review that L3 exposes only the operation-task platform-draft path.
- [ ] Review bounded connection/request/header timeouts, connection pool, page limit, <=100 SKU limit and response-size limit.
- [ ] Exercise offline/fake mappings for unauthorized, expired, rate-limited, unavailable, invalid-request and protocol errors.
- [ ] Confirm 429 exposes safe rate-limit state and `Retry-After` metadata without automatic business retry.
- [ ] Confirm internal and Provider request IDs correlate safe status/snapshot/audit evidence.

## D. Inventory Sync

- [ ] From an authorized tenant/shop fixture, manually trigger one read-only run with a unique `Idempotency-Key`.
- [ ] Confirm immutable snapshots, normalization and <=100 total SKU enforcement.
- [ ] Confirm existing P9 SKU binding calibration and manual-binding fallback are reused.
- [ ] Confirm run history, snapshot counts, calibration counts, manual backlog and audit correlation.
- [ ] Repeat the same idempotency key/payload and confirm the original result is returned.
- [ ] Repeat the key with a different payload and confirm conflict.
- [ ] Manually rerun only a failed/cancelled tenant-scoped source revision; confirm no automatic retry.

## E. Security

- [ ] Confirm tenant/store authorization on credential, control and read-run routes.
- [ ] Confirm strict JSON rejects unknown fields, multiple values and oversized bodies.
- [ ] Confirm Provider base URL is trusted config, official HTTPS host only, with no userinfo/query/fragment or non-443 port.
- [ ] Confirm OAuth replay, redirect allowlist and state binding fail closed.
- [ ] Confirm redaction covers Authorization, Cookie, access/refresh token, app/client secret, database URL and Redis URL keys.
- [ ] Confirm revoked/expired credentials cannot decrypt or invoke a Provider.
- [ ] Confirm kill switches override feature flags and all real calls remain denied at L0.
- [ ] Confirm no P10 inventory write or automatic business retry exists. At L0, production Worker/outbox dispatch remains disabled.

## F. Gray

- [ ] Configure at most one tenant, one shop and `maxSku<=100`; confirm overflow is rejected.
- [ ] Confirm Draft, PendingApproval, Approved, Active, Paused and Stopped are represented by the model.
- [ ] Confirm Approved/Active evaluation requires both Owner and Technical Lead approval.
- [ ] Confirm Owner and Technical Lead duties are performed by two different active administrators; verify their organizational responsibilities in the release work order because the application role model only proves distinct administrator identities.
- [ ] Confirm the system exposes no API that automatically grants either human approval.
- [ ] Confirm Pause and Stop block Provider read through the kill-switch/Gray guard.
- [ ] Confirm runtime remains L0 and no real Gray begins during this checklist.

## G. Recovery and Operations

- [ ] Review `backup-preproduction.sh` artifact, checksum, metadata, retention configuration and environment guard.
- [ ] Review `restore-preproduction.sh` explicit isolated target identity and production-restore denial.
- [ ] Review `rollback-preproduction.sh` previous immutable image selection, readiness check and non-implicit database restore.
- [ ] Execute timed backup/restore/rollback and five kill-switch drills only after independent pre-production exists.
- [ ] Record measured RPO/RTO, alert fire/recovery and dedicated-host performance evidence; do not infer these from code/build checks.

## H. Controlled Douyin Platform Draft Write

Run only on an independent pre-production or explicitly approved production target with a revocable test/allowlisted shop. L3 permits `save_as_platform_draft` only. It does not permit online publication, listing, inventory mutation, automatic business retry, unreviewed execution or multi-shop rollout.

- [ ] Confirm source HEAD/image, CI checks, migration backup, isolated restore, application rollback and rollback ownership are recorded in the release work order.
- [ ] Confirm the database contains at most one enabled tenant/shop allowlist and the selected shop is an active, authorized `douyin_shop` owned by that tenant.
- [ ] Confirm one active gray policy for the same shop with `maxSku<=100`, approved by two different administrators acting as Owner and Technical Lead.
- [ ] Confirm `P10_CURRENT_ALLOWED_LEVEL=L3`, real Provider/network/credential/product-draft-write/Worker flags are true, `PRODUCT_PUBLISH_QUEUE_ENABLED=true`, and `WORKER_REAPER_ENABLED=true`.
- [ ] Confirm `P10_AUTOMATIC_RETRY_ENABLED=false`, `P10_INVENTORY_MUTATION_ENABLED=false`, and no online-publish setting is enabled.
- [ ] Keep provider, tenant, shop and write kill switches active until the final go/no-go; release only the approved scope and leave the read switch independent.
- [ ] Create the task from the operation-task center for one eligible product. Confirm product/shop/SKU/mapping/request snapshots are frozen and no platform request occurs before approval.
- [ ] Approve the exact draft version and payload hash, then execute once. Confirm a duplicate request/idempotency key does not create another downstream task or platform draft.
- [ ] Confirm the Worker revalidates task/draft/approval/attempt/downstream bindings and runtime controls before obtaining the Provider client.
- [ ] Confirm platform success produces one `product_publish_task`, one publication and expected SKU rows, then converges the operation task to `draft_written` without publishing online.
- [ ] Simulate queue unavailability and process interruption in pre-production. Confirm the transactional outbox re-delivers infrastructure work without recreating a completed platform draft and the task reaper converts an expired in-flight write to `result_unknown`.
- [ ] For `result_unknown`, confirm all legacy retry controls remain disabled. Use only manual read-only `product.detail` reconciliation with `operationtask.execute` and store-operation permission; queued, running and known-failed states must return 409 without mutation. An existing draft converges to success, while no match remains non-retryable and requires human investigation.
- [ ] Activate each provider/tenant/shop/write kill switch during a controlled drill and confirm platform access is blocked before the Provider call. Record alert/health behavior and recovery steps.
- [ ] Confirm the legacy direct create endpoint returns HTTP 409 with `DOUYIN_OPERATION_TASK_REQUIRED`; traditional publish, multi-target, batch, task retry and batch retry paths produce zero Douyin writes and zero local state changes.
- [ ] Inspect logs, audit events and Admin responses for absence of credentials, frozen Provider request secrets and raw platform payloads.
- [ ] After the observation window, pause/stop gray or re-engage kill switches according to the release decision. Do not expand tenant/shop/SKU scope without a new approval cycle.

## Real Platform Acceptance

Status: **blocked_by_external_infrastructure_and_credentials**

The following remain separate and must not be attempted until an independent pre-production server, managed key source, Douyin app credential, revocable allowlisted shop, approved release work order and Section H controls are available:

- [ ] Real OAuth authorization and callback.
- [ ] Real credential rotation/revocation drill.
- [ ] Real `product.detail` read, pagination, timeout and rate-limit validation.
- [ ] Real read integration, snapshot/calibration/manual rerun evidence.
- [ ] G0/G1 read-only Gray with Owner + Technical Lead approval.
- [ ] One L3 `save_as_platform_draft` write using the operation-task-only path and the full Section H recovery/kill-switch drill.
- [ ] Dedicated-host performance, soak, alerts, RPO/RTO and final Production Acceptance.

Production inventory write, online publication, automatic business retry and multi-shop rollout are not part of this checklist and remain unapproved. Until every applicable CI, manual, backup, gray, rollback, credential and release-work-order item is signed, the correct status is **code ready for controlled acceptance, not live**.
