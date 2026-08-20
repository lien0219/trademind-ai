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
cp .env.example .env
# 在 .env 中设置独立随机的 COLLECTOR_SERVICE_TOKEN（至少 32 字符）
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
# 在 .env 中设置独立随机的 COLLECTOR_SERVICE_TOKEN（至少 32 字符）
docker compose -f docker-compose.full.yml up -d --build
```

## GHCR 预构建镜像

`Container Images` GitHub Actions 工作流负责 main 验证构建与正式 Tag 发布。只有 `main` 在镜像相关源码变化时自动生成验证镜像；`dev`、`feat/*`、`fix/*` 和 `release/*` 的 push 不发布镜像。`v<version>` Git Tag 通过版本与 `main` 归属校验后生成正式镜像。三个服务镜像统一发布到一个容器包：

```text
ghcr.io/lien0219/trademind
```

backend、admin、collector 仍是三个独立镜像，各自拥有多架构 manifest digest；统一 Package 只改变 GHCR 仓库名，并通过 `backend-*`、`admin-*`、`collector-*` 标签隔离服务。

所有镜像均构建 `linux/amd64` 与 `linux/arm64`，并附带 OCI 元数据、SBOM 和 provenance。镜像版本的唯一来源是 [`deploy/IMAGE_VERSION`](../deploy/IMAGE_VERSION)，格式为不含 `+build` 元数据、最长 48 字符的 Docker tag 安全 SemVer。

| 标签 | 更新规则 | 用途 |
| --- | --- | --- |
| `<service>-main` | main 每次构建更新 | 跟随 main，例如 `backend-main`。 |
| `<service>-main-v<version>` | main 每次构建更新；版本文件变化后切换标签 | 标识 main 当前版本，例如 `admin-main-v0.2.0`。 |
| `<service>-sha-<full-commit>` | main 与正式 Tag 构建均写入 | 按服务和提交定位镜像；它仍是可变 tag，不替代 manifest digest。 |
| `<service>-v<version>`、`<service>-<version>` | 仅通过校验的同名 Git Tag 发布 | 正式版本标签，例如 `collector-v0.2.0` 与 `collector-0.2.0`。 |
| `<service>-latest` | 仅通过校验的正式 Git Tag 更新 | 各服务最新正式版本，不由普通 main 合并更新。 |

`<service>` 为 `backend`、`admin` 或 `collector`。使用 Compose 从统一 Package 拉取同一版本的完整服务组：

```env
TRADEMIND_BACKEND_IMAGE=ghcr.io/lien0219/trademind:backend-main-v0.2.0
TRADEMIND_ADMIN_IMAGE=ghcr.io/lien0219/trademind:admin-main-v0.2.0
TRADEMIND_COLLECTOR_IMAGE=ghcr.io/lien0219/trademind:collector-main-v0.2.0
```

```bash
docker compose -f docker-compose.full.yml pull backend admin collector
docker compose -f docker-compose.full.yml up -d --no-build
```

### 正式发布

1. 在 `release/*` 更新 `deploy/IMAGE_VERSION`、Changelog 和部署说明，经 PR 合并到 `main`。
2. 等待 `main` 的 CI 完成并完成人工验收。
3. 从已更新的 `main` 提交创建与版本文件完全一致的 annotated Tag 并推送：

```bash
git switch main
git pull --ff-only origin main
git tag -a v0.2.0 -m "TradeMind v0.2.0"
git push origin v0.2.0
```

Tag 必须严格为 `v<deploy/IMAGE_VERSION>`，并指向已包含在远程 `main` 中的提交；否则工作流直接失败。正式 Tag 为每个服务发布 `<service>-v0.2.0`、`<service>-0.2.0`、`<service>-sha-<full-commit>` 和 `<service>-latest`，不会自动部署、切流或创建 GitHub Release。应为 `v*` 配置 Tag 保护，禁止强制移动或删除已发布版本。

Docker tag 均可移动。需要严格可复现的部署时，从工作流 Summary 或 GHCR Package 页面取得三个 manifest digest，并使用完整 `image@sha256:<digest>` 引用：

```env
TRADEMIND_BACKEND_IMAGE=ghcr.io/lien0219/trademind@sha256:<backend-manifest-digest>
TRADEMIND_ADMIN_IMAGE=ghcr.io/lien0219/trademind@sha256:<admin-manifest-digest>
TRADEMIND_COLLECTOR_IMAGE=ghcr.io/lien0219/trademind@sha256:<collector-manifest-digest>
```

P10 预生产使用同一组 backend/Admin digest：

```env
P10_API_IMAGE=ghcr.io/lien0219/trademind@sha256:<backend-manifest-digest>
P10_ADMIN_IMAGE=ghcr.io/lien0219/trademind@sha256:<admin-manifest-digest>
```

公开仓库首次生成统一 Package 后，维护者需要在该 Package 的设置中确认可见性为 Public；工作流只使用仓库自带的 `GITHUB_TOKEN` 写入 GHCR，不保存 PAT 或镜像仓库密码。若 Package 仍为私有，拉取前使用具有 `read:packages` 权限的凭据登录 `ghcr.io`。迁移前的三个旧 Package 不会被工作流自动删除，确认不再被部署引用后再由维护者在 GitHub Package 设置中清理。

## 默认访问地址

| 服务 | 地址 |
| --- | --- |
| Admin | `http://127.0.0.1:8000` |
| Backend Health | `http://127.0.0.1:8080/health` |

Collector 不发布宿主机端口，只允许 backend 通过 Compose 内部网络访问；PostgreSQL 与 Redis 的宿主机映射仅绑定 `127.0.0.1`。

## 端口配置

可在 `.env` 中覆盖以下端口：

```env
ADMIN_PUBLISH_PORT=8000
BACKEND_PUBLISH_PORT=8080
POSTGRES_PUBLISH_PORT=5432
REDIS_PUBLISH_PORT=6379
```

完整环境变量说明见 [env.md](env.md)。修改 Docker 变量时必须同步唯一模板 `.env.example`、`docker-compose.full.yml`、本文档和 `docs/env.md`。

AI 客服自动回复使用 backend 已有 Redis 服务和独立 ready/processing 队列。`.env` 仅保留队列名和 Worker 并发等基础设施参数；消息同步、自动回复总开关和轮询间隔在 Admin「客服 / AI 自动回复」中管理并默认关闭。上线前先检查 `/health` 的 `customerAutoReplyQueue`：Redis、客服消息同步 Worker、轮询调度器和自动回复消费者必须均可用，再验证平台客服 Provider 的收取与发送契约、AI Provider 和店铺授权，最后在页面开启总开关并逐店确认开启。

运营任务中心的真实抖店能力默认保持 `P10_CURRENT_ALLOWED_LEVEL=L0`。经独立发布工单批准的 L3 只允许人工审核后的 `save_as_platform_draft`：必须同时启用 P10 Provider/网络/凭据/草稿写/Worker 开关、`PRODUCT_PUBLISH_QUEUE_ENABLED=true` 和 `WORKER_REAPER_ENABLED=true`，自动业务重试与库存 mutation 必须关闭，并在生产控制面完成单租户、单白名单店铺、两名不同管理员分别承担 Owner/Technical Lead 审批、active 灰度和 provider/tenant/shop/write kill switch 放行。部署、迁移、备份恢复、回滚、CI、真实凭据和真实平台人工验收完成前，不得把代码部署描述为已上线。

P5-V 可观测性默认使用 `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`。Docker 本地试用不配置真实 telemetry backend 时，`OTEL_EXPORTER_OTLP_ENDPOINT` 保持为空并在 Admin 显示“未配置导出后端”；不要把 Mock Collector 验证写成生产 collector 已上线。

生产指标入口采用 Nginx 与 Gin 双层限制。`deploy/nginx/trademind.conf` 必须保留精确的 `location = /internal/metrics`，先写监控网段的 `allow`，最后 `deny all`；同一网段同步到 `METRICS_ALLOWLIST_CIDRS`。`HTTP_TRUSTED_PROXY_CIDRS` 只填写实际反向代理地址，不能填写全网段。修改后先执行 `nginx -t`，再从允许网段验证抓取成功，并从公网地址和伪造 `X-Forwarded-For` 请求验证返回 403。

P7 性能数据集与负载测试只能在隔离 `APP_ENV=performance` 环境执行；普通 Docker 试用与生产部署必须保持 `PERFORMANCE_TEST_MODE=false`、`ALLOW_PERFORMANCE_DATASET=false`，不得把隔离压测描述为真实生产容量验证。

## 安全配置

生产环境或公网部署前必须修改：

- `JWT_SECRET`
- `APP_MASTER_KEY`
- `ADMIN_BOOTSTRAP_PASSWORD`
- `COLLECTOR_SERVICE_TOKEN`（backend 与 Collector 使用同一个至少 32 字符的独立随机值）
- `POSTGRES_PASSWORD`
- `DB_PASSWORD`
- 所有第三方平台、AI、存储、Webhook、邮箱等密钥
- `HTTP_TRUSTED_PROXY_CIDRS` 与 `METRICS_ALLOWLIST_CIDRS`（按实际代理和监控网段收敛）

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

**1688 采集浏览器 Profile**：`docker-compose.full.yml` 使用 `trademind_full_collector_profiles` 与 `trademind_full_collector_storage_states` 命名卷持久化 1688 登录 Cookie（含 Login Data、Cookies、History、Local Storage、Session Storage 等 Chromium 用户数据），Collector 以 Playwright 镜像内置的非 root 用户运行。卷数据**必须纳入备份、禁止提交 Git**；本地 `collector/data/browser-profiles/` 同理。容器内默认无图形界面，首次登录建议在宿主机本地运行 collector（`COLLECTOR_HEADLESS=0`）并按受控流程把 Profile 导入命名卷；或在已配置远程桌面的 Linux 服务器上打开登录浏览器。

Collector 的 `/health` 保持无鉴权供容器编排探活，其余接口在 Token 缺失时返回 503、Token 不匹配时返回 401。采集目标只允许公网 HTTP/HTTPS 地址；私网、回环、链路本地、保留地址、DNS 解析结果和浏览器 WebSocket 请求都会在访问前复核。生产仍需用容器出站策略限制 Collector 只能访问业务所需公网目标。

两套 Compose 的服务、端口和数据卷应分开理解。

## 配置校验

CI 会执行轻量 Docker 配置检查：

```bash
bash scripts/release/validate-image-version.sh
docker compose -f docker-compose.full.yml config
```

本地修改镜像版本、Dockerfile、Compose 或 `.env.example` 后，建议先执行同样命令确认版本格式、语法和变量引用正确。真实多架构构建与 GHCR 推送由 `Container Images` 工作流执行。
# P10 Independent Pre-production

P10 maps pre-production to the existing `staging` profile and uses the separate `trademind-preproduction` Compose project. It does not reuse `docker-compose.yml`, `docker-compose.full.yml`, or production resources.

```bash
cp .env.example .env
# edit .env: APP_ENV=staging and fill this host's non-secret identifiers
node scripts/preproduction-preflight.mjs --mode config
deploy/scripts/deploy-preproduction.sh
```

Inject `PREPRODUCTION_DB_PASSWORD`, `PREPRODUCTION_REDIS_PASSWORD`, `PREPRODUCTION_APP_MASTER_KEY`, `PREPRODUCTION_JWT_SECRET`, `P10_API_IMAGE`, and `P10_ADMIN_IMAGE` from the target host or managed secret source. The repository contains references and placeholders only.

The deployment waits for PostgreSQL and Redis health, starts the backend migration path with its advisory lock, and accepts the deployment only after `/health/ready` reports database, Redis, migrations, and `staging` as ready. Backup, isolated restore, application rollback, and non-destructive teardown entry points are under `deploy/scripts/*-preproduction.sh`.

External infrastructure status must be supplied at runtime to `node scripts/preproduction-preflight.mjs --mode external`; generated evidence JSON is not retained in the working tree. Until host, PostgreSQL, Redis, domain, credential availability, deployment rehearsal, and teardown rehearsal are all proven, pre-production remains blocked.

The full-stack development Compose explicitly passes the P10 L0 variables listed in [`env.md`](env.md). It rejects non-L0 and all real Provider/network/credential/read, mutation, Worker, and automatic-retry flags. It is only suitable for repository-side/manual fixture checks and must not be treated as the independent pre-production environment.

P10 reuses the existing recovery foundations instead of creating parallel mechanisms: `deploy/scripts/backup-preproduction.sh` creates a PostgreSQL custom-format artifact, SHA-256 checksum and metadata; `restore-preproduction.sh` restores only into an explicit isolated database identity; `rollback-preproduction.sh` restores previous immutable application images and performs readiness checks without an implicit database restore. Production restore remains disabled by default.
