# 本地开发说明

本文说明贸灵 TradeMind 的本地开发启动方式。完整项目由 Go backend、React admin、Node collector、PostgreSQL 与 Redis 组成。

## 环境要求

- Node.js
- pnpm `9.15+`
- Go `1.22+`
- Docker / Docker Compose

## 安装依赖

```bash
pnpm install
pnpm install:collector:browsers
```

## 一键开发启动

```bash
pnpm dev
```

`pnpm dev` 会启动本地基础设施与三个开发服务：

- `docker-compose.yml` 中的 PostgreSQL / Redis
- backend Go 服务
- admin 管理端
- collector 采集服务

## 常用命令

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm dev:stop
pnpm dev:reset
```

说明：

- `pnpm check:dev`：检查 Node、pnpm、Go、Docker、环境变量等。
- `pnpm dev:infra`：仅启动 PostgreSQL 与 Redis。
- `pnpm dev:stop`：停止默认 `docker-compose.yml` 服务，不删除 volume。
- `pnpm dev:reset`：重置默认 Compose 数据卷，可能清空本地数据库。

## 默认端口

| 服务 | 默认地址 |
| --- | --- |
| backend | `http://127.0.0.1:8080` |
| backend health | `http://127.0.0.1:8080/health` |
| admin | 通常为 `http://127.0.0.1:8000`，以终端输出为准 |
| collector | `http://127.0.0.1:3100` |
| collector health | `http://127.0.0.1:3100/health` |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |

## 环境变量

本地开发使用 `.env.example` 作为模板：

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

关键配置：

- `DB_DRIVER=postgres`
- `DB_PORT=5432`
- `REDIS_ADDR=127.0.0.1:6379`
- `APP_HTTP_ADDR=:8080`
- `COLLECTOR_HTTP_ADDR=:3100`
- `COLLECTOR_BASE_URL=http://127.0.0.1:3100`

不要提交 `.env` 或任何真实密钥。

## 分服务调试

基础设施：

```bash
pnpm dev:infra
```

后端：

```bash
pnpm dev:backend
```

管理端：

```bash
pnpm dev:admin
```

采集服务：

```bash
pnpm dev:collector
```

## 后端格式化

修改或新增 `backend/**/*.go` 后，在 `backend` 目录执行：

```bash
go fmt ./...
```

## 采集服务调试

```bash
pnpm collect:test -- --url "https://detail.1688.com/offer/..."
pnpm collect:test -- --source aliexpress --url "https://www.aliexpress.com/item/..."
```

## 故障排查

- Docker 未启动：先启动 Docker Desktop 或本机 Docker 服务。
- 端口冲突：修改 `.env` 或停止占用端口的进程。
- 后端连不上数据库：确认 `docker compose ps` 中 PostgreSQL 为 healthy。
- Collector 无法打开浏览器：重新执行 `pnpm install:collector:browsers`。
