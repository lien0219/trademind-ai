# Collector Production Hardening — Scope and Acceptance Design

**Date:** 2026-11-28
**Status:** Approved to begin implementation; production acceptance remains pending
**Baseline:** `dev` at `e08bd8fd82af`
**Related evidence:** `collector-readiness-gap-matrix.canvas.tsx` (local review artifact; not a production test report)

## Goal

Improve all registered Collector sources toward production readiness through truthful provider contracts, repeatable parser regressions, explicit anonymous-access boundaries, and per-source acceptance. Do not claim production readiness until automated CI and controlled preproduction/manual validation pass for each source.

## Current scope

Registered sources are `1688`, `pinduoduo` (Pifa wholesale URLs only), `taobao_tmall`, `aliexpress`, `shein_temu` (planned placeholder), and `custom`.

This first implementation slice will:

1. Establish one CI contract fixture for the public provider catalogue and assert that the Collector registry and Go offline fallback match it.
2. Correct only proven metadata drift without broadening platform URL support or changing collect/import business semantics.
3. Align all Admin single-collect entry points and Pinduoduo task/login suggestions with the actual Pifa-only Provider; unsupported mobile consumer URLs must not trigger task-creation requests or be recommended as recovery actions.
4. Add deterministic parser regression tests for implemented providers using inline, representative parser payloads. Such payloads prove parser behavior only; they are not evidence that the live platform currently exposes the same structure.
5. Keep SHEIN and Temu explicitly unavailable until separately assessed and implemented.
6. Document which checks remain CI- or controlled-sample-only.

## Security and product boundaries

- Anonymous collection means reading only product data publicly exposed to a clean unauthenticated browser session.
- Do not bypass or automate CAPTCHA, anti-bot challenges, access controls, or login requirements. Surface blocked/login/verification outcomes and missing fields accurately.
- Never fabricate missing price, inventory, variant, image, or attribute values to make a result appear complete.
- Keep platform-specific parsing within its Provider. Shared logic must remain narrow and demonstrably reusable.
- Preserve the existing Go import pipeline, task states, API envelopes, permission checks, and read-only external collection behavior unless separately designed and accepted.
- Do not use credentials, real shops, production data, or external platform write requests for local tests. Live product-page acceptance runs only against isolated preproduction services and volumes; production is limited to approved health/readiness probes, never product task creation or sample collection.

## Production acceptance model

Each source must be accepted independently. Evidence includes:

- supported URL and field scope accurately represented in API/UI metadata;
- deterministic parser/access/quality regressions included in existing CI;
- controlled, revocable sample URLs compared against the visible source page for the business-required fields;
- separate anonymous and authenticated-profile results, with expired-session and challenge cases classified safely;
- partial-result, timeout, retry/throttle, and recovery behavior checked where applicable;
- exact revision CI plus manual preproduction sign-off.

Synthetic or sanitized parser fixtures do not satisfy the controlled live-sample requirement. Accuracy percentages and service-level thresholds must be agreed against the required fields and representative sample set rather than invented from fixture tests.

## Follow-on subprojects

- Harden 1688 parsing against price/SKU/image drift and verify its anonymous clean-profile behavior.
- Validate Pinduoduo wholesale anonymous and optional profile collection; do not imply support for general Pinduoduo consumer URLs unless actually implemented.
- Validate Taobao/Tmall price, SKU, image, login and partial-result behavior.
- Validate AliExpress price support, product variants/images and anonymous access; retain beta status until accepted.
- Assess SHEIN and Temu separately; implement only public, permitted product-page extraction and keep access challenges fail-closed.
- Make custom-rule quality evidence and status precise without promising general site coverage.
- Revisit unified field-level quality/provenance data only through a separate API/DTO design; this slice does not change the product payload contract.
