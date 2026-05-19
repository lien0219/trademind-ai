# 本地开发说明

本文说明贸灵 TradeMind 的本地开发启动方式。完整项目由 Go backend、React admin、Node collector、PostgreSQL 与 Redis 组成。

## 环境要求

- Node.js
- pnpm `9.15+`
- Go `1.22+`
- **二选一**（基础设施）：
  - Docker / Docker Compose（默认，`pnpm dev` 会自动 `docker compose up` 拉起 PostgreSQL / Redis）
  - 或本机已安装并运行 **PostgreSQL**（默认 `127.0.0.1:5432`）与 **Redis**（默认 `127.0.0.1:6379`），账号密码与 `.env` 一致

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

- PostgreSQL / Redis：优先使用 Docker Compose（`docker-compose.yml`）；若未检测到可用 Docker，则检测本机 `.env` 配置的 PostgreSQL / Redis 端口是否可连接，两者都可用则跳过 Compose
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

- `pnpm check:dev`：检查 Node、pnpm、Go、Docker 或本机 PostgreSQL / Redis、环境变量等。
- `pnpm dev:infra`：仅启动 PostgreSQL 与 Redis。
- `pnpm dev`：启动前会自动释放本机 backend / admin（8000–8010）/ collector 端口上残留的上一进程，避免端口占用导致 backend 启动失败。
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

完整变量说明见 [env.md](env.md)。新增或修改变量时，还要按 [module-map.md](module-map.md) 检查 Docker、README、部署文档和代码默认值。

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

- Docker 未安装或未启动：可安装 Docker Desktop，或在本机启动 PostgreSQL / Redis（端口与 `.env` 中 `DB_HOST`/`DB_PORT`、`REDIS_ADDR` 一致）。
- 端口冲突：修改 `.env` 或停止占用端口的进程。
- 后端连不上数据库：使用 Docker 时确认 `docker compose ps` 中 PostgreSQL 为 healthy；使用本机服务时确认对应端口可连接。
- Collector 无法打开浏览器：重新执行 `pnpm install:collector:browsers`。
