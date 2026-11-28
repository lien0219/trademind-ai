# 多实例安全设计（P2）

> TradeMind API 支持水平扩展：通过 **迁移锁、任务 DB 租约、Worker 注册心跳** 避免双写与重复消费。

## 架构假设

```text
        ┌─────────┐   ┌─────────┐
        │ API #1  │   │ API #2  │
        └────┬────┘   └────┬────┘
             │    Redis 队列   │
             └────────┬────────┘
                      │
                 PostgreSQL
```

- 无状态 HTTP：JWT 会话，不依赖单机内存会话。
- 有状态异步：任务状态在 DB + Redis LIST，Worker 可运行于任一 API 进程。

## 迁移锁（PostgreSQL Advisory Lock）

多实例同时 `AutoMigrate` 时仅一实例执行 DDL：

- `RunMigrateWithLock` → `pg_try_advisory_lock(8837291, 20260710)`。
- 未获锁每 500ms 重试，直至 `MIGRATION_LOCK_TIMEOUT_SECONDS`（默认 120s）。
- MySQL 等驱动 **跳过** advisory lock（约定单实例迁移）。

详见 `MIGRATION_LOCK_DESIGN.md`。

`staging` / `production` 的 PostgreSQL 启动迁移始终加锁。其他环境默认通过 `MIGRATION_RUN_ON_STARTUP=true` 启用同一迁移入口。

## Worker 租约（任务级 Leader）

**不是**全局选主；而是 **每条任务最多一个持有者**：

| 机制 | 说明 |
| --- | --- |
| `locked_by` + `locked_until` | BRPOP 后原子 claim |
| 心跳续期 | TTL/3 刷新 `locked_until` |
| Reaper | 过期租约回收，`lease_expired` 事件 |
| `lock_version` | 乐观并发控制 |

适用：collect、image、order_sync、customer_message_sync、product_publish、inventory_sync。

同一任务不会被两个 Worker 同时执行；不同任务并行无锁争用。

## Worker 实例注册（`worker.Registry`）

`WORKER_HEARTBEAT_ENABLED=true` 时：

- 每实例写 `worker_instances`（type、hostname、metadata）。
- `last_heartbeat_at` 周期更新；`MarkStaleWorkers` 标 stale。
- `/health` 汇总 `workers running` 与队列深度。

`WORKER_HEARTBEAT_ENABLED=false` 仍生成 `workerId` 并写任务租约，但不落实例表。

## 进程内 Leader 模式（轻量）

**任务告警扫描** `TASK_ALERT_SCAN_ENABLED`：

- 每进程可注册 `worker.TypeTaskAlertScan`。
- 实际是否执行由 settings `taskcenter.enable_alert_scan_worker` 门闸。
- 扫描使用 context 超时 `TASK_ALERT_SCAN_LOCK_TTL_SECONDS`（默认 120s），非 PG advisory；多实例可能重复扫描但 `upsert` 告警行幂等。

未来可复用 advisory lock 或 Redis SETNX 实现严格单 leader 告警扫描。

## Redis 与幂等

- 消费者 `BRPOP` 仅一实例获消息；DB claim 防崩溃后双执行。
- 第二层：`idempotency_records` 与领域唯一索引。

## 部署与反模式

- 必备：Redis 队列、`WORKER_REAPER_ENABLED=true`、`MIGRATION_RUN_ON_STARTUP=true`。
- 勿在多实例下关闭队列改 API 内同步；勿关闭 Reaper 后强杀进程（任务卡 `running`）。
- MySQL 无 advisory，多实例须外部协调单次迁移。

相关：`MIGRATION_LOCK_DESIGN.md`、`TASK_RELIABILITY_DESIGN.md`。
