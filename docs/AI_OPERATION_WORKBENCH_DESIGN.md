# AI 商品运营工作台设计（Phase A3.3）

> 日期：2026-06-27

## 目标

将 AI 文案、AI 图片、发布检查、刊登批次异常、失败任务中心聚合为统一待办工作台，运营人员打开一个页面即可知道「哪些商品需要处理、下一步点哪里」。

**不做**：新增 AI 生成能力、自动应用 AI 结果、自动创建刊登草稿、自动上架、调用外部平台 API、重写 taskcenter。

## 模块

- 后端：`backend/internal/modules/aiopsworkbench`
- 前端：`admin/src/pages/AI/OperationWorkbench`
- 菜单：**AI 工具 → 商品运营工作台**（`/ai/operation-workbench`）

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/ai/operation-workbench/summary` | 顶部 5 卡片统计 |
| `GET` | `/api/v1/ai/operation-workbench/todos` | 分页待办列表（默认 50 条） |
| `GET` | `/api/v1/ai/operation-workbench/todos/:id` | 单条待办详情 |
| `POST` | `/api/v1/ai/operation-workbench/todos/refresh` | 重新聚合（只读，无副作用） |

### 筛选参数

`type`、`priority`、`platform`、`shopId`、`keyword`、`start`、`end`、`page`、`pageSize`

## 待办类型

| 内部码 | 用户可见 |
| --- | --- |
| `ai_text_review` | AI 文案待复核 |
| `ai_text_conflict` | AI 文案内容冲突 |
| `ai_image_review` | AI 图片待复核 |
| `ai_image_conflict` | AI 图片有冲突 |
| `publish_check_failed` | 发布检查未通过 |
| `publish_check_warning` | 发布检查建议处理 |
| `publish_batch_failed` | 刊登任务失败 |
| `publish_batch_partial_success` | 刊登任务部分成功 |
| `taskcenter_failure` | 系统失败任务 |

## 优先级

| 级别 | 含义 | 典型映射 |
| --- | --- | --- |
| P0 | 数据覆盖 / 发布错误 / 权限 | 预留 |
| P1 | 阻断继续运营 | 文案/图片冲突、发布检查 failed、刊登 failed、taskcenter 失败 |
| P2 | 建议处理 | 质量 warning、发布检查 warning、刊登 partial_success |
| P3 | 普通提醒 | 待复核 AI 文案/图片 |

## 待办来源

### AI 文案

- 表：`ai_product_text_items`
- `pending_review` / `success` 且未 applied/rejected → `ai_text_review`
- `conflict` → `ai_text_conflict`
- `failed` → 仅经 taskcenter 展示为 `taskcenter_failure`（工作台去重）
- `quality_warnings` 非空 → 优先级升为 P2

跳转：`/product/ai-text-batches/:batchId?itemId=:itemId`

### AI 图片

- 表：`ai_product_image_items`
- 规则同 AI 文案

跳转：`/product/ai-image-batches/:batchId?itemId=:itemId`

### 发布检查

- 复用 `productcheck.CheckProductReadiness`（mode=draft）
- `error` → `publish_check_failed`（P1）
- `warning` → `publish_check_warning`（P2）
- 每条检查项一条待办，去重键含 `issueCode`

跳转：`/product/drafts/:id?tab=readiness&section=publish-check`

### 刊登批次

- 表：`product_publish_batches`
- `failed` → `publish_batch_failed`
- `partial_success` → `publish_batch_partial_success`

跳转：`/product/publish-batches/:batchId`

### 失败任务中心

- 复用 `taskcenter.ListFailures`
- 未 resolved、未 ignored/handled 的失败任务 → `taskcenter_failure`
- 与 AI 文案/图片 failed 同源去重

跳转：任务 `detailUrl` 或 `/ops/task-center/failures`

## 去重规则

Key：`todo:{sourceType}:{sourceId}:{issueCode}`

同一 source 的 failed 不在 AI 待复核与 taskcenter 重复展示。

## 消失条件

- AI 文案/图片：已应用、已放弃、冲突已解决、重生成进入新状态
- 发布检查：readiness 问题码消失
- 刊登批次：重试成功或失败项处理完成
- taskcenter：resolved 或 handled

## 与 operationdashboard 关系

- `operationdashboard`：广义商品运营 KPI / 漏斗 / 快捷入口
- `aiopsworkbench`：AI 运营待办明细 + 跳转，面向日常处理

## 性能

- 各来源 SQL 限量抓取（默认每源 ≤500）
- 发布检查扫描最近更新商品 ≤200 条/次聚合
- 列表分页，默认 pageSize=50
- 不加载 AI 大字段与平台 raw

## 抖店

本阶段未修改抖店 Provider 或 OpenAPI 字段；抖店仍为 **Release Candidate**。
