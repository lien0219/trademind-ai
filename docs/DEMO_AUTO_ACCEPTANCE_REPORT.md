# TradeMind Phase F8.1 Full-Project Demo Auto Acceptance Report

> Generated: 2026-06-30T10:44:45.5867425Z
> API: http://127.0.0.1:8080 | Backend: reachable

## Phase

**Phase F8.1-Auto** - Full-project demo smoke + static scans (not final manual acceptance)

## Summary

| Metric | Value |
| --- | --- |
| Conclusion | **passed** |
| Failed steps | 0 |
| Blocked steps | 0 |

## Step results

| Step | Status | Exit | Detail |
| --- | --- | --- | --- |
| go test regression | passed | 0 |  |
| go build backend | skipped | 0 | -SkipBuild |
| pnpm build:admin | skipped | 0 | -SkipBuild |
| git diff --check | skipped | 0 | -SkipBuild |
| check-ui-copy | passed | 0 |  |
| demo-empty-state-scan | passed | 0 |  |
| demo-sensitive-confirm-scan | passed | 0 |  |
| security-release-check | passed | 0 |  |
| check-doc-links | passed | 0 |  |
| demo-route-smoke | passed | 0 |  |
| seed-demo-data | passed | 0 |  |
| seed-demo-permissions | passed | 0 |  |
| demo-dashboard-smoke | passed | 0 |  |
| demo-rbac-smoke | passed | 0 |  |
| demo-order-inventory-customer-smoke | passed | 0 |  |
| ai-text-route-smoke | passed | 0 |  |
| ai-text-trial-run | passed | 0 |  |
| ai-image-route-smoke | passed | 0 |  |
| ai-image-trial-run | passed | 0 |  |
| publish-batch-perf | passed | 0 |  |
| ai-operation-workbench-perf | passed | 0 |  |

## Artifacts

- [demo-route-smoke.auto.json](demo-route-smoke.auto.json)
- [demo-dataset.auto.json](demo-dataset.auto.json)
- [ai-text-trial-run.auto.json](ai-text-trial-run.auto.json)
- [ai-image-trial-run.auto.json](ai-image-trial-run.auto.json)
- [publish-batch-perf.auto.json](publish-batch-perf.auto.json)
- [ai-operation-workbench-perf.auto.json](ai-operation-workbench-perf.auto.json)
- [COPYWRITING_AUDIT.auto.md](COPYWRITING_AUDIT.auto.md)
- [SECURITY_RELEASE_CHECK.auto.md](SECURITY_RELEASE_CHECK.auto.md)
- [DOCS_CONSISTENCY_CHECK.md](DOCS_CONSISTENCY_CHECK.md)

## Manual test checklist (out of scope for automation)

- [ ] Real preprod SSH deployment
- [ ] Nginx / HTTPS
- [ ] Storage public access
- [ ] Preprod backup and rollback
- [ ] 1366 / 1024 visual walkthrough
- [ ] Douyin real OAuth
- [ ] Douyin readonly E2E
- [ ] Douyin write E2E
- [ ] 48-72h gray observation
- [ ] v0.1.0-demo tag final confirmation

## Final status

```text
MVP Demo Ready
Tag pending
Not Production Ready
Douyin Release Candidate
```

No v0.1.0-demo tag in this phase. No real Douyin E2E. No production gray release.
