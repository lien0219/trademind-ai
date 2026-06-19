# Phase A3.1 批量 AI 文案操作设计

> 日期：2026-06-19  
> 阶段：AI 商品运营体验 Phase A3.1

## 目标

```text
多商品批量 AI 标题 / 描述生成
→ 批量复核
→ 单条编辑
→ 批量应用
→ 冲突保护
→ 安全撤销
```

## 任务模型

新增表（与现有 `ai_tasks` / `ai_operation_batches` 并存，不割裂）：

| 表 | 用途 |
| --- | --- |
| `ai_product_text_batches` | 批量任务父级：批次号、状态、幂等键、统计 |
| `ai_product_text_items` | 子项：商品 × 内容类型（title / description） |

每个子项生成时调用现有 `product.OptimizeTitleWithBatch` / `GenerateDescriptionWithBatch`，**`SaveAIField=false`**，结果写入 `ai_tasks` 并关联 `ai_task_id`。

子项状态：`pending` → `running` → `pending_review` / `failed` → `applied` / `rejected` / `conflict` / `cancelled`。

## API（`/api/v1/products/ai-text/...`）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/batches/check` | 创建前检查 |
| POST | `/batches` | 创建批次（幂等） |
| GET | `/batches` | 列表 |
| GET | `/batches/:id` | 详情 + 复核项 |
| POST | `/batches/:id/retry-failed` | 只重试失败项 |
| POST | `/batches/:id/cancel-pending` | 取消 pending |
| POST | `/batches/:id/apply-selected` | 批量应用（冲突保护） |
| POST | `/batches/:id/undo-applied` | 批量撤销本批次应用 |
| POST | `/items/:id/regenerate` | 单条重新生成 |
| POST | `/items/:id/update-edited-text` | 保存编辑文案 |
| POST | `/items/:id/apply` | 单条应用 |
| POST | `/items/:id/reject` | 放弃建议 |

## 应用与冲突

- 单条 / 批量应用均复用 `product.ApplyAITitle` / `ApplyAIDescription` → `applyAIContent`。
- 校验 `expectedUpdatedAt` + `sourceSnapshotHash`；冲突标记 `conflict`，不阻断其他子项。
- 应用记录写入 `product_ai_content_applications`。

## 撤销

- 单条：商品详情现有 `undo-ai-title` / `undo-ai-description`。
- 批量：`POST .../undo-applied` 只撤销本批次 `application_id` 关联项。

## 质量 Warning

生成成功后运行 `quality.go` 规则，中文 warning 展示于复核台，不阻断应用。

## 与刊登配置

Phase A3.1 **不改造** A2.2 `commonConfig` / `overrides`。应用后的 `ai_title` / `ai_description` 仍作为后续刊登草稿默认商品内容。

## 本阶段不做

- 批量图片 AI
- 自动直接上架
- 新增真实平台 OpenAPI
- 每个平台独立标题策略（留 A3.2+）

## 前端入口

- 商品草稿列表 → **批量 AI 优化** → `/product/ai-text-batch`
- AI 工具 → **批量文案任务** → `/ai/text-batches`
- 复核工作台 → `/product/ai-text-batches/:id`

## Phase A3.1.1 补充（2026-06-19）

### 失败任务中心

- `taskType=ai_text`，`sourceTable=ai_product_text_items`
- 聚合状态：`failed`、`conflict`；可选 `pending_review`/`success` 且 `quality_warnings` 非空 → `ai_text_quality_warning`
- 去重：`task_type + source_id + failure_category`
- 深链：`/product/ai-text-batches/:batchId?itemId=:itemId`
- 恢复：应用成功 / 放弃 / 重生成成功后不再出现在未处理失败列表
- 重试：`POST /api/v1/products/ai-text/batches/:id/retry-failed`（failed / pending / running）；单条 `POST .../items/:id/regenerate`

### 旧入口

- `/ai/batches`：菜单隐藏，页面保留 + 旧版提示，链到 `/ai/text-batches`
- 禁止删除旧页（历史 `ai_operation_batches` 可查）

### 真实 Provider 试跑

- 见 [`BATCH_AI_TEXT_UX_ACCEPTANCE.md`](BATCH_AI_TEXT_UX_ACCEPTANCE.md)
- A3.1.2：**passed**（Qwen；16 子项；脚本 `scripts/ai-text-trial-run.ps1`）

## Phase A3.1.2 补充（2026-06-19）

### 路由 smoke

- `scripts/ai-text-route-smoke.ps1` / `.sh`：health + 12 条 ai-text 路由不得 404
- 结果：`docs/ai-text-route-smoke.json`

### 真实试跑与 P1 修复

- **异步 context**：`CreateBatch` 后台 goroutine 改用 `detachedGinContext`（`context.Background()`），避免 HTTP 返回后 generation 永久 `pending`
- **retry-failed**：同时重试 `failed` / `pending` / `running`（服务重启孤儿项）
- **试跑规模**：5 标题 + 5 描述 + 3 商品×双类型 = 16 子项；全部 `pending_review`，无自动覆盖

### 验收脚本

- `scripts/ai-text-route-smoke.ps1`
- `scripts/ai-text-trial-run.ps1`（从 `.env` 读取登录，不写密钥到输出）
