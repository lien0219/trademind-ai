# 本地开发说明

TradeMind 由 Go backend、React Admin、Node collector、PostgreSQL 与 Redis 组成。

## 环境要求

- Node.js 20+
- pnpm 10.34+
- Go 1.26+
- Docker / Docker Compose，或与 `.env` 匹配的本机 PostgreSQL 和 Redis

## 安装与启动

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

`pnpm dev` 启动 PostgreSQL/Redis、backend、Admin 和 collector。若 Docker 不可用，会检查 `.env` 指定的本机 PostgreSQL/Redis。
若未配置 `COLLECTOR_SERVICE_TOKEN`，该命令会仅为本次开发会话生成随机内部 Token，并同时传给 backend 与 collector；不会写回 `.env` 或输出 Token。单独启动 backend 与 collector 时，仍应在环境变量或 `.env` 中配置相同的非空 Token。

常用命令：

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm dev:stop
pnpm dev:reset
pnpm build:admin
pnpm build:collector
```

`pnpm dev:reset` 会重置默认 Compose 数据卷，可能清空本地数据，执行前必须确认目标。

## 默认端口

| 服务 | 默认地址 |
| --- | --- |
| backend | `http://127.0.0.1:8080` |
| backend health | `http://127.0.0.1:8080/health` |
| Admin | 通常为 `http://127.0.0.1:8000`，以终端输出为准 |
| collector | `http://127.0.0.1:3100` |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |

## 环境变量

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

不要提交 `.env`、真实密钥、Token、Cookie 或平台凭据。完整说明见 [env.md](env.md)。

## 测试与验收

- GitHub Actions 是自动化回归入口，持续运行前端、Collector、后端、契约、架构、PostgreSQL、Redis 和 Admin E2E 测试。
- 本地默认运行与改动相关的静态检查、配置检查和必要构建；完整自动化回归交由 CI。
- 功能、页面、文案、响应式和业务流程最终由维护者人工验收。
- 本地不要求存在 `trademind_test`，也不会自动重建。数据库集成测试由 CI service container 提供隔离数据库。
- 若明确要在本地运行数据库测试，必须显式提供指向隔离测试库的 `TEST_DATABASE_URL`；严禁连接开发库或生产库。

核心 CI 命令包括：

```bash
pnpm test:frontend
pnpm test:collector
pnpm test:contracts
pnpm test:backend
pnpm test:db:inventory
pnpm test:redis
pnpm architecture:test
pnpm workflow:test
```

Admin E2E 使用 Mock API，不访问真实平台；详情见 `.agents/skills/admin-e2e-testing/SKILL.md`。

## 分服务调试

### Backend

```bash
pnpm dev:backend
```

### Admin

```bash
pnpm dev:admin
```

### Collector

```bash
pnpm dev:collector
pnpm collect:test
```

`collect:test` 仅用于采集器测试模式，不等同于生产平台验收。

## 本地产物

`.playwright-mcp/`、`playwright-report/`、`test-results/`、截图和临时日志只用于当前排障，完成后清理，不提交 Git。不要新增阶段 gate 或长期运行证据目录。

## 生产能力边界

进入生产维护阶段不等于自动启用真实平台能力。真实凭据、真实网络、平台写入、Worker、自动业务重试和灰度仍由现有 fail-closed 配置与外部审批控制。
