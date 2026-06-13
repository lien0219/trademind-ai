# 抖店重复订单数据修复指南（Phase 10.3）

> **目的**：在创建 `ux_orders_shop_platform_ext_order` / `ux_order_items_order_ext_item` 唯一索引前，人工识别并处理历史重复数据。  
> **原则**：不自动删除、不静默合并；所有清理须人工确认。

---

## 1. 上线前检查顺序

1. 备份 PostgreSQL（全库或 `orders` / `order_items` 表）。
2. 启动应用或执行数据库迁移；`migrateDouyinPhase102Indexes` 会**自动**检测重复数据（无需手动 SQL）。
3. 若迁移成功 → 唯一索引已创建或已存在，可继续上线。
4. 若启动/迁移报错 `phase102 blocked: found ... duplicate ...` → **停止上线**，按本文档人工处理后再重试。

---

## 2. 重复订单（orders）判断规则

对同一 `(shop_id, platform, external_order_id)` 的多条记录：

| 保留优先级 | 条件 |
| --- | --- |
| 1 | 有完整 `order_items` 且库存已扣减关联的记录 |
| 2 | `updated_at` 最新且状态更接近终态（已发货/已完成） |
| 3 | `created_at` 最早且数据字段最完整 |

**不要保留**：

- `external_order_id` 为空或明显测试数据
- 明显重复导入、字段大量缺失的副本

**处理方式**：

- 将废弃记录软删除（`deleted_at`）或合并字段到保留记录后软删除副本
- 记录操作日志：操作者、保留 ID、废弃 ID 列表、原因

---

## 3. 重复订单行（order_items）

对同一 `(order_id, external_item_id)` 重复：

- 保留与平台 SKU、数量、金额一致的一条
- 删除（软删）其余副本
- 确认库存扣减日志 `inventory_change_logs` 只关联保留行

---

## 4. 迁移失败时的错误信息

应用迁移若检测到重复，会返回类似：

```text
phase102 blocked: found N duplicate order groups (example shop=... platform=... external_order_id=... count=... sample_ids=[...])
```

- `sample_ids` 为内部 UUID，不含买家隐私
- 按 sample 查询完整记录后人工决策

---

## 5. 回滚

索引回滚（不影响业务数据）：

```sql
DROP INDEX IF EXISTS ux_orders_shop_platform_ext_order;
DROP INDEX IF EXISTS ux_order_items_order_ext_item;
```

---

## 6. 修复后验证

1. 重复检查 SQL 返回 0 行
2. 重启应用，确认迁移成功
3. 触发一次订单同步，确认 Upsert 不报错
4. 同一 `external_order_id` 重复同步后库内仍为 1 条

---

**状态**：Release Candidate；真实环境重复数为 0 仍待有凭证环境验证。
