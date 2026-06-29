# 库存扣减记录（Phase F3）

## 路由

`/inventory/deductions`（`/inventory/effects` 301 式 redirect）

## 数据来源

`order_inventory_effects` 表 + 关联 orders / product_skus。

## 字段

| 字段 | 说明 |
| --- | --- |
| 扣减时间 | createdAt |
| 来源订单 | 链接 `/orders/:id?tab=inventory` |
| 商品 / SKU | 关联 product 标题与 skuCode |
| 扣减数量 | quantity |
| 扣减前/后库存 | beforeStock / afterStock |
| 扣减状态 | success / failed / skipped |
| 失败原因 | errorMessage |

## 来源类型

- `deduct` → 订单同步扣减
- `restore` → 系统回滚
- 人工修正走 `inventory_change_logs`（库存流水页）

## 联动

- 扣减失败 → 订单异常工作台 `inventory_deduct_failed`
- 订单详情库存 Tab → 扣减记录深链
- 失败任务中心（库存同步类任务单独归类）

## 禁止

- 静默扣减失败
- 多仓 WMS 流水（MVP 不做）
