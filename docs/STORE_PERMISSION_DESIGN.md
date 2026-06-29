# 店铺授权设计（Phase F5）

## 数据表

`user_store_permissions`

| 字段 | 说明 |
| --- | --- |
| userId | 管理员 UUID |
| storeId | 店铺 UUID（shops.id） |
| platform | 平台 slug（冗余便于展示） |
| permissionScope | `view` / `operate` / `manage` |

## 范围规则

- **admin**：不过滤店铺
- **operator / readonly**：列表与详情按 `storeId IN (...)` 过滤
- 无授权店铺：列表为空；详情 404

## 覆盖模块

订单、客服、库存同步/预警、失败任务中心、操作日志（含 shopId 的记录）

## API

- `PUT /api/v1/admin/users/:id/store-permissions`（admin only）
