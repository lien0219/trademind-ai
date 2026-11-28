# 数据库迁移锁设计（P2）

> 多 API 实例并行启动时，通过 **PostgreSQL Advisory Lock** 保证 `AutoMigrate` 串行执行，避免 DDL 竞态。

## 实现位置

- `backend/internal/database/migration_lock.go`
- 启动入口：`cmd/server/main.go`

```go
database.RunMigrateWithLock(ctx, db, timeout, database.AutoMigrateProductionSchema)
```

## Advisory Lock 键

使用 **两个 int32** 作为 `pg_advisory_lock` 键（会话级锁）：

| 键 | 值 | 说明 |
| --- | --- | --- |
| `migrationLockKey1` | `8837291` | 项目固定盐值 |
| `migrationLockKey2` | `20260710` | P2 可靠性阶段标识（2026-07-10） |

```sql
SELECT pg_try_advisory_lock(8837291, 20260710);  -- 非阻塞尝试
-- 迁移完成后
SELECT pg_advisory_unlock(8837291, 20260710);
```

`pg_try_advisory_lock` 失败时不阻塞连接，应用层循环重试。

## 执行流程

```text
启动 → staging/production 或 MIGRATION_RUN_ON_STARTUP=true？
  ├─ 是 → RunMigrateWithLock
  │         ├─ PostgreSQL 固定会话 → 获取锁 → 迁移 → 同会话释放锁
  │         └─ 锁被占用时每 500ms 重试，直到等待超时
  └─ 否 → 本地直接调用 AutoMigrateProductionSchema
```

### 超时

- `MIGRATION_LOCK_TIMEOUT_SECONDS`（默认 **120**）。
- 等待连接和获取锁共用该超时。迁移使用调用方数据库上下文，并复用持锁会话。
- 迁移失败时仍尝试释放锁。解锁失败会使启动失败。

### 非 PostgreSQL

`db.Dialector.Name() != "postgres"` 时 **直接** `run(db)`，不加锁。

MySQL 部署约定：单窗口迁移或 CI 专用迁移 Job。

## 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `MIGRATION_RUN_ON_STARTUP` | `true` | 启动时使用迁移锁。`staging` 和 `production` 的 PostgreSQL 始终使用锁。 |
| `MIGRATION_LOCK_TIMEOUT_SECONDS` | `120` | 等待锁最长时间 |

`MIGRATION_RUN_ON_STARTUP=false` 仅影响本地锁策略。服务启动仍会执行 schema 检查及迁移。安排大型数据库变更时，先在隔离环境验证上一版本、候选版本与数据库的兼容性，再在发布工单指定迁移窗口。

## P2 迁移内容

`migrateReliabilitySchema` 在 `AutoMigrate` 链中：

- 表：`idempotency_records`、`webhook_events`
- 索引：客服 `client_message_id`、webhook 唯一、幂等状态/租约索引

与 Phase 10.2 订单唯一索引、刊登批次索引 **独立**，均在 AutoMigrate 序列内顺序执行。

## 运维

滚动发布可并行启动；holder 崩溃会话结束自动释锁。监控 `database_migrate_failed` 与 lock timeout。
