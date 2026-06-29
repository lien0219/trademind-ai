# 订单异常工作台设计（Phase F2）

## 路由

`/orders/exceptions?orderId=&exceptionType=`

## 异常类型

| 类型 | 来源 |
| --- | --- |
| sku_unmatched | 未绑定 `product_sku_id` |
| sku_ambiguous | 多候选待确认 |
| insufficient_stock / inventory_deduct_failed | 库存影响流水 |
| inventory_sync_failed | 库存同步任务失败 |
| order_sync_partial_failed | `order_sync_tasks.status=partial_success` |

## 深链

| 方向 | URL |
| --- | --- |
| 异常 → 订单详情 | `/orders/:orderId` |
| 异常 → 同步任务 | `/orders/sync-tasks?id=:taskId` |
| 异常 → 失败任务中心 | `/ops/task-center/failures?taskType=order_sync&keyword=:id` |
| 订单详情 → 异常 | `/orders/exceptions?orderId=:id` |

## 标记已处理

- **不等于**自动修复 SKU 或库存事实
- 仅写入 `order_exception_marks`；文案在 UI 明确说明

## 人工绑定

- 必须确认弹窗
- 低置信度候选不自动绑定
- 不覆盖 `manual_bound`
