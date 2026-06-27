# 批量刊登性能报告（Phase A2.1 / R1）

## 测试方法

```powershell
# 需本地 API + 足够 draft 商品；无 local_draft_only 店铺时脚本自动创建 demo 店铺
.\scripts\publish-batch-perf.ps1 -ApiBase http://127.0.0.1:8080 -OutFile docs/publish-batch-perf.json
```

脚本对三档规模调用：

- `POST /api/v1/product-publish/batch-targets/check`
- `POST /api/v1/product-publish/batch-targets/create-drafts`
- `GET /api/v1/product-publish/batches/:id`

目标平台均为 **`local_draft_only`**（TikTok / Shopee / Lazada / Amazon demo 店铺）。

## 执行策略结论

| 项 | 结论 |
|----|------|
| 单批上限 | 300 子任务（`PUBLISH_BATCH_MAX_TASKS`） |
| create-drafts | **同步** orchestration |
| local_draft_only | 同步 DB 写入，不调外部 OpenAPI |
| 抖店 real_draft_create | 子任务 pending + Redis worker（本脚本未测） |

## R1 基准结果（2026-06-27）

> 样本商品多数发布检查为 blocked，子任务记为 skipped；本表衡量 **API 编排耗时**，非业务 success 率。

| 规模 | check (ms) | create-drafts (ms) | batch detail (ms) | 子任务数 | 外部 API | AI 任务 |
|------|------------|--------------------|--------------------|----------|----------|---------|
| 20×2 | 14.8 | 20.4 | 2.4 | 40 | 否 | 否 |
| 50×2 | 30.4 | 33.3 | 1.7 | 100 | 否 | 否 |
| 100×3 | 134.4 | 91.9 | 1.6 | 300 | 否 | 否 |

原始 JSON：`docs/publish-batch-perf.json`

## SQL 查询数

MVP **未** 内置 HTTP 级 SQL 计数。索引见 [`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)。

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-06-19 | Phase A2.1 初版 |
| 2026-06-27 | Phase R1：实测 20×2 / 50×2 / 100×3 + demo 店铺自动创建 |
