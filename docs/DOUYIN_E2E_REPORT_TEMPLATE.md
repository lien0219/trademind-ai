# 抖店真实环境 E2E 验收报告模板

> **用途**：在有真实 App Key / Secret 与授权店铺时填写；无凭证环境标记 `blocked_by_real_credentials`，不得伪造通过。  
> **发布状态**：Release Candidate（本报告不自动将抖店标记为 production available）

---

## 1. 环境与元信息

| 字段 | 值 |
| --- | --- |
| 执行人 | |
| 执行时间（UTC） | |
| TradeMind 版本 / Git SHA | |
| 管理端 URL | |
| API Base URL | |
| 数据库 | PostgreSQL |
| 环境 | `development` / `staging` / `production` |
| `real_api_enabled` | |
| 测试店铺 ID（内部 UUID） | |
| 是否允许写操作 | `ALLOW_DOUYIN_WRITE_TEST=true` 是 / 否 |

---

## 2. 应用配置与预检

| 检查项 | 结果 | 备注 |
| --- | --- | --- |
| App Key 已配置 | pass / fail / blocked | |
| App Secret 可解密 | pass / fail / blocked | |
| OAuth Redirect URI（HTTPS） | pass / fail / blocked | |
| 生产预检 `POST .../production-preflight` | passed / warning / failed | `passedCount` / `failedCount` |
| Storage 公网探针 | pass / fail / blocked | 错误码：`STORAGE_PUBLIC_*` |

---

## 3. 店铺授权与 Token

| 检查项 | 结果 | requestId（脱敏） | 备注 |
| --- | --- | --- | --- |
| OAuth 授权 | pass / fail / blocked | | |
| Token 刷新 | pass / fail / blocked | | |
| 店铺连接测试 | pass / fail / blocked | | |
| 授权状态 `authorized` | pass / fail | | |

---

## 4. 只读链路（推荐先跑）

| 步骤 | API / 能力 | 结果 | 备注 |
| --- | --- | --- | --- |
| 类目同步 | `shop.getShopCategory` | pass / fail / blocked | |
| 属性同步 | `product.getCatePropertyV2` | pass / fail / blocked | |
| 商品详情回查 | `product.detail` | pass / fail / blocked | |
| 订单列表（小时间窗） | `order.searchList` | pass / fail / blocked | 重复订单数 = |

---

## 5. 写操作链路（需显式开启写测试）

| 步骤 | API / 能力 | 结果 | 测试资源前缀 | 人工清理 |
| --- | --- | --- | --- | --- |
| 图片上传 | `supplyCenter.material.batchUploadImageSync` | pass / fail / blocked | | |
| 商品草稿创建 | `product.addV2` | pass / fail / blocked | `[TradeMind E2E]` | |
| SKU 绑定校准 | `product.detail` + 本地匹配 | pass / fail / blocked | | |
| 库存同步 | `sku.syncStock` | pass / fail / blocked | 仅测试 SKU | |
| 订单同步 Upsert | 多页 `order.searchList` | pass / fail / blocked | | 重复 Upsert = 0 |
| 库存扣减 | 本地 ledger | pass / fail / blocked | | 重复扣减 = 0 |

---

## 6. 数据一致性与幂等（Phase 10.2+）

| 检查项 | 期望 | 实测 | 备注 |
| --- | --- | --- | --- |
| 订单唯一索引 | `(shop_id, platform, external_order_id)` 无重复 | | |
| 订单行唯一 | `(order_id, external_item_id)` 无重复 | | |
| partial_success 重试 | 仅重试失败页 | pass / fail / n/a | |
| maxPages + hasMore | 状态为 partial_success | pass / fail / n/a | |
| 商品草稿超时恢复 | `product.detail` 回查后不重创 | pass / fail / n/a | |
| 库存同步 dedup | 同 publicationSku + stock 不重复 pending | pass / fail / n/a | |

---

## 7. 失败任务中心与安全

| 检查项 | 结果 | 备注 |
| --- | --- | --- |
| 失败任务可重试 | pass / fail | |
| 日志无完整 Token / Secret | pass / fail | |
| Fixture 无真实隐私 | pass / fail | |

---

## 8. 结论

| 项 | 值 |
| --- | --- |
| 未通过 P0 项 | |
| 未通过 P1 项 | |
| `blocked_by_real_credentials` | 是 / 否 |
| 是否允许灰度上线 | 是 / 否 / 待 Phase 10.3–10.4 |
| 下一步 | |

---

## 9. 变更记录

| 日期 | 摘要 |
| --- | --- |
| 2026-06-13 | Phase 10.2 初版模板（契约 Fixture + 幂等 + 断点恢复验收项） |
