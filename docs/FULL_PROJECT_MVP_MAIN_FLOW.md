# TradeMind 全项目 MVP 主链路

> **Phase F1**（2026-06-29）— 定义全项目 MVP 主链路各步骤的页面入口、失败兜底、下一步跳转与任务中心感知。
> **Phase F7**（2026-06-30）— Demo 数据覆盖 16 步；无凭证可走查（抖店 mock / `local_draft_only`）。
> **说明**：最终人工测试与真实 E2E **留 Phase F9**。

## 主链路总览

```text
采集商品
  → 生成商品草稿
  → 补齐商品信息
  → AI 优化标题 / 描述
  → AI 处理图片
  → 发布检查
  → 创建平台刊登草稿
  → 同步平台订单
  → SKU 匹配 / 人工绑定
  → 订单扣减库存
  → 平台库存同步
  → 客服消息拉取
  → AI 客服回复建议
  → 人工确认发送
  → 失败任务中心兜底
  → 总 Dashboard 查看运营状态
```

## 链路成熟度

| 阶段 | 代码闭环 | Demo 可演示 | 真实平台验收 |
| --- | --- | --- | --- |
| 采集 → 草稿 → AI → 发布检查 → 刊登批次 | ✅ | ✅ | 抖店 RC |
| 订单 → SKU → 库存 | ✅ | ✅（F7 样本） | `manual_required`（F9） |
| 客服 → AI 建议 → 人工发送 | ✅ | ✅（F7 样本） | `manual_required`（F9） |
| 失败兜底 → Dashboard | ✅ | ✅ | — |

---

## 步骤详解

### 1. 采集商品

| 项 | 内容 |
| --- | --- |
| **页面入口** | 采集 → **采集中心** `/collect/hub`；**采集任务** `/collect/tasks`；**批量采集** `/collect/batches` |
| **核心 API** | `POST /api/v1/collect/tasks`；`POST /collect/batches` |
| **成功输出** | `collect_tasks.status=success` → 关联 `product_id` |
| **失败兜底** | 任务失败 → **运维 → 失败任务中心** `/ops/task-center/failures`（类型：采集）；支持重试、打开浏览器登录 |
| **下一步跳转** | 采集任务详情 → **查看商品草稿**；或 商品 → **商品草稿** `/product/drafts?source=*` |
| **任务中心感知** | ✅ `taskType=collect` |

---

### 2. 生成商品草稿

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品 → **商品草稿** `/product/drafts` |
| **核心 API** | 采集成功自动 `POST /products`（内部）；`GET /products` |
| **成功输出** | `products.status=draft` |
| **失败兜底** | 采集 partial_success → 失败任务中心；草稿缺失字段 → 商品详情顶部 **运营进度** 提示 |
| **下一步跳转** | 点击行 → **商品详情** `/product/drafts/:id` |
| **任务中心感知** | 采集失败联动；草稿本身无独立失败类型 |

---

### 3. 补齐商品信息

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情 `/product/drafts/:id` — Tab：基础信息 / 图片 / 规格 / 定价 |
| **核心 API** | `PUT /products/:id`；SKU/图片 CRUD；`POST /products/:id/pricing/apply` |
| **成功输出** | `operationProgress` 步骤推进；发布检查 warning 减少 |
| **失败兜底** | 外链图片同步失败 → 失败任务中心（image/sync）；定价异常 → 页面 inline 错误 |
| **下一步跳转** | 顶部 **下一步** → 发布检查 Tab 或 AI Tab |
| **任务中心感知** | 图片同步类失败 ✅ |

---

### 4. AI 优化标题 / 描述

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情 AI 区；**批量 AI 优化** `/product/ai-text-batch`；**AI 工具 → 批量文案任务** `/ai/text-batches`；**商品运营工作台** `/ai/operation-workbench` |
| **核心 API** | `/products/:id/ai/optimize-title|generate-description`；`/products/ai-text/batches/*` |
| **成功输出** | 批次 `pending_review` → 复核 → `applied` |
| **失败兜底** | AI 失败 → 失败任务中心 `ai_text`；工作台待办跳转复核页 |
| **下一步跳转** | 应用后 → 发布检查；工作台 → 商品详情 / 复核页 |
| **任务中心感知** | ✅ |

---

### 5. AI 处理图片

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情图片 Tab；**批量 AI 图片** `/product/ai-image-batch`；**AI 工具 → 图片任务** `/ai/image-tasks`；工作台 |
| **核心 API** | `/image/tasks/*`；`/products/ai-images/batches/*` |
| **成功输出** | 任务 success / success_with_review → 应用到商品图 |
| **失败兜底** | 失败 → 失败任务中心 `ai_image` / `image_task`；低质量结果禁止静默设主图 |
| **下一步跳转** | 应用图片 → 发布检查 |
| **任务中心感知** | ✅ |

---

### 6. 发布检查

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情 **发布检查** Tab（锚点 `publish-check`）；工作台筛选「发布检查 failed/warning」 |
| **核心 API** | `GET /products/:id/readiness` |
| **成功输出** | `passed` 或仅剩 warning |
| **失败兜底** | `failed` 项展示中文动作与深链 Tab；不进失败任务中心（设计为前置 gate） |
| **下一步跳转** | **去刊登** → 刊登 Tab；批量 → **批量创建刊登草稿** |
| **任务中心感知** | 工作台聚合 ✅；独立 failure mark 否 |

---

### 7. 创建平台刊登草稿

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情 **刊登** Tab；**批量创建刊登草稿** `/product/publish-batch`；**刊登任务** `/product/publish-tasks`；**刊登批次详情** `/product/publish-batches/:id` |
| **核心 API** | `/products/:id/publish-targets/create-drafts`；`/product-publish/batches/*`；抖店 `/platform-configs/douyin_shop/create-draft` |
| **成功输出** | `product_publish_tasks.status=success`；抖店 `publishStatus=draft_created` |
| **失败兜底** | 失败 → 失败任务中心 `publish`；批次 partial_success → 批次详情重试失败项 |
| **下一步跳转** | 抖店 → SKU 绑定；订单同步入口于店铺管理 |
| **任务中心感知** | ✅ |

---

### 8. 同步平台订单

| 项 | 内容 |
| --- | --- |
| **页面入口** | 店铺 → **店铺管理** `/shops/manage`（同步订单）；订单 → **同步任务** `/orders/sync-tasks`；订单 → **订单列表** `/orders/list` |
| **核心 API** | `POST /shops/:id/sync-orders`；`/order-sync/tasks/*` |
| **成功输出** | `orders` 表 upsert；同步任务 success / partial_success |
| **失败兜底** | 失败 → 失败任务中心 `order_sync`；partial_success 可重试失败页 |
| **下一步跳转** | 订单详情 → SKU 匹配 Tab；异常 → **异常工作台** |
| **任务中心感知** | ✅ |

---

### 9. SKU 匹配 / 人工绑定

| 项 | 内容 |
| --- | --- |
| **页面入口** | 订单 → **规格匹配** `/orders/sku-matches`；订单详情 SKU Tab；商品详情刊登 Tab（抖店 SKU 绑定）；**异常工作台** `/orders/exceptions` |
| **核心 API** | `POST /orders/:id/match-skus`；`POST /order-items/:itemId/bind-sku`；抖店 `bind-sku` / `sync-sku-bindings` |
| **成功输出** | `bindStatus=bound`；`order_item_sku_matches.status=matched` |
| **失败兜底** | unmatched/ambiguous → 异常工作台；库存同步前阻断 `DOUYIN_SKU_BINDING_REQUIRED` |
| **下一步跳转** | 绑定完成 → 库存同步；异常处理 → 重试扣减 |
| **任务中心感知** | 订单异常部分联动；SKU 绑定错误码在刊登/库存失败中 ✅ |

---

### 10. 订单扣减库存

| 项 | 内容 |
| --- | --- |
| **页面入口** | 库存 → **订单库存影响** `/inventory/effects`；**库存流水** `/inventory/logs` |
| **核心 API** | 订单同步后内部 `DeductInventoryForOrder`；`POST /orders/:id/deduct-inventory`（如暴露） |
| **成功输出** | `order_inventory_effects` + `inventory_change_logs` |
| **失败兜底** | 扣减失败 → 异常工作台 **重试扣减**；失败任务中心（如配置告警） |
| **下一步跳转** | 库存预警 `/inventory/alerts`；平台库存同步 |
| **任务中心感知** | 异常工作台 ✅ |

---

### 11. 平台库存同步

| 项 | 内容 |
| --- | --- |
| **页面入口** | 商品详情 **库存** Tab；库存 → **库存同步任务** `/inventory/sync-tasks`；**库存同步批次** `/inventory/sync-batches` |
| **核心 API** | `POST /products/:id/sync-inventory`；`/inventory-sync/tasks/*` |
| **成功输出** | 同步任务 success；平台库存与本地一致 |
| **失败兜底** | 失败 → 失败任务中心 `inventory_sync`；SKU 未绑定前置阻断 |
| **下一步跳转** | 库存预警；Dashboard 库存异常 |
| **任务中心感知** | ✅ |

---

### 12. 客服消息拉取

| 项 | 内容 |
| --- | --- |
| **页面入口** | 客服 → **会话列表** `/customer/conversations`；**消息同步任务** `/customer/message-sync-tasks` |
| **核心 API** | `POST /shops/:id/sync-customer-messages`；`/customer/message-sync/tasks/*` |
| **成功输出** | 新消息写入 `customer_messages` |
| **失败兜底** | 同步失败 → 失败任务中心 `customer_sync` |
| **下一步跳转** | 点击会话 → **AI 客服工作台** `/customer/conversations/:id` |
| **任务中心感知** | ✅ |

---

### 13. AI 客服回复建议

| 项 | 内容 |
| --- | --- |
| **页面入口** | **AI 客服工作台** `/customer/conversations/:id` |
| **核心 API** | `POST /customer/conversations/:id/ai/generate-reply` |
| **成功输出** | `customer_reply_suggestions.status=pending` |
| **失败兜底** | AI 失败 inline 提示；可重试生成 |
| **下一步跳转** | **确认发送** 按钮 |
| **任务中心感知** | 部分（AI 任务 log）；非独立 failure 类型 |

---

### 14. 人工确认发送

| 项 | 内容 |
| --- | --- |
| **页面入口** | AI 客服工作台 — 编辑建议 → **发送到平台** |
| **核心 API** | `POST /customer/conversations/:id/send-platform-message` |
| **成功输出** | 消息 `direction=outbound`；suggestion `accepted` |
| **失败兜底** | 发送失败 → 页面错误 + 可重试；**不**自动重发 |
| **下一步跳转** | 返回会话列表；Dashboard 客服待回复 |
| **任务中心感知** | 平台发送失败可扩展进 taskcenter（Phase F4） |

---

### 15. 失败任务中心兜底

| 项 | 内容 |
| --- | --- |
| **页面入口** | 运维 → **失败任务中心** `/ops/task-center/failures`；**告警中心** `/ops/task-center/alerts` |
| **核心 API** | `/task-center/failures/*`；`/task-center/alerts/*` |
| **覆盖类型** | collect、ai_text、ai_image、image_task、publish、order_sync、inventory_sync、customer_sync 等 |
| **能力** | 筛选、批量重试、忽略、处理、深链回原模块 |
| **下一步跳转** | 各模块详情页 / 复核页 / 批次页 |
| **任务中心感知** | 自身即为兜底层 ✅ |

---

### 16. 总 Dashboard 查看运营状态

| 项 | 内容 |
| --- | --- |
| **页面入口** | 工作台 → **商品运营看板** `/dashboard/product-operations`；AI → **商品运营工作台** `/ai/operation-workbench` |
| **核心 API** | `GET /dashboard/product-operations`；`/ai/operation-workbench/summary` |
| **展示** | KPI、待办、漏斗、异常、快捷入口、最近动态 |
| **失败兜底** | API 失败 → 页面错误态 + 本地兜底结构 |
| **下一步跳转** | 12+ 快捷入口深链到各模块 |
| **任务中心感知** | 聚合失败/异常计数 ✅ |
| **Phase F6/F7** | 全链路总运营首页 + Demo KPI 样本（10 项） |

---

## 主链路断点（F7 后剩余）

| 断点 | 说明 | 阶段 |
| --- | --- | --- |
| 抖店真实 E2E | 无凭证时 create-draft / 订单 / 库存为 `manual_required` | F9 |
| Storage 公网 | 抖店图片上传前置条件 | F9 |

> F2–F6 已收口：RBAC、订单详情、客服失败兜底、Dashboard 全链路入口、Demo 全链路样本（F7）。

## 不在本阶段验收的范围

- 最终人工完整走查（Phase F9）
- 真实预发部署与 HTTPS（Phase F9）
- 抖店真实 E2E 与 48–72h 灰度（Phase F9）
- 生产灰度与 Production Ready 判定（Phase F9）
- 打 `v0.1.0-demo` tag（Phase F9 后人工决定）

## 相关文档

- [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md)
- [FULL_PROJECT_DEVELOPMENT_PLAN.md](FULL_PROJECT_DEVELOPMENT_PLAN.md)
- [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md)
- [DEMO_CHECKLIST.md](../DEMO_CHECKLIST.md)
