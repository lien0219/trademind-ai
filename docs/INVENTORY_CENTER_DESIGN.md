# 库存中心设计（Phase F3）

## 路由

| 路径 | 说明 |
| --- | --- |
| `/inventory` | 库存中心主入口（SKU 维度汇总） |
| `/inventory/alerts` | 库存预警 |
| `/inventory/deductions` | 库存扣减记录（原 `/inventory/effects` 重定向至此） |
| `/inventory/sync-tasks` | 平台库存同步任务 |
| `/inventory/sync-batches` | 同步批次 |
| `/inventory/logs` | 本地库存流水 |

## API

- `GET /api/v1/inventory` — 库存中心列表（keyword / stockStatus / skuBindStatus / syncStatus / hasException 等）
- `GET /api/v1/inventory/alerts` — 预警列表
- `GET /api/v1/inventory/effects` — 扣减/回滚影响记录
- `GET /api/v1/inventory-sync/tasks` — 同步任务

## 字段

库存中心每行对应一个 **本地 product_sku**，展示：

- 本地库存 / 可用库存（MVP 等同）
- 预警阈值、库存状态
- SKU 绑定状态（bound / unbound / ambiguous / none）
- 平台同步状态摘要
- 最近扣减、最近同步、异常数量

## 原则

- 不自动同步平台库存
- 不自动补货、不创建采购单
- 技术字段默认折叠（详情 Drawer / 技术详情 Tab）
- 状态全部中文化

## 深链

- 订单详情 `?tab=inventory` → 库存影响 Tab
- `/inventory?skuId=:skuId` → 定位 SKU
- `/inventory/deductions?orderId=:orderId` → 订单扣减记录
- `/inventory/sync-tasks?productSkuId=:skuId&id=:taskId` → 同步任务

## 权限（F3 轻量 RBAC）

- `admin` / `operator`：可查看与写操作
- `readonly`：仅查看；后端拒绝 adjust-stock / sync / retry
