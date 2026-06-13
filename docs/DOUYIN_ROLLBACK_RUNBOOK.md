# 抖店回滚 Runbook（Phase 10.3）

## 紧急停用（首选，不删数据）

1. 管理端 → 平台开放配置 → 抖店运行状态 → **紧急停用**
2. 或 API：`POST /api/v1/platform/douyin/runtime-status/emergency-disable` body `{ "reason": "..." }`
3. 确认 Worker 不再调用抖店写接口；历史任务/订单可查看

## 功能开关回滚

在 `platform_douyin_shop` 设置中关闭：

- `real_api_enabled`
- `order_sync_enabled`
- `inventory_sync_enabled`
- `product_publish_enabled`

## 数据库索引回滚（仅索引，不删业务数据）

```sql
DROP INDEX IF EXISTS ux_orders_shop_platform_ext_order;
DROP INDEX IF EXISTS ux_order_items_order_ext_item;
```

## 运行状态恢复

```sql
-- 或通过管理端/API 恢复为 normal
UPDATE settings SET item_value = 'normal'
WHERE group_key = 'platform_douyin_shop' AND item_key = 'platform_runtime_status';
```

## 重复数据导致迁移失败

见 [`DOUYIN_DUPLICATE_DATA_REPAIR.md`](DOUYIN_DUPLICATE_DATA_REPAIR.md) — **禁止自动删除**，人工处理后重启迁移。

## 验证回滚

- 新建抖店任务应被阻止或标记 `cancelled`
- 日志无 Token/Secret 明文
- 应用可正常启动
