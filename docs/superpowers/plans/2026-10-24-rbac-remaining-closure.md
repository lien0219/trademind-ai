# TradeMind RBAC Remaining Closure Implementation Plan

> **For agentic workers:** Execute inline with `superpowers:executing-plans`; work stays on the existing `dev` checkout and preserves all current worktree changes.

**Goal:** Close the remaining server-side authorization, tenant/store isolation, and durable audit gaps found during the final P0–P2 RBAC review, then record verified and blocked acceptance items.

**Architecture:** Load the current principal from the authenticated request and enforce permission checks at every Task Center HTTP handler. Carry tenant and explicit store-scope state into list, detail, and mutation paths so an empty store grant cannot become an unrestricted query. Persist audit evidence in the business transaction when possible; for queue or external side effects, persist a fail-closed request record before starting the side effect.

**Tech Stack:** Go, Gin, GORM, PostgreSQL migrations, React/TypeScript Admin, Playwright, pnpm.

**Spec:** `docs/RBAC_PERMISSION_DESIGN.md`, `docs/SECURITY_PERMISSION_CHECKLIST.md`, and the P0–P2 requirements in the user request.

## Global Constraints

- Stay on `dev`; do not modify `main`, production systems, or production data.
- Preserve all existing staged/unstaged changes and do not weaken permissions, assertions, or CI baselines.
- Use only explicitly guarded PostgreSQL test resources; never fall back to a development database.
- Keep API response envelopes and existing frontend permission semantics unless a tested security correction requires synchronization.
- Mark unavailable local/remote checks `BLOCKED` or `NOT RUN`; never infer a pass from earlier commits.

---

### Task 1: Enforce Task Center permissions on the server

**Files:**
- Create: `backend/internal/modules/taskcenter/handler_authorization_test.go`
- Modify: `backend/internal/modules/taskcenter/handler.go`
- Modify: `backend/internal/modules/taskcenter/router.go` only if middleware placement is needed to cover every route.
- Test: `backend/internal/modules/taskcenter/handler_authorization_test.go`

**Interfaces:**
- Reads use the permission keys already exposed by the Admin menu: `task.retry` for failures/business-task data and `alerts.read` or `task.retry` for alert lists.
- Failure retries and marks require their current task permission; system-alert state changes require the existing `alerts.ack` or `alerts.silence` key that matches the action.
- Missing principal, inactive principal, role lookup failure, or missing database denies the request.

- [ ] Add Gin handler tests showing users without the required permission receive 403 and the handler performs no service-side mutation.
- [ ] Run the new tests and confirm they fail because current handlers do not enforce permissions.
- [ ] Add a shared fail-closed authorization helper and invoke it from every Task Center read/write handler according to the existing permission matrix.
- [ ] Run Task Center tests and confirm permitted admin/operator/readonly/reviewer/custom-role cases retain the documented behavior.

### Task 2: Enforce tenant and explicit store scopes throughout Task Center

**Files:**
- Create: `backend/internal/modules/taskcenter/tenant_scope_test.go`
- Modify: `backend/internal/modules/taskcenter/handler.go`
- Modify: `backend/internal/modules/taskcenter/service.go`
- Modify: `backend/internal/modules/taskcenter/service_list.go`
- Modify: `backend/internal/modules/taskcenter/service_commands.go`
- Modify: `backend/internal/modules/taskcenter/service_alert.go`
- Modify: `backend/internal/modules/taskcenter/service_notify.go`
- Modify: the existing PostgreSQL migration/model pair only if the source-row scope cannot be enforced without schema support.

**Interfaces:**
- Tenant identity and principal lookup errors are returned as authorization failures; tenant ID zero never means an unfiltered query.
- Non-admin principals carry an explicit restricted-store flag separately from the allowed store ID slice, so an empty grant means deny all shop-linked rows.
- List, summary, detail, retry, mark, alert, notification-history, and manual-notification paths use the same tenant/store policy.

- [ ] Add tests for empty store grants, one allowed store, a denied cross-store detail/mutation, cross-tenant alerts, and missing tenant context.
- [ ] Run those tests and confirm the empty-grant and cross-store cases fail on the current behavior.
- [ ] Apply explicit tenant/store filters to the relevant source queries and associated alert/notification queries; reject missing or out-of-scope source rows before mutations.
- [ ] Run the Task Center and IDOR test suites and confirm authorized in-scope results remain available.

### Task 3: Make sensitive Task Center audit evidence fail closed

**Files:**
- Create: `backend/internal/modules/taskcenter/audit_atomicity_test.go`
- Modify: `backend/internal/modules/taskcenter/handler.go`
- Modify: `backend/internal/modules/taskcenter/service_alert.go`
- Modify: `backend/internal/modules/taskcenter/service_commands.go`
- Modify: `backend/internal/modules/taskcenter/service_notify.go`
- Test: `backend/internal/modules/operationlog` only if its transaction helper needs a focused regression.

**Interfaces:**
- Database-backed state changes and their audit records commit or roll back together.
- Retry, alert scan/generate, and manual-notification requests require durable audit persistence before enqueueing or calling a configured notifier.
- Audit dependency absence or persistence failure returns a 5xx response and does not begin the protected action.

- [ ] Add tests that inject an audit persistence failure and assert no protected mutation, queue action, or notification is performed.
- [ ] Verify the tests fail on current ignored `OpLog.Write` errors.
- [ ] Use `WriteTx` with the same GORM transaction for database mutations; persist request evidence before non-transactional side effects and propagate failures.
- [ ] Run the focused taskcenter and operationlog tests, including transaction rollback coverage.

### Task 4: Reconcile RBAC matrix, frontend controls, checklist, and final evidence

**Files:**
- Modify only where findings require it: `admin/src/utils/menuAccess.ts`, Task Center page actions, `docs/RBAC_PERMISSION_DESIGN.md`, `docs/SECURITY_PERMISSION_CHECKLIST.md`, `docs/PROGRESS.md`.
- Test: existing Admin permission/component/E2E suites and backend security tests.

- [ ] Compare every Task Center menu and action with its backend permission gate and add missing frontend disabled/denied behavior without using the UI as authorization.
- [ ] Update the permission coverage matrix, completion checklist, remaining risks, and manual acceptance cases with exact PASS/FAIL/BLOCKED/NOT RUN evidence.
- [ ] Run fresh Go format, focused tests, vet, race, and build; run frontend typecheck/lint/component/build and Admin E2E as the environment permits.
- [ ] Recheck `git status`, `git diff --check`, migration up/down compatibility, and current-head GitHub Actions availability; do not claim remote CI passes unless this exact commit has a completed run.
