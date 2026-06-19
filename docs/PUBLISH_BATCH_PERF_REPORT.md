# 批量刊登性能报告（Phase A2.1）

## 测试方法

```powershell
# 需本地 API + 足够 draft 商品与 local_draft_only 店铺
$env:ADMIN_BOOTSTRAP_EMAIL = "admin@example.com"
$env:ADMIN_BOOTSTRAP_PASSWORD = "your-password"
.\scripts\publish-batch-perf.ps1 -ApiBase http://127.0.0.1:8080 -OutFile docs/publish-batch-perf.json
```

脚本对三档规模调用：

- `POST /api/v1/product-publish/batch-targets/check`
- `POST /api/v1/product-publish/batch-targets/create-drafts`
- `GET /api/v1/product-publish/batches/:id`

目标平台均为 **`local_draft_only`**（不调用外部 OpenAPI，不创建 AI 任务）。

## 执行策略结论

| 项 | 结论 |
|----|------|
| 单批上限 | 300 子任务（100 商品 × 20 目标矩阵上限，实际以 `PUBLISH_BATCH_MAX_TASKS` 为准） |
| create-drafts | **同步** orchestration（循环创建子任务后返回） |
| local_draft_only | 同步 DB 写入，100×3 预计 <30s（取决于 DB 与商品体量） |
| 抖店 real_draft_create | 子任务 pending + Redis worker，HTTP 不等待 OpenAPI |
| 风险 | `PRODUCT_PUBLISH_QUEUE_ENABLED=false` 时抖店 inline 可能拖长请求；建议生产保持队列开启 |
| 后续 | 单批 >300 或混合大量抖店 inline 时再考虑批次 worker（本阶段未做） |

## SQL 查询数

MVP **未** 内置 HTTP 级 SQL 计数。批次列表/详情依赖索引见 [`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)。可在 Postgres 上对典型 SQL 执行 `EXPLAIN ANALYZE` 抽检。

## 基准结果（集成测试环境参考）

在 `go test` 集成测试（sqlite 内存库、1 商品 × 1 目标）中，单次 create-drafts 约 **10–20ms**。

**生产基准**：请在有 ≥100 个 draft 商品与 ≥3 个 `local_draft_only` 店铺的环境运行 `publish-batch-perf.ps1`，将 `docs/publish-batch-perf.json` 结果填入下表：

| 规模 | check (ms) | create-drafts (ms) | batch detail (ms) | 子任务数 | ready | warning | blocked | 外部 API | AI 任务 |
|------|------------|--------------------|--------------------|----------|-------|---------|---------|----------|---------|
| 20×2 | _待运行_ | _待运行_ | _待运行_ | 40 | — | — | — | 否 | 否 |
| 50×2 | _待运行_ | _待运行_ | _待运行_ | 100 | — | — | — | 否 | 否 |
| 100×3 | _待运行_ | _待运行_ | _待运行_ | 300 | — | — | — | 否 | 否 |

> CI / 无本地 API 时以上表保留「待运行」；验收以脚本 JSON 为准。

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-06-19 | Phase A2.1 初版：脚本 + 执行策略 + 集成测试参考耗时 |
