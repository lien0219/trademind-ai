# 抖店生产审计报告（Phase 10.1）

> **扫描日期**：2026-06-13  
> **发布状态**：Release Candidate（生产加固 Phase 10.1 完成；**未**标记为 production available）  
> **真实 E2E**：`blocked_by_real_credentials`（本地/CI 无真实抖店 App Key、Secret 与授权店铺）

---

## 1. 当前实现清单（Phase 1–9.2 + 10.1）

| 模块 | 路径 | 能力 |
| --- | --- | --- |
| OpenAPI Client | `backend/internal/providers/platform/douyinshop/` | 统一签名、Token 刷新、shop/category/image/product/order/inventory |
| OAuth | `backend/internal/modules/shop/douyin_oauth.go` | 授权、回调、刷新、撤销、连接测试 |
| 类目/属性 | `shop/douyin_category.go` | 缓存同步 |
| 商品映射/图片 | `product/douyin_*.go` | 映射、素材中心上传 |
| 草稿创建 | `productpublish/douyin_create.go` | `product.addV2` 草稿 |
| SKU 绑定 | `productpublish/douyin_sku_*.go` | 自动校准 + 手动绑定 |
| 订单同步 | `douyinshop/order.go` + `ordersync` | 分页 `order.searchList` |
| 库存同步 | `douyinshop/inventory.go` + `inventory` | `sku.syncStock` |
| 失败任务/日志 | `taskcenter` + `operationlog` | 失败分类与审计 |
| **生产预检（10.1）** | `backend/internal/modules/douyinpreflight/` | 配置/授权/开关/Storage/数据状态 |
| **Storage 公网验证（10.1）** | `backend/internal/pkg/storagepublic/` | 上传探针 + 匿名 HTTP 探测 |

### 1.1 API 调用是否统一走 Client

- **是**：所有抖店 OpenAPI（token、类目、图片、商品、订单、库存）均经 `douyinshop.Client.do`。
- **例外（预期）**：商品图片下载（Storage/外链 → 字节）在 `product/douyin_images.go` 使用独立 HTTP GET，不经过 OpenAPI Client。

### 1.2 散落 HTTP / 重复 Token 刷新

| 项 | 结论 |
| --- | --- |
| 散落 Douyin OpenAPI HTTP | **无** |
| Token 刷新 HTTP 实现 | **单点**（`token.go`） |
| 刷新编排入口 | 多处：`Client.Do` 自动刷新、`ensureFreshClient`、`DouyinOAuthRefresh`、`GetShopInfo` 强制刷新 |
| 平台「测试连接」 | **仅配置校验**，不做真实 API（易误导，见 P1-2） |

---

## 2. 生产风险清单

### P0（上线阻断）

| ID | 风险 | 状态（10.1） |
| --- | --- | --- |
| P0-1 | Storage `public_base` 非公网 HTTPS，抖店无法拉取图片 | **已加检测**（`POST /storage/test-public-access` + 预检项）；真实公网 URL 仍待凭证环境验证 |
| P0-2 | 真实 App Key / OAuth / Token 刷新 / 全链路 E2E 未在真实环境验收 | **blocked_by_real_credentials** |
| P0-3 | 订单重复扣库存 / 商品重复创建等平台幂等未生产加固 | **Phase 10.2 已加固**（DB 唯一索引、订单 partial 重试、草稿 platform 回查、库存 dedup）；真实环境重复数为 0 仍待验 |

### P1（灰度前必须处理或明确接受）

| ID | 风险 | 状态（10.1） |
| --- | --- | --- |
| P1-1 | `real_api_enabled` / `product_publish_enabled` 配置存在但未在 Worker 强制校验 | 未修复（10.2） |
| P1-2 | 平台设置「测试连接」不做真实 API，与店铺 OAuth 测试语义不一致 | 未修复；预检 + 店铺测试可部分替代 |
| P1-3 | 无统一重试/限流/熔断策略 | 待 Phase 10.3 |
| P1-4 | 无结构化 Prometheus 指标 | 待 Phase 10.4 |
| P1-5 | 任务 stale 回收未统一 | 待 Phase 10.3 |
| P1-6 | 订单分页断点恢复生产化不足 | **Phase 10.2 已加固**（`retryPagesOnly`、输出 merge、`hasMore` → partial_success） |
| P1-7 | 接口 Fixture / 契约测试缺失 | **Phase 10.2 已加**（`douyinshop/testdata/` + `contract_test.go`） |

### P2（可灰度后迭代）

| ID | 风险 |
| --- | --- |
| P2-1 | 双路径 Token 刷新（`ensureFreshClient` + `Client.Do`）可能重复 refresh |
| P2-2 | `httppublic.IsPublicHTTPURL` 仅语法检查，不 DNS 解析（Storage 探针已做 DNS+IP 校验） |
| P2-3 | 任务告警默认关闭，需运维显式开启 |
| P2-4 | `GetShopInfo` 连接测试总是 force-refresh，自动化下可能增加限流 |

---

## 3. 分类统计（扫描基线）

| 级别 | 数量 | 说明 |
| --- | --- | --- |
| **P0** | **3** | 公网 Storage（工具已备、真实 URL 待验）、真实 E2E、幂等（10.2） |
| **P1** | **7** | 开关 enforcement、诊断一致性、可靠性、可观测、契约 |
| **P2** | **4** | 维护性与运维体验 |

---

## 4. 需要真实凭证验证的项目

以下 **不能** 在无凭证环境标记为通过：

1. Token `create` / `refresh` 真实响应与字段校准  
2. `shop.getShopCategory` / `product.getCatePropertyV2` 真实数据  
3. `supplyCenter.material.batchUploadImageSync` 使用真实公网图片 URL  
4. `product.addV2` / `product.detail` 多规格草稿  
5. `order.searchList` 分页与 Upsert 重复数为 0  
6. `sku.syncStock` 与绑定 SKU  
7. Storage 公网探针在 **生产域名 + 证书** 下的 HTTP 200 + `image/*`  
8. 限流 / 超时 / Token 并发刷新单飞（需压测或真实触发）

**人工步骤**（有凭证后）：

1. 配置 `platform_douyin_shop` 真实 App Key / Secret / Redirect URI / Service ID  
2. OAuth 授权至少 1 家测试店铺  
3. Storage 配置 **HTTPS 公网** `public_base`（或 COS/OSS/S3 对外前缀）  
4. 设置 → 存储 → **测试公网访问**  
5. 设置 → 平台开放配置 → 抖店 → **运行生产预检**（可选开启 liveTest）  
6. 按 [`DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md) 执行全链路  

---

## 5. Phase 10.1 已修复 / 新增

| 项 | 说明 |
| --- | --- |
| 生产预检 API | `POST/GET .../platform/douyin/production-preflight` |
| Storage 公网 E2E 探针 | `POST /api/v1/storage/test-public-access` |
| 管理端入口 | 平台开放配置 → 抖店「生产预检」；存储设置「测试公网访问」 |
| 错误码 | `STORAGE_PUBLIC_*` 系列 |
| 操作日志 | `douyin.production.preflight`、`storage.public_access.test` |
| 预检结果持久化 | `settings.douyin_preflight.latest_result` |

---

## 5.1 Phase 10.2 已修复 / 新增（契约与一致性）

| 项 | 说明 |
| --- | --- |
| 接口 Fixture | `backend/internal/providers/platform/douyinshop/testdata/*.json`（脱敏 Mock） |
| 契约测试 | `douyinshop/contract_test.go` — product.detail / order.searchList 解析与错误映射 |
| 订单 DB 唯一索引 | `ux_orders_shop_platform_ext_order`、`ux_order_items_order_ext_item`（Postgres migration） |
| 订单 partial 重试 | `ordersync/checkpoint.go` — `retryPagesOnly`、输出 merge、失败页-only 重试 |
| 抖店分页重试 | `SyncOrdersPaginated(..., retryPages)` — 仅拉取指定页 |
| 商品草稿幂等 | `GetProductDetailByOuterID` + 超时/重试前先 platform 回查；`mappingHash` 写入任务快照 |
| 库存同步 dedup | `stockVersion`（= targetStock）+ 同 publicationSku pending/running 去重 |
| E2E 报告模板 | [`docs/DOUYIN_E2E_REPORT_TEMPLATE.md`](DOUYIN_E2E_REPORT_TEMPLATE.md) |

---

## 6. 仍阻塞上线

1. 真实凭证 E2E 全绿（见 §4）  
2. P0-3 真实环境重复订单 / 重复扣减验证（代码加固已完成，见 §5.1）  
3. P1 可靠性 / 可观测 / 紧急停用（Phase 10.3–10.4）  
4. 灰度 48–72h 无阻断错误  
5. 回滚演练（文档 Phase 10.4）

---

## 7. 变更记录

| 日期 | 摘要 |
| --- | --- |
| 2026-06-13 | Phase 10.2：Fixture/契约测试、订单断点恢复、幂等 migration、草稿 platform 回查、库存 dedup |
| 2026-06-13 | Phase 10.1：基线扫描、生产预检、Storage 公网验证、管理端入口 |
