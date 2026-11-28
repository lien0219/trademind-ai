# Collector Provider Contract and Regression Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Establish trustworthy provider catalogue parity and deterministic parser regression coverage as the first independently verifiable step toward per-source Collector production acceptance.

**Architecture:** Keep platform parsing in existing provider modules. A small shared contract fixture under `tests/contracts` is used only by tests to compare Collector metadata with the Go fallback shown when Collector is unavailable. Provider parser tests call existing pure assembly functions with inline fixture payloads; no network, browser-profile, provider-write, or product-import behavior is changed. Correct proven provider-catalogue and Pinduoduo supported-scope drift; reject flagged AliExpress verification pages even when they expose a non-empty document title.

**Tech Stack:** Node.js, TypeScript, Vitest, React Testing Library, Playwright Test, Go standard testing, shared JSON contract fixture.

**Spec:** `docs/superpowers/specs/2026-11-28-collector-production-hardening.md`

## Global Constraints

- Preserve the existing API envelope, normalized product payload, task state machine, permissions, import behavior, and read-only platform interaction.
- Anonymous mode is public-page-only. Do not bypass login, CAPTCHA, anti-bot or other access controls; do not invent values for missing fields.
- Do not add third-party dependencies, skip/ignore rules, new baseline exceptions, or one-off acceptance/report artifacts.
- Deterministic parser fixtures are not live-platform accuracy or production-readiness evidence.
- Keep changes on `feat/collector-production-hardening`, created from `dev`; do not commit without explicit user authorization, and do not push, merge, deploy, or contact real shops/platform accounts.
- Update `docs/provider.md`, `docs/module-map.md` if boundaries/contracts change, and `docs/PROGRESS.md` for the maintenance milestone.
- Follow `docs/task-checklist.md`; unrun full automation remains for CI unless the focused test is needed to demonstrate a TDD cycle.

---

### Task 1: Lock provider metadata and fallback parity

**Files:**
- Create: `tests/contracts/collector-providers.json`
- Create: `collector/src/providers/__tests__/registry.contract.test.ts`
- Create: `backend/internal/modules/collect/providers_contract_test.go`
- Modify: `backend/internal/modules/collect/providers.go`

**Interfaces:**
- Contract source fields: `source`, `name`, `description`, `status`, `batchSupported`, `urlPatterns`, `features`, `notes`.
- Collector test consumes `listProviderPublicMetas()`.
- Go test consumes `defaultCollectProvidersFallback()`.

- [x] Write tests asserting both runtime metadata sources equal the canonical shared fixture, preserving canonical provider order and field values.
- [x] Confirm the Go contract test fails because fallback metadata advertises an unsupported Pinduoduo consumer URL, omits AliExpress limitations, uses different SHEIN/Temu URL patterns, and serializes planned features as `null` instead of the Collector's empty list.
- [x] Align all four fallback metadata differences with the Collector registry; do not expand any Collector adapter.
- [x] Run the focused Vitest and Go contract tests and confirm both pass.


### Task 2: Prevent unsupported Pinduoduo URLs from being submitted

**Files:**
- Modify: `admin/src/utils/pinduoduoUrl.ts`
- Test: `admin/src/utils/__tests__/pinduoduoUrl.test.ts`
- Modify: `admin/src/pages/Collect/components/PinduoduoCollectModal.tsx`
- Modify: `admin/src/pages/Collect/Tasks/index.tsx`
- Modify: `admin/src/i18n/sharedGapTranslations.ts`
- Test: `admin/src/constants/__tests__/localizedCopy.test.tsx`
- Modify: `collector/src/providers/sourcePinduoduo/url-type.ts` and `collector/src/providers/sourcePinduoduo/access-detect.ts`
- Modify: `backend/internal/modules/collect/batch_stats.go`, `backend/internal/modules/taskcenter/failureclassifier/failureclassifier.go`, and `backend/internal/modules/taskcenter/collect_extra.go`
- Test: `admin/e2e/specs/collect-hub-responsive.spec.ts`
- Test: `collector/src/providers/sourcePinduoduo/__tests__/url-type.test.ts`
- Test: `backend/internal/modules/collect/batch_stats_test.go`
- Test: `backend/internal/modules/taskcenter/collect_extra_test.go`
- Test: `backend/internal/modules/taskcenter/failureclassifier/failureclassifier_test.go`

**Interfaces:**
- Add pure helper `isPinduoduoCollectSupportedUrl(url: string): boolean`; it is true only for `wholesale_detail` URLs currently supported by the Collector.
- Both Admin single-collect surfaces use the helper to disable/guard submission.

- [x] Write a utility regression proving a Pifa `goods/detail?gid=` URL is supported while a `mobile.yangkeduo.com/goods.html?goods_id=` URL is unsupported.
- [x] Run the focused unit test and confirm it fails before adding the helper.
- [x] Add the helper and gate both the modal and task form; unsupported links show the existing actionable hint and produce no task request.
- [x] Replace the mobile URL example with the localized Pifa pattern already used in the provider contract; remove or correct contradictory introductory copy.
- [x] Align Collector, batch failure, and task-center suggestions so login/unsupported-URL errors recommend only Pifa wholesale detail URLs, not unsupported ordinary consumer detail URLs.
- [x] Extend the existing collector-hub E2E Mock to include Pinduoduo, then verify the modal disables unsupported mobile URLs, enables Pifa detail URLs, and sends zero writes unless explicitly submitted.
- [x] Run the focused Admin unit test and static checks; the persistent browser regression remains covered by Admin E2E CI.

### Task 3: Cover known 1688 parser regressions

**Files:**
- Create: `collector/src/providers/source1688/__tests__/parser.test.ts`
- Read before modifying: `docs/collector-1688-pitfalls.md`
- Modify production parser only if a new assertion demonstrates a defect.

**Interfaces:**
- Tests consume `assembleParsedProduct(inputUrl, BrowserExtractPayload)`.

- [x] Add an inline payload with `unitWeight` but no price-keyed value; assert no price is inferred from weight.
- [x] Add a representative multi-color/mm price-table payload; assert SKU dimensions are separated from price/stock text and prices remain attached to the matching rows.
- [x] Add a mixed complete/partial payload assertion for missing-field and warning behavior.
- [x] Run `pnpm --filter @trademind/collector exec vitest run --config vitest.config.ts src/providers/source1688/__tests__/parser.test.ts`.

### Task 4: Cover existing Pinduoduo wholesale and access-result parsing

**Files:**
- Create: `collector/src/providers/sourcePinduoduo/__tests__/wholesale-detail.test.ts`
- Modify production parsing only when tests expose a reproducible defect.

**Interfaces:**
- Tests consume `assemblePifaWholesaleProduct` and `validateWholesaleCollectQuality`.

- [x] Add a representative Pifa payload covering title cleanup, price range, SKU rows, image-region classification, and attributes.
- [x] Add an incomplete-page fixture verifying warnings and partial-result quality disposition; page-access/CAPTCHA classification remains in the separate access detector and is never bypassed.
- [x] Run the focused Pinduoduo tests.

### Task 5: Cover existing Taobao/Tmall and AliExpress assembly behavior

**Files:**
- Create: `collector/src/providers/sourceTaobaoTmall/__tests__/parser.test.ts`
- Create: `collector/src/providers/sourceAliExpress/__tests__/parser.test.ts`
- Create: `collector/src/providers/sourceAliExpress/__tests__/provider-access.test.ts`
- Modify production parsing only when tests expose a reproducible defect.

**Interfaces:**
- Taobao tests consume the pure `resolvePriceFromPayload`, `mergeTaobaoPayload`, and `validateTaobaoCollectQuality` functions with deterministic payloads.
- AliExpress tests consume `assembleAeProduct` with a deterministic `AeBrowserPayload`.

- [x] Cover structured price and SKU mapping, product image separation/deduplication, and meaningful missing-field warnings for Taobao/Tmall.
- [x] Cover AliExpress structured title/image/attribute/SKU extraction and blocked/empty-title classification.
- [x] Verify the Provider rejects a flagged verification page even when its document title is non-empty.
- [x] Confirm test payloads are explicitly labeled parser fixtures, not captured live snapshots.
- [x] Run the focused Taobao/Tmall and AliExpress parser/access tests.

### Task 6: Document scope and verification boundaries

**Files:**
- Modify: `docs/provider.md`
- Modify: `docs/module-map.md` only if contract ownership or test boundaries are newly introduced.
- Modify: `docs/PROGRESS.md`
- Review: `docs/task-checklist.md`

- [x] Document the six registered sources, the Pinduoduo Pifa-only boundary, SHEIN/Temu planned status, and the distinction between parser fixtures and live acceptance.
- [x] Document that live product-page acceptance is isolated-preproduction-only; production collector scope is health/readiness probes, not task creation or sample collection.
- [x] Record this completed local milestone without claiming all sources production-ready.
- [x] Run `git diff --check`, Collector focused tests, `pnpm build:collector`, `pnpm build:admin`, affected Go unit packages, `pnpm test:contracts`, `pnpm check:ui-copy --strict`, `pnpm quality:baseline:admin-ts`, `pnpm quality:sensitive`, `pnpm architecture:check`, and Playwright E2E test discovery. Browser E2E remains for CI.
- [x] Report which automated suites remain delegated to CI and which per-source controlled manual checks remain outstanding.

## Follow-on plan boundary

This plan does not implement every platform or claim production acceptance. After this baseline lands, continue with separate, testable per-source hardening plans, beginning with 1688 and then Pinduoduo, Taobao/Tmall, AliExpress, custom rules, and separate SHEIN/Temu feasibility/adapter work. Any unified public result-quality/provenance payload requires a separate API/DTO design and contract review.
