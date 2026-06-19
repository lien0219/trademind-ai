# 多平台刊登设计（Phase A2）

> 单商品多平台 / 多店铺刊登中心（A1.2）+ **多商品批量创建刊登草稿**（A2）。

## 平台能力分级

| 内部能力码 | 运营可见文案 | 行为 |
| --- | --- | --- |
| `real_draft_create` | 可创建平台草稿 | 调用平台真实写接口创建草稿（当前仅 **抖店**） |
| `local_draft_only` | 仅生成本地草稿 | 生成本地 `product_publications` + 任务快照，**不调用**外部平台 API |
| `not_configured` | 尚未配置 | 平台开放配置或刊登预设未完成 |
| `not_authorized` | 店铺未授权 | 需先在店铺管理完成 OAuth |
| `disabled` | 已停用 | Provider 或能力已关闭 |

能力来源：`GET /api/v1/platform/providers` 注册表 + 店铺表 + 平台开放配置完整性；**不在页面写死平台列表**。

## 单商品多目标刊登（A1.2）

1. **刊登目标**：按平台展示已授权店铺，支持多选。
2. **统一配置**：标题 / 描述 / 价格 / 图片 / 包裹 / 库存策略（接口预留 `commonConfig`）。
3. **单独配置**：各平台 Tab 内覆盖（抖店类目属性、图片同步等）；**单独配置优先生效**。

### 单商品 API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/products/:id/publish-targets` | 可刊登平台与店铺及能力分级 |
| POST | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查（不写库、不调平台写接口） |
| POST | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建刊登草稿；每目标一子任务，汇总为 `product_publish_batches`（`batch_type=single_product`） |

## 多商品批量刊登（Phase A2）

### 场景

```text
多个商品 → 单平台单店铺
多个商品 → 单平台多店铺
多个商品 → 多平台多店铺
```

### 运营入口

- 商品草稿列表：多选 → **批量创建刊登草稿**
- 向导页：`/product/publish-batch?productIds=...`（5 步）
- 批次列表：商品 → 刊登任务 → **刊登批次** Tab
- 批次详情：`/product/publish-batches/:id`

### 批量 API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/product-publish/targets` | 全局平台 / 店铺能力（向导第 2 步） |
| POST | `/api/v1/product-publish/batch-targets/check` | 多商品 × 多目标矩阵预检查 |
| POST | `/api/v1/product-publish/batch-targets/create-drafts` | 创建多商品批次与子任务 |
| GET | `/api/v1/product-publish/batches` | 批次列表 |
| GET | `/api/v1/product-publish/batches/:id` | 批次详情 + 子任务 |
| POST | `/api/v1/product-publish/batches/:id/retry-failed` | 只重试失败子任务 |
| POST | `/api/v1/product-publish/batches/:id/cancel-pending` | 只取消 pending 子任务 |

### 检查响应摘要

- `ready` → 可以创建草稿
- `warning` → 建议检查（可继续，需人工确认）
- `blocked` → 暂不能创建草稿

每个 **商品 × 目标** 的 `issues[]` 含中文化 `title` / `message`；内部码在 `technicalDetails.rawCode`。

创建参数：

- `onlyReady=true`：只创建 ready 项
- `includeWarnings=false`：跳过 warning 项
- `blocked` 项默认跳过，不可强行提交

### 批次与子任务模型

- `product_publish_batches`：`batch_type=multi_product` 时 `product_id` 可为空；保存 `product_count`、`target_count`、`task_count`、配置快照 `input`
- `product_publish_tasks.batch_id` + `target_key`：每个商品 × 每个目标一条子任务
- 子任务 `input` 保存 `effectiveConfig` + `configSources` 快照

批次状态：`pending` / `running` / `partial_success` / `success` / `failed` / `cancelled`

### 配置优先级

```text
系统默认 → 平台默认 → 店铺默认 → 批量统一配置 → 商品覆盖 → 平台覆盖 → 店铺覆盖 → 商品+平台+店铺覆盖
```

MVP 统一字段：`priceRule`、`imageStrategy`、`stockStrategy`、`packageWeight`、`packageSize`、`remark`

### 幂等

- 批次：`publish-batch:{userId}:{productIdsHash}:{targetsHash}:{configHash}`
- 子任务：同一商品 + 店铺 + 平台已有成功抖店 / 本地草稿时不重复创建

### 操作日志

- `product.publish.batch.check`
- `product.publish.batch.create`
- `product.publish.batch.retry_failed`
- `product.publish.batch.cancel_pending`

失败任务中心：子任务 `batch_id` 存在时，详情链接跳转批次详情页。

## 与直接上架的边界

- 本阶段名称准确为 **批量创建刊登草稿**，不是「一键发布 / 直接上架」。
- 未接入真实 Provider 的平台**不得**伪装为已发布成功。
- 抖店仍为 **Release Candidate**；OpenAPI 字段未改；复用现有 `create-draft` 链路。

## Phase A2 实施边界（禁止）

- 自动直接上架
- 新增真实平台 OpenAPI
- 修改抖店 OpenAPI 字段
- 一个子任务失败导致整个批次回滚
- 把 `local_draft_only` 伪装成真实平台发布成功

## 下一阶段（不在 A2）

- 统一配置 UI 完整表单（标题 / 描述策略等）
- 各跨境平台真实 `ProductPublishProvider` 草稿创建升级
- 批次异步队列化（当前同步创建子任务，抖店异步 worker 照旧）

## Phase A2.1 验收与生产安全收口

### 批量规模上限

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `PUBLISH_BATCH_MAX_PRODUCTS` | 100 | 单批最多商品数 |
| `PUBLISH_BATCH_MAX_TARGETS` | 20 | 单批最多刊登目标数 |
| `PUBLISH_BATCH_MAX_TASKS` | 300 | 商品数 × 目标数上限 |

超限 HTTP 400：`本次选择的商品和刊登目标较多，请分批创建刊登草稿。`

### 数据库 migration

显式 Postgres migration：[`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)（`product_id` 可空、查询索引、活跃批次 `idempotency_key` 部分唯一索引）。

### 执行策略（本阶段结论）

- **保持** create-drafts 同步 orchestration；单批 ≤300 子任务。
- `local_draft_only`：同步 DB，预计可接受。
- 抖店：子任务 pending + Redis worker；生产应保持 `PRODUCT_PUBLISH_QUEUE_ENABLED=true`。
- 未引入独立批次 worker 队列。

### 测试与脚本

- 集成测试：`backend/internal/modules/productpublish/batch_targets_integration_test.go`
- 性能脚本：`scripts/publish-batch-perf.ps1` → [`PUBLISH_BATCH_PERF_REPORT.md`](PUBLISH_BATCH_PERF_REPORT.md)
- UX 验收：[`PUBLISH_BATCH_UX_ACCEPTANCE.md`](PUBLISH_BATCH_UX_ACCEPTANCE.md)
