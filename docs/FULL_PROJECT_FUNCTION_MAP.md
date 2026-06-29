# TradeMind 全项目功能地图

> **Phase F1**（2026-06-29）— 全项目功能缺口审计与后续开发路线规划。  
> **当前策略**：先把功能按规划开发完整，再统一进入总体验收；**不**进入最终人工测试、真实预发、抖店真实 E2E、生产灰度。  
> **当前状态**：`MVP Demo Ready` · `非 Production Ready` · 抖店 **Release Candidate** · `v0.1.0-demo` **Tag pending**

## 完成度分级

| 级别 | 含义 |
| --- | --- |
| `done` | 代码、页面、API、数据表齐备，Demo 可演示 |
| `partial` | 主能力已有，但缺联动、权限、真实平台或体验收口 |
| `missing` | 规划内能力尚未实现 |
| `deferred` | 明确后置，不在当前全项目 MVP 范围 |
| `manual_required` | 代码就绪，需人工凭证 / 环境 / 最终验收 |

## 模块总览

| # | 模块 | 完成度 | 阻塞 MVP | 风险 |
| --- | --- | --- | --- | --- |
| 1 | 登录 / 注册 / 用户 / 权限 | partial | 否 | 中 |
| 2 | 系统设置 | done | 否 | 低 |
| 3 | AI 设置 | done | 否 | 低 |
| 4 | 图片 AI 设置 | done | 否 | 低 |
| 5 | OCR 设置 | partial | 否 | 低 |
| 6 | Storage 设置 | done | 否 | 中 |
| 7 | 平台开放配置 | partial | 否 | 高 |
| 8 | 店铺授权 | partial | 否 | 高 |
| 9 | 采集中心 | done | 否 | 低 |
| 10 | 商品草稿 | done | 否 | 低 |
| 11 | 商品详情 | done | 否 | 低 |
| 12 | 定价规则 | done | 否 | 低 |
| 13 | 图片管理 | done | 否 | 低 |
| 14 | SKU / 规格管理 | done | 否 | 低 |
| 15 | AI 商品运营 | done | 否 | 低 |
| 16 | 发布检查 | done | 否 | 低 |
| 17 | 多平台刊登 | partial | 否 | 中 |
| 18 | 刊登批次 | done | 否 | 低 |
| 19 | 抖店草稿创建 | partial | 否 | 高 |
| 20 | 订单同步 | partial | 否 | 中 |
| 21 | 订单异常工作台 | partial | 否 | 中 |
| 22 | SKU 自动匹配 | partial | 否 | 中 |
| 23 | SKU 人工绑定 | partial | 否 | 中 |
| 24 | 库存预警 | done | 否 | 低 |
| 25 | 库存扣减 | done | 否 | 低 |
| 26 | 平台库存同步 | partial | 否 | 中 |
| 27 | 客服消息 | partial | 否 | 中 |
| 28 | AI 客服回复建议 | partial | 否 | 中 |
| 29 | 失败任务中心 | done | 否 | 低 |
| 30 | 操作日志 | done | 否 | 低 |
| 31 | 总 Dashboard | partial | 否 | 低 |
| 32 | Demo 数据 | done | 否 | 低 |
| 33 | 自动化脚本 | done | 否 | 低 |
| 34 | 文档中心 | partial | 否 | 低 |

---

## 1. 登录 / 注册 / 用户 / 权限

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/user/login`（登录 + 注册 Tab） |
| **已有接口** | `POST /api/v1/auth/login`、`logout`、`GET profile`、`POST register`、`send-email-code` |
| **已有数据表** | `admin_users`（含 `role` 字段，默认 `admin`） |
| **已完成能力** | JWT Bearer 鉴权、邮箱验证码注册、启动时 bootstrap 管理员、登录/登出操作日志 |
| **缺失能力** | RBAC 权限矩阵与路由级鉴权；管理员用户 CRUD；运营/只读角色；无权限提示页 |
| **阻塞整体 MVP** | 否（单管理员 Demo 可用） |
| **下一阶段建议** | Phase F5：角色权限矩阵 |
| **风险等级** | 中 — 任意登录管理员拥有全 API 权限 |

---

## 2. 系统设置

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/settings/system`、`/settings/security`、`/settings/email`、`/settings/alert-notify` |
| **已有接口** | `GET/PUT /api/v1/settings`；`POST /settings/test-email` |
| **已有数据表** | `settings`（`group_key=system|security|mail|alert_notify` 等） |
| **已完成能力** | 通用 settings CRUD、敏感字段加密、告警通知配置、邮箱 SMTP 测试 |
| **缺失能力** | 统一「配置状态中心」聚合视图（Phase F5） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F5 配置状态中心 |
| **风险等级** | 低 |

---

## 3. AI 设置

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/settings/ai` |
| **已有接口** | `GET/PUT /settings`（`group=ai`）、`POST /settings/test-ai` |
| **已有数据表** | `settings`（ai provider、model、api_key 等）；`ai_prompts` |
| **已完成能力** | OpenAI-compatible Provider、多模型配置、连接测试、Prompt 模板页 `/ai/prompts` |
| **缺失能力** | 配置状态中心一键总览 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F5 |
| **风险等级** | 低 |

---

## 4. 图片 AI 设置

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/settings/image` |
| **已有接口** | `POST /settings/test-image`、`POST /settings/test-ocr` |
| **已有数据表** | `settings`（`group=image`） |
| **已完成能力** | remove.bg / OpenAI Image / ComfyUI / dashscope 等 Image Provider 配置与测试 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F5 配置状态汇总 |
| **风险等级** | 低 |

---

## 5. OCR 设置

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/settings/image` 内 OCR Tab（非独立菜单） |
| **已有接口** | `POST /api/v1/settings/test-ocr` |
| **已有数据表** | `settings`（`ocr_provider`、PaddleOCR / 阿里云 / 腾讯云 / AI 视觉等键） |
| **已完成能力** | OCR Provider 抽象、真实 OCR 调用测试、图片文字翻译严格 OCR 模式 |
| **缺失能力** | 独立 OCR 设置入口；配置状态中心 OCR 健康态 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F5 或保持合并于图片 AI 设置并增强状态提示 |
| **风险等级** | 低 |

---

## 6. Storage 设置

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/settings/storage` |
| **已有接口** | `POST /settings/test-storage`、`POST /storage/test-public-access` |
| **已有数据表** | `settings`（`group=storage`）；`files` |
| **已完成能力** | local / S3 / COS / OSS / R2 Provider；上传 API；公网访问测试（抖店图片上传前置） |
| **缺失能力** | 生产环境公网 Storage 验证（`manual_required`） |
| **阻塞整体 MVP** | 否（本地 Demo 可用） |
| **下一阶段建议** | Phase F9 真实预发 Storage 验收 |
| **风险等级** | 中 — 抖店图片上传依赖公网 URL |

---

## 7. 平台开放配置

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/settings/platforms`、`/settings/platform-publish`、`/settings/integrations` |
| **已有接口** | `GET /platform/providers`、`GET/PUT /platform/settings/:platform`、`POST .../test-connection`、`GET/PUT /platform/publish-settings/:platform`；抖店生产预检 `POST/GET /platform/douyin/production-preflight` |
| **已有数据表** | `settings`（`platform_*` 分组） |
| **已完成能力** | 抖店/TikTok/Shopee/Lazada/Amazon 应用配置 schema；加密 Secret；连接测试；刊登预设 |
| **缺失能力** | 非抖店平台真实 OpenAPI 深度接入；统一配置健康 Dashboard |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F5 配置状态中心；抖店 Phase F9 真实 E2E |
| **风险等级** | 高 — 抖店仍为 Release Candidate |

---

## 8. 店铺授权

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/shops/manage`；各平台 OAuth 入口于设置/店铺页 |
| **已有接口** | `GET/POST/PUT/DELETE /shops`；OAuth：`/shops/:id/oauth/{douyin,tiktok,shopee,lazada,amazon}/*`；公开回调 `/shops/oauth/douyin/callback` |
| **已有数据表** | `shops`、`shop_auth_tokens` |
| **已完成能力** | 多平台 OAuth 闭环（抖店最深）；Token 加密；刷新/撤销/连接测试 |
| **缺失能力** | 真实平台凭证授权（`manual_required`）；eBay/Temu/SHEIN 等仍为 Planned |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F9 抖店真实 OAuth 验收 |
| **风险等级** | 高 |

---

## 9. 采集中心

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/collect/hub`、`/collect/tasks`、`/collect/batches`、`/collect/browser-profiles`、`/collect/rules`、`/collect/monitor` |
| **已有接口** | `POST/GET /collect/tasks`、`/collect/batches/*`、`/collect/monitor`、规则 CRUD、浏览器 Profile、1688/PDD/淘宝登录辅助 |
| **已有数据表** | `collect_tasks`、`collect_batches`、`collect_task_events`、`collect_rules`、`collect_browser_profiles` |
| **已完成能力** | 1688/拼多多/淘宝天猫/custom/aliexpress(beta) 采集；批量采集；AI 规则生成；失败进任务中心 |
| **缺失能力** | 速卖通/SHEIN/Temu 生产级采集 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F7 Demo 数据覆盖多源采集 |
| **风险等级** | 低 |

---

## 10. 商品草稿

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/product/drafts` |
| **已有接口** | `GET/POST /products`、`DELETE /products/:id`；列表含 `operationProgress` 摘要 |
| **已有数据表** | `products` |
| **已完成能力** | 草稿 CRUD、归档、来源筛选、运营进度摘要、批量多选（刊登/AI） |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F6 Dashboard 商品状态聚合 |
| **风险等级** | 低 |

---

## 11. 商品详情

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/product/drafts/:id` |
| **已有接口** | `GET/PUT /products/:id`、`GET /products/:id/operation-progress` |
| **已有数据表** | `products` 及关联表 |
| **已完成能力** | 统一草稿视图；顶部运营进度条；Tab：基础/图片/规格/定价/发布检查/刊登/库存；下一步入口 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F6 深链与 Dashboard 联动优化 |
| **风险等级** | 低 |

---

## 12. 定价规则

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/settings/pricing`；商品详情刊登 Tab 应用定价 |
| **已有接口** | `POST /pricing/calculate`、`POST /products/:id/pricing/apply`、`POST /products/pricing/batch-apply` |
| **已有数据表** | `settings`（`group=pricing`） |
| **已完成能力** | 成本来源、固定/百分比/倍率加价、运费、佣金、汇率、利润与尾数保护 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | — |
| **风险等级** | 低 |

---

## 13. 图片管理

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | 商品详情图片 Tab；`/files`；`/ai/image-tasks` |
| **已有接口** | 商品图片 CRUD/reorder/sync；`POST /files/upload`；`/image/tasks/*`、`/ai/image/*` |
| **已有数据表** | `product_images`、`files`、`image_tasks`、`image_task_items` |
| **已完成能力** | 外链同步到 Storage；AI 图片任务 14+ 类型；文字翻译；设主图/详情图；发布检查图片告警 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F7 Demo 含 AI 图片样本 |
| **风险等级** | 低 |

---

## 14. SKU / 规格管理

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | 商品详情规格 Tab |
| **已有接口** | SKU CRUD、库存阈值、`GET /product-skus/search` |
| **已有数据表** | `product_skus` |
| **已完成能力** | 规格组/规格值、价格库存、批量库存设置预览 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | — |
| **风险等级** | 低 |

---

## 15. AI 商品运营

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/ai/operation-workbench`、`/ai/text-batches`、`/ai/image-batches`、`/product/ai-text-batch`、`/product/ai-image-batch`、复核详情页 |
| **已有接口** | 单商品 AI；`/products/ai-text/batches/*`、`/products/ai-images/batches/*`；`/ai/operation-workbench/*` |
| **已有数据表** | `ai_product_text_batches/items`、`ai_product_image_batches/items`、`ai_tasks`、`ai_prompts` |
| **已完成能力** | 批量文案/图片四步向导；人工复核；冲突保护应用/撤销；工作台聚合待办 |
| **缺失能力** | 旧版 `/ai/batches` 仍并存（兼容）；`/ai/chat` 未实现 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F6 工作台与 Dashboard 统一入口 |
| **风险等级** | 低 |

---

## 16. 发布检查

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | 商品详情「发布检查」Tab；工作台/看板跳转 |
| **已有接口** | `GET /products/:id/readiness`、`POST /products/readiness/batch` |
| **已有数据表** | 无独立表（结果实时计算 + 任务快照引用） |
| **已完成能力** | passed/warning/failed 三态；平台/采集/图片/AI/刊登前置校验；中文动作建议 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | — |
| **风险等级** | 低 |

---

## 17. 多平台刊登

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | 商品详情刊登 Tab；`/product/publish-tasks` |
| **已有接口** | `/products/:id/publish-targets`、`check`、`create-drafts`；`/product-publish/tasks/*` |
| **已有数据表** | `product_publish_tasks`、`product_publications`、`product_publication_skus`、`product_platform_publish_configs` |
| **已完成能力** | 单商品多平台多店铺预检查；TikTok/Shopee/Lazada/Amazon `local_draft_only` 本地快照 |
| **缺失能力** | 除抖店外真实平台草稿 API |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | 后续平台接入按 Provider 模板扩展（非 F2–F8 优先） |
| **风险等级** | 中 |

---

## 18. 刊登批次

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/product/publish-batch`、`/product/publish-batches/:id` |
| **已有接口** | `/product-publish/batch-targets/*`、`/product-publish/batches/*` |
| **已有数据表** | `product_publish_batches`、`product_publish_tasks` |
| **已完成能力** | 5 步向导；统一/覆盖配置；批量上限与幂等；partial_success；失败重试；失败任务中心联动 |
| **缺失能力** | 大批次队列化（当前同步编排 ≤300 子任务） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F6 刊登异常 Dashboard 入口 |
| **风险等级** | 低 |

---

## 19. 抖店草稿创建

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial`（`manual_required` 真实 E2E） |
| **已有页面** | 商品详情刊登 Tab（映射/图片/创建草稿/SKU 绑定） |
| **已有接口** | `/products/:id/platform-configs/douyin_shop/*`；`/product-publications/:id/douyin/*` |
| **已有数据表** | `product_platform_publish_configs`、`product_publications`、`product_publication_skus` |
| **已完成能力** | 类目同步、字段映射、素材中心图片上传、`product.addV2` 草稿创建、SKU 校准与手动绑定 |
| **缺失能力** | 真实凭证 E2E（`blocked_by_real_credentials`）；公网 Storage；直接上架（deferred） |
| **阻塞整体 MVP** | 否（Demo 可走本地链路） |
| **下一阶段建议** | Phase F9 抖店真实 E2E |
| **风险等级** | 高 |

---

## 20. 订单同步

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/orders/list`、`/orders/sync-tasks` |
| **已有接口** | `GET/PUT /orders/*`；`POST /shops/:id/sync-orders`；`/order-sync/tasks/*` |
| **已有数据表** | `orders`、`order_items`、`order_shipments`、`order_sync_tasks` |
| **已完成能力** | 手动触发同步；分页 checkpoint；partial_success；抖店 `order.searchList` |
| **缺失能力** | 全平台自动化轮询；售后/退款（deferred） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F2** 订单中心完善 |
| **风险等级** | 中 |

---

## 21. 订单异常工作台

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/orders/exceptions` |
| **已有接口** | `/orders/exceptions/*`（handle/ignore/bind-sku/retry-deduct/retry-inventory-sync） |
| **已有数据表** | `order_exception_marks` |
| **已完成能力** | 异常标记、SKU 绑定、重试扣减/库存同步、与订单列表 Tab 联动 |
| **缺失能力** | 完整 OMS 工作流；与失败任务中心更深聚合 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F2** |
| **风险等级** | 中 |

---

## 22. SKU 自动匹配

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/orders/sku-matches`；订单详情 SKU 匹配 Tab |
| **已有接口** | `POST /orders/:id/match-skus`；`GET /order-item-sku-matches` |
| **已有数据表** | `order_item_sku_matches` |
| **已完成能力** | 订单同步后自动匹配策略（settings inventory）；候选 SKU 查询 |
| **缺失能力** | 跨平台统一匹配规则 UI；ambiguous 批量处理 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F2** |
| **风险等级** | 中 |

---

## 23. SKU 人工绑定

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial`（抖店 `done`，订单侧 `partial`） |
| **已有页面** | 商品详情刊登 Tab（抖店 SKU 绑定）；订单异常/规格匹配页 |
| **已有接口** | `POST /product-publication-skus/:id/douyin/bind-sku`；`POST /order-items/:itemId/bind-sku` |
| **已有数据表** | `product_publication_skus`、`order_item_sku_matches` |
| **已完成能力** | 抖店 publication SKU 手动绑定/解绑/重校；订单行人工绑定 |
| **缺失能力** | 非抖店平台 SKU 绑定 UI |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F2 + F3 SKU 未绑定阻断强化 |
| **风险等级** | 中 |

---

## 24. 库存预警

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/inventory/alerts` |
| **已有接口** | `GET /inventory/alerts` |
| **已有数据表** | 基于 `product_skus` 阈值与同步状态计算 |
| **已完成能力** | 缺货、低库存、平台库存不一致告警 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F3 Dashboard 库存异常入口 |
| **风险等级** | 低 |

---

## 25. 库存扣减

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/inventory/effects`、`/inventory/logs` |
| **已有接口** | 订单扣减/恢复；`GET /inventory/logs`、`/inventory/effects` |
| **已有数据表** | `inventory_change_logs`、`order_inventory_effects` |
| **已完成能力** | 订单同步后策略扣减；手动调整；审计流水 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F3 扣减失败兜底强化 |
| **风险等级** | 低 |

---

## 26. 平台库存同步

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/inventory/sync-tasks`、`/inventory/sync-batches`；商品详情库存 Tab |
| **已有接口** | `POST /product-publication-skus/:id/sync-inventory`、`POST /products/:id/sync-inventory`；`/inventory-sync/tasks|batches/*` |
| **已有数据表** | `inventory_sync_tasks`、`inventory_sync_batches` |
| **已完成能力** | 抖店 `sku.syncStock`；批量同步队列；SKU 未绑定阻断 |
| **缺失能力** | 默认可用性（`inventory_sync_enabled` 默认 off）；全平台同步 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F3** |
| **风险等级** | 中 |

---

## 27. 客服消息

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/customer/conversations`、`/customer/message-sync-tasks` |
| **已有接口** | 会话/消息 CRUD；`POST /shops/:id/sync-customer-messages`；`/customer/message-sync/tasks/*` |
| **已有数据表** | `customer_conversations`、`customer_messages`、`customer_message_sync_tasks` |
| **已完成能力** | 手动收件箱；平台消息同步任务；TikTok 等平台 Provider 预留 |
| **缺失能力** | 全渠道实时同步；复杂工单（deferred） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F4** |
| **风险等级** | 中 |

---

## 28. AI 客服回复建议

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/customer/conversations/:id`（AI 客服工作台） |
| **已有接口** | `POST /customer/conversations/:id/ai/generate-reply`；suggestion accept/discard/update；`POST /send-platform-message` |
| **已有数据表** | `customer_reply_suggestions` |
| **已完成能力** | AI 生成建议；人工编辑；人工确认发送；**不**自动外发 |
| **缺失能力** | 关联订单/商品上下文增强；发送失败与任务中心深度联动 |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F4** |
| **风险等级** | 中 |

---

## 29. 失败任务中心

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/ops/task-center/failures`、`/ops/task-center/alerts` |
| **已有接口** | `/task-center/summary`、`/failures/*`、`/alerts/*` |
| **已有数据表** | `task_failure_marks`、`task_alerts`、`task_alert_notifications` |
| **已完成能力** | 跨模块失败聚合（采集/AI/刊登/订单/库存/客服）；重试/忽略/处理；深链跳转 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F2–F4 新失败类型接入 |
| **风险等级** | 低 |

---

## 30. 操作日志

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | `/system/operation-logs` |
| **已有接口** | `GET /operation-logs` |
| **已有数据表** | `operation_logs` |
| **已完成能力** | 登录/设置/采集/AI/刊登/抖店/订单/库存等关键操作不可变日志 |
| **缺失能力** | — |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | — |
| **风险等级** | 低 |

---

## 31. 总 Dashboard

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | `/dashboard/product-operations`（商品运营看板） |
| **已有接口** | `GET /dashboard/product-operations` |
| **已有数据表** | 只读聚合 SQL（无独立表） |
| **已完成能力** | KPI、待办、漏斗、异常、12 项快捷入口、最近动态 |
| **缺失能力** | 全链路总运营首页（订单/库存/客服/配置提醒统一入口 — Phase F6） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F6** |
| **风险等级** | 低 |

---

## 32. Demo 数据

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | —（脚本驱动） |
| **已有接口** | HTTP API 由 `scripts/seed-demo-data.*` 调用 |
| **已有数据表** | 写入 products/collect/ai/publish 等 |
| **已完成能力** | 20 类商品 slot + 7 任务样本；`docs/DEMO_DATASET.md` |
| **缺失能力** | 订单/库存/客服/失败任务全链路 Demo 样本（Phase F7） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | **Phase F7** |
| **风险等级** | 低 |

---

## 33. 自动化脚本

| 项 | 内容 |
| --- | --- |
| **完成度** | `done` |
| **已有页面** | — |
| **已有接口** | — |
| **已有数据表** | — |
| **已完成能力** | `demo-auto-acceptance`、`demo-route-smoke`、`seed-demo-data`、AI 试跑、抖店 E2E preflight、perf 脚本 |
| **缺失能力** | 订单/客服/库存专项 smoke（Phase F7–F8） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | Phase F8 冻结后补全总体验收脚本 |
| **风险等级** | 低 |

---

## 34. 文档中心

| 项 | 内容 |
| --- | --- |
| **完成度** | `partial` |
| **已有页面** | 无 Admin 内嵌文档中心 |
| **已有接口** | — |
| **已有数据表** | — |
| **已完成能力** | 仓库 `docs/` 文档体系；`docs/README.md` 导航；AGENTS / module-map / PROGRESS |
| **缺失能力** | Admin 内文档/help 入口（可选 P2） |
| **阻塞整体 MVP** | 否 |
| **下一阶段建议** | 保持仓库 docs 为主；F1 新增全项目规划四文档 |
| **风险等级** | 低 |

---

## 相关文档

- [FULL_PROJECT_MVP_MAIN_FLOW.md](FULL_PROJECT_MVP_MAIN_FLOW.md) — MVP 主链路定义
- [FULL_PROJECT_DEVELOPMENT_PLAN.md](FULL_PROJECT_DEVELOPMENT_PLAN.md) — F2–F9 开发阶段
- [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md) — P0–P3 缺口分级
- [PROGRESS.md](PROGRESS.md) — 历史进度与变更记录
