# 库存失败任务联动（Phase F3）

## 失败任务中心 taskType

`inventory_sync` — 来源表 `inventory_sync_tasks`

## 归类（failure category）

| 类别 | 说明 |
| --- | --- |
| inventory_deduct_failed | 订单扣减失败（异常工作台为主入口） |
| inventory_sync_failed | 同步任务失败 |
| inventory_sync_partial_success | 部分 SKU 同步失败 |
| inventory_sku_not_bound | SKU 未绑定阻断 |
| inventory_sku_ambiguous | 绑定歧义阻断 |
| inventory_stock_invalid | 库存值非法 |
| inventory_platform_permission_denied | 平台权限不足 |
| inventory_product_not_bound | 平台商品未绑定 |
| inventory_platform_sku_missing | 平台 SKU 缺失 |

## 深链

- 任务详情 → `/inventory/sync-tasks?id=:taskId`
- 异常工作台 inventory_sync_task → TaskCenterURL + InventoryURL
- 订单扣减失败 → `/inventory/deductions?orderId=:orderId`

## 操作

查看库存、查看商品、查看订单、查看 SKU 绑定、重试同步、查看同步任务。

## 原则

- 阻断（未绑定/歧义）不算系统失败，单独提示
- 同步失败与低库存预警分开展示
- 解决后状态随任务/异常 mark 同步
