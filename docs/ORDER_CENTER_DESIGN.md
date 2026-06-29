# 订单中心设计（Phase F2）

> 路由以 Admin 实现为准：`/orders/list`、`/orders/:id`、`/orders/exceptions`、`/orders/sync-tasks`。

## 目标

- 订单列表展示 SKU 匹配、库存扣减、同步与异常汇总
- 独立订单详情深链 `/orders/:id?itemId=`
- 与异常工作台、失败任务中心、同步任务互跳

## 列表字段

| 字段 | 来源 |
| --- | --- |
| SKU 匹配状态 | `order_item_sku_matches` 聚合 |
| 库存扣减状态 | `order_inventory_effects` 聚合 |
| 同步状态 | `externalOrderId` + `platform` 推导 |
| 异常数量 | 未匹配/ambiguous + 扣减失败 |

## 权限（F2 轻量）

| 角色 | 能力 |
| --- | --- |
| admin / operator | 查看、绑定 SKU、重试 |
| readonly | 仅查看；写操作 API 403 |

## 敏感信息

- 详情 API 默认脱敏手机号、邮箱
- 不返回平台 `rawData` 到列表

## 相关文档

- [ORDER_EXCEPTION_WORKBENCH_DESIGN.md](ORDER_EXCEPTION_WORKBENCH_DESIGN.md)
- [ORDER_SYNC_PARTIAL_SUCCESS_UX.md](ORDER_SYNC_PARTIAL_SUCCESS_UX.md)
- [ORDER_SKU_MATCHING_UX.md](ORDER_SKU_MATCHING_UX.md)
