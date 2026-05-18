# Docker 部署说明

本文说明如何使用 Docker Compose 启动完整 TradeMind 项目。

## 组成服务

`docker-compose.full.yml` 包含：

- PostgreSQL 16
- Redis 7
- backend：Go Gin API
- admin：React 管理端，使用 nginx 托管并代理 `/api`
- collector：Node.js + Playwright 采集服务

## 快速启动

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

## 默认访问地址

| 服务 | 地址 |
| --- | --- |
| Admin | `http://127.0.0.1:8000` |
| Backend Health | `http://127.0.0.1:8080/health` |
| Collector Health | `http://127.0.0.1:3001/health` |

## 端口配置

可在 `.env` 中覆盖以下端口：

```env
ADMIN_PUBLISH_PORT=8000
BACKEND_PUBLISH_PORT=8080
COLLECTOR_PUBLISH_PORT=3001
POSTGRES_PUBLISH_PORT=5432
REDIS_PUBLISH_PORT=6379
```

完整环境变量说明见 [env.md](env.md)。修改 Docker 变量时必须同步 `.env.docker.example`、`docker-compose.full.yml`、本文档和 `docs/env.md`。

## 安全配置

生产环境或公网部署前必须修改：

- `JWT_SECRET`
- `APP_MASTER_KEY`
- `ADMIN_BOOTSTRAP_PASSWORD`
- `POSTGRES_PASSWORD`
- `DB_PASSWORD`
- 所有第三方平台、AI、存储、Webhook、邮箱等密钥

不要把真实密钥提交到仓库，也不要写入镜像。

## 常用命令

启动：

```bash
docker compose -f docker-compose.full.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.full.yml ps
```

查看日志：

```bash
docker compose -f docker-compose.full.yml logs -f backend
docker compose -f docker-compose.full.yml logs -f admin
docker compose -f docker-compose.full.yml logs -f collector
docker compose -f docker-compose.full.yml logs -f postgres
docker compose -f docker-compose.full.yml logs -f redis
```

停止并保留数据卷：

```bash
docker compose -f docker-compose.full.yml down
```

清空数据卷：

```bash
docker compose -f docker-compose.full.yml down -v
```

> `down -v` 会删除 PostgreSQL、Redis、上传目录等 Compose 管理的数据卷，请谨慎执行。

## 默认管理员

默认管理员由 `.env` 中的以下变量决定：

```env
ADMIN_BOOTSTRAP_EMAIL=admin@example.com
ADMIN_BOOTSTRAP_PASSWORD=admin123456
```

首次登录后请尽快修改密码。生产环境不要使用示例密码。

## 与本地开发 Compose 的区别

- `docker-compose.yml`：仅用于本地开发基础设施，包含 PostgreSQL + Redis。
- `docker-compose.full.yml`：用于完整 Docker 部署，包含 PostgreSQL + Redis + backend + admin + collector。

两套 Compose 的服务、端口和数据卷应分开理解。

## 配置校验

CI 会执行轻量 Docker 配置检查：

```bash
docker compose -f docker-compose.full.yml config
```

本地修改 Dockerfile、Compose 或 `.env.docker.example` 后，建议先执行同样命令确认语法和变量引用正确。
