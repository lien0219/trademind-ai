# 多平台刊登设计（Phase A1.2）

> 单商品多平台 / 多店铺刊登中心；批量多商品发布留待下一阶段。

## 平台能力分级

| 内部能力码 | 运营可见文案 | 行为 |
| --- | --- | --- |
| `real_draft_create` | 可创建平台草稿 | 调用平台真实写接口创建草稿（当前仅 **抖店**） |
| `local_draft_only` | 仅生成本地草稿 | 生成本地 `product_publications` + 任务快照，**不调用**外部平台 API |
| `not_configured` | 尚未配置 | 平台开放配置或刊登预设未完成 |
| `not_authorized` | 店铺未授权 | 需先在店铺管理完成 OAuth |
| `disabled` | 已停用 | Provider 或能力已关闭 |

能力来源：`GET /api/v1/platform/providers` 注册表 + 店铺表 + 平台开放配置完整性；**不在页面写死平台列表**。

## 单商品多目标刊登

1. **刊登目标**：按平台展示已授权店铺，支持多选。
2. **统一配置**：标题 / 描述 / 价格 / 图片 / 包裹 / 库存策略（接口预留 `commonConfig`）。
3. **单独配置**：各平台 Tab 内覆盖（抖店类目属性、图片同步等）；**单独配置优先生效**。

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/products/:id/publish-targets` | 可刊登平台与店铺及能力分级 |
| POST | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查（不写库、不调平台写接口） |
| POST | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建刊登草稿；每目标一子任务，汇总为 `product_publish_batches` |

### 检查响应摘要

- `ready` → 可以创建草稿
- `warning` → 需要检查（可继续，需人工确认）
- `blocked` → 暂不能创建草稿

每个目标的 `issues[]` 含 `title` / `message` / `severity`；内部码在 `technicalDetails.rawCode`，默认不在主文案展示。

### 创建草稿

- 抖店目标：复用 `POST .../douyin_shop/create-draft` 同款链路（`product.addV2` 保存平台草稿）。
- `local_draft_only`：同步写入本地 publication + `TaskTypeLocalDraftCreate` 成功任务。
- 部分失败 → 批次 `partial_success`；成功项不回滚。
- `retryFailedOnly` + `batchId` 仅重试失败子任务。

## 与直接上架的边界

- 本阶段名称准确为 **批量创建刊登草稿**，不是「一键发布 / 直接上架」。
- 未接入真实 Provider 的平台**不得**伪装为已发布成功。
- 抖店仍为 **Release Candidate**；OpenAPI 字段未改。

## 下一阶段（不在 A1.2）

- 多商品批量选择 + 批量刊登中心
- 统一配置 UI 完整表单
- 各跨境平台真实 `ProductPublishProvider` 草稿创建（在 Provider 就绪后由 `real_draft_create` 升级）
