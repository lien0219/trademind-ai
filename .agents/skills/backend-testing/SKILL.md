---
name: backend-testing
description: TradeMind Go/Gin/GORM 后端单元、HTTP、PostgreSQL、Redis、队列、状态机和第三方适配器测试规范
---

# TradeMind 后端测试规范

## 自动适用

涉及 `backend/**`、Go handler/service/model/repository/provider、数据库、Redis、队列、worker、scheduler、auth、权限、状态机或后端 CI 时自动适用。后端生产代码和测试代码质量由 `.agents/skills/code-quality/SKILL.md` 约束。

handler/service/repository/adapter/worker 的测试应验证模块边界、事务归属、幂等和越层调用风险；详细边界、循环依赖和 Architecture Baseline/Ratchet 由 `.agents/skills/modular-architecture/SKILL.md` 定义。

## 技术栈

- 语言：Go 版本以 `backend/go.mod` 的 `go` 指令为准，module 为 `github.com/trademind-ai/trademind/backend`。
- HTTP：Gin。
- ORM：GORM；默认 PostgreSQL，也存在 MySQL driver。
- Redis：go-redis v9，LIST 队列 + workers。
- 测试：标准 `go test`、`httptest`、`testify`。不引入第二套冲突框架。

## 单元测试

覆盖 domain/service/provider 纯逻辑：正常、参数错误、资源不存在、权限不足、状态不允许、外部依赖失败、事务回滚、幂等、重复请求、错误 envelope、边界值。

重点模块：auth、adminperm、product、SKU、pricing、inventory、productcheck readiness、productpublish、Douyin draft/SKU binding、image/files、taskcenter、idempotency、queue/worker、webhook、observability。

## HTTP 集成测试

使用 Gin + `httptest`。测试真实 route、middleware、auth、DTO、handler、service、error handler 和 response envelope。第三方平台必须 fake/stub，不访问真实平台。

关键 endpoint：auth/profile、image/providers、product detail、readiness、inventory、publications、publication SKUs、publish targets、Douyin create-draft、traditional publish。

## PostgreSQL 集成测试

必须通过安全守卫：数据库名或 URL 包含 `test`、`_test` 或 `e2e`，环境为 test，禁止生产域名/生产库名/默认开发库。无安全测试库时测试应明确 skip 或失败原因，不 fallback 到开发库。

覆盖 AutoMigrate 从空库执行、关键表/索引/约束、repository CRUD、unique/foreign key、transaction rollback、pagination、ordering、concurrency、idempotency、JSON/enum/state 字段。

## Redis / 队列测试

必须通过安全守卫：`TEST_REDIS_URL`、测试 DB 编号或测试 key 前缀，不连接生产/开发业务 Redis。覆盖 cache set/get/expire、miss、invalidation、lock/idempotency、enqueue、consume、retry、failed task、duplicate task、状态转换。

## 后台任务和状态机

queue/worker/scheduler/cron/background task 需要覆盖 created/running/success/failure/retry/cancel/timeout/duplicate/idempotency/非法转换/外部平台失败/事务失败。时间逻辑使用 fake clock 或短上下文，不长 sleep。

## 第三方适配器

Douyin、TikTok、Shopee、Lazada、Amazon、AI、image、storage、OCR、email、collector 均使用 fake server/stub。覆盖 4xx/5xx/timeout/非法 JSON/字段缺失/限流/token 失效/签名失败/retry 边界。

## 命令

- `pnpm test:backend`：Go 单元测试。
- `pnpm test:backend:integration`：PostgreSQL/HTTP 集成测试（需安全测试库）。
- `pnpm test:db`：数据库迁移/约束测试。
- `pnpm test:redis`：Redis/队列集成测试（需安全测试 Redis）。

## 禁止项

不得修改业务逻辑让测试容易；不得连接真实服务；不得 skip/only 掩盖失败；不得用真实凭据或真实店铺。
