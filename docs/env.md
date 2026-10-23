# 环境变量说明

Bootstrap 管理账号：`ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PHONE` 指定恢复身份，后端启动时保持管理员与启用状态，并对恢复变更作事务审计及安全版本更新。普通运行环境不会用 `.env` 新密码覆盖已有数据库密码；修改密码使用用户管理的重置与首次改密流程。公开自助注册已关闭，新控制台账号由管理员创建。仅在授权的隔离测试中配置 `TEST_DATABASE_URL` / `TEST_REDIS_URL`，测试不可回退开发或生产资源。

本文件是唯一配置模板 `.env.example` 的说明索引。`.env` 是唯一运行配置并保持 Git ignored；新增、删除或重命名环境变量时，必须同步更新本文件，并检查 `docs/module-map.md` 中的关联项。

## 使用方式

本地开发：

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

Docker 完整部署也使用同一模板：

```bash
cp .env.example .env
docker compose -f docker-compose.full.yml up -d --build
```

环境由 `APP_ENV` 选择。进程环境变量优先于 `.env`，CI、容器平台和 Secret Store 可安全覆盖模板值；禁止创建 `.env.local`、`.env.test.local`、`.env.staging`、`.env.production` 等变体。

## 安全规则

- 不提交 `.env`。
- 生产环境必须替换 `JWT_SECRET`、`APP_MASTER_KEY`、`ADMIN_BOOTSTRAP_PASSWORD`、数据库密码。
- AI API Key、平台 Secret、Access Token、Refresh Token、Webhook Secret 不应写入环境模板，优先通过后台 settings 加密保存。
- 日志不得输出完整密钥、Token、Cookie 或密码。

## 后端基础配置

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `APP_ENV` | `development` | backend | 否 | 主运行环境使用 `development` / `test` / `staging` / `production`；既有 `demo` / `performance` 仅用于受控演示和隔离压测。 |
| `APP_HTTP_ADDR` | `:8080` | backend | 否 | Go API 监听地址。 |
| `P7_DIAGNOSTICS_*` | `P7_DIAGNOSTICS_ENABLED=false` | legacy diagnostics compatibility | 否 | 默认关闭；如临时诊断必须写入仓库外临时目录，完成后删除，不得创建或提交 `artifacts/`。 |
| `APP_MASTER_KEY` | 空 / 64 位 hex | backend | 是 | AES-GCM 主密钥，用于 settings 敏感配置加密。 |
| `ADMIN_BOOTSTRAP_EMAIL` | 空 / `admin@example.com` | backend | 否 | 初始管理员邮箱。 |
| `ADMIN_BOOTSTRAP_PHONE` | 空 | backend | 否 | 初始管理员手机号。 |
| `ADMIN_BOOTSTRAP_PASSWORD` | 空 / 示例密码 | backend | 是 | 初始管理员密码，生产必须强密码。 |
| `JWT_SECRET` | `change-me-in-production` | backend | 是 | JWT 签名密钥。 |
| `JWT_EXPIRE_HOURS` | `168` | backend | 否 | JWT 有效期小时数。 |
| `UPLOAD_MAX_MB` | `10` | backend | 否 | 单文件上传大小上限。 |

## 可观测性与 OTLP

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `OBSERVABILITY_ENABLED` | `true` | backend | 否 | 是否启用日志、指标、追踪等可观测性基础能力。 |
| `OBSERVABILITY_MODE` | `local` / `hybrid` | backend | 否 | 本地、Prometheus、OTel 或混合模式。 |
| `OBSERVABILITY_ENVIRONMENT` | `development` | backend | 否 | 低基数环境标签。 |
| `METRICS_ENABLED` | `true` | backend | 否 | 是否注册并暴露 Prometheus 指标。生产环境必须开启。 |
| `METRICS_PATH` | `/internal/metrics` | backend | 否 | 内部指标路径；部署层必须使用精确路径匹配，禁止经通用 API 代理公开。 |
| `METRICS_INTERNAL_ONLY` | `true` | backend | 否 | 是否启用应用层 CIDR 访问控制；生产环境必须为 `true`。 |
| `HTTP_TRUSTED_PROXY_CIDRS` | `127.0.0.1/32,::1/128` | backend | 否 | Gin 可信反向代理 CIDR。只配置实际代理地址；staging/production 拒绝无效 CIDR 与 `0.0.0.0/0`、`::/0`。 |
| `METRICS_ALLOWLIST_CIDRS` | `127.0.0.1/32,::1/128` | backend | 否 | 允许抓取指标的客户端 CIDR，需与 Nginx `location = /internal/metrics` 的 allowlist 一致。 |
| `TRACING_ENABLED` | `false` | backend | 否 | 是否启用 tracing。真实 OTLP backend 未配置时保持 `false` 或本地 Mock 验证。 |
| `OTEL_SERVICE_NAME` | `trademind-api` | backend | 否 | OTLP resource `service.name`。 |
| `OTEL_SERVICE_VERSION` | 空 | backend | 否 | OTLP resource `service.version`。 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 空 | backend | 否 | OTLP/HTTP endpoint；代码会规范化为 `/v1/traces`。为空表示真实 telemetry backend 验证 Deferred。 |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/json` | backend | 否 | P5-V 使用标准 OTLP/HTTP JSON。 |
| `OTEL_EXPORTER_OTLP_HEADERS` | 空 | backend | 是 | 可选 header 列表，格式 `k=v,k2=v2`；不得提交真实 Token，日志不得输出。 |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` / `false` | backend | 否 | 本地 Mock Collector 可为 `true`；生产建议 `false`。 |
| `OTEL_TRACE_SAMPLE_RATIO` | `0` / `0.1` | backend | 否 | 采样比例，生产上限由配置校验限制。 |
| `OTEL_EXPORT_TIMEOUT_SECONDS` | `10` | backend | 否 | 单次导出超时，代码限制最大 30 秒。 |
| `OTEL_EXPORT_QUEUE_SIZE` | `1024` | backend | 否 | OTel batcher 有界队列大小。 |
| `OTEL_EXPORT_BATCH_SIZE` | `128` | backend | 否 | 单批导出数量，不得超过队列大小。 |
| `OTEL_EXPORT_RETRY_MAX` | `2` | backend | 否 | 429/5xx 受控重试次数，上限 5。 |
| `ALERTING_ENABLED` | `true` | backend | 否 | 是否启用内部告警规则评估与投递重试；生产环境必须为 `true`。 |
| `ALERT_DEFAULT_COOLDOWN_SECONDS` | `300` | backend | 否 | 同一告警指纹的默认冷却时间。 |
| `ALERT_RECOVERY_ENABLED` | `true` | backend | 否 | 是否在告警恢复时记录恢复通知。 |

## Webhook 入站（公开 HTTP）

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `WEBHOOK_MAX_BODY_KB` | `512` | backend | 否 | `POST /api/v1/webhooks/:platform/:eventType` 请求体上限（KiB）。 |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | `300` | backend | 否 | 允许的时间戳时钟偏差；超时或远未来时间戳返回 `WEBHOOK_TIMESTAMP_EXPIRED`。 |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | `false` | backend | 否 | 启用 `internal-test` HMAC-SHA256 测试验签；**仅** `APP_ENV=development` / `test` 生效，production 强制关闭。 |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | `3` | backend | 否 | DB 轮询 `webhook_events.status=queued` 的间隔秒数。 |
| `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` | 空 | backend | 否 | P3.2 多店铺 Webhook 显式测试兜底绑定 ID；仅 `development` / `test` 生效，`staging` / `production` 配置后 fail-fast。 |
| `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` | `false` | backend | 否 | Demo 环境是否允许使用显式 Webhook 兜底绑定；`staging` / `production` 必须为 `false`。 |

## 数据库

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `DB_DRIVER` | `postgres` | backend | 否 | 默认 PostgreSQL；仅遗留库或明确要求时用 MySQL。 |
| `DB_HOST` | `127.0.0.1` / `postgres` | backend | 否 | 数据库地址。 |
| `DB_PORT` | `5432` | backend | 否 | PostgreSQL 默认 5432。 |
| `DB_USER` | `trademind` | backend | 否 | 数据库用户。 |
| `DB_PASSWORD` | 明显占位值 | backend | 是 | 数据库密码；真实值只写入 `.env` 或 Secret Store。 |
| `DB_NAME` | `trademind` | backend | 否 | 数据库名。 |
| `DB_TIMEZONE` | `UTC` | backend | 否 | 数据库时区。 |
| `DB_MAX_OPEN_CONNECTIONS` | `100` | backend | 否 | P7 数据库连接池最大打开连接数；生产非法配置 fail-fast。 |
| `DB_MAX_IDLE_CONNECTIONS` | `10` | backend | 否 | P7 数据库连接池最大空闲连接数。 |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `3600` | backend | 否 | P7 单连接最大生命周期。 |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `900` | backend | 否 | P7 空闲连接最长保留时间。 |
| `DB_QUERY_TIMEOUT_MS` | `5000` | backend | 否 | P7 查询超时预算；逐步接入仓储查询。 |
| `DB_TRANSACTION_TIMEOUT_MS` | `10000` | backend | 否 | P7 事务超时预算；必须大于等于查询超时。 |
| `POSTGRES_DB` | `trademind` | docker postgres | 否 | Docker Postgres 初始化库名。 |
| `POSTGRES_USER` | `trademind` | docker postgres | 否 | Docker Postgres 用户。 |
| `POSTGRES_PASSWORD` | 示例密码 | docker postgres | 是 | Docker Postgres 密码。 |

## Redis

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `REDIS_ADDR` | `127.0.0.1:6379` / `redis:6379` | backend | 否 | Redis 地址。 |
| `REDIS_PASSWORD` | 空 | backend | 是 | Redis 密码。 |
| `REDIS_DB` | `0` | backend | 否 | Redis DB 编号。 |

## Collector

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `COLLECTOR_BASE_URL` | `http://127.0.0.1:3100` | backend | 否 | Go API 调用 Collector 的基础地址。 |
| `COLLECTOR_TIMEOUT_SECONDS` | `120` | backend | 否 | 后端调用 Collector 超时；淘宝/天猫任务会按页面打开超时自动放宽（约 `gotoTimeoutMs + 90s`）。 |
| `COLLECTOR_SERVICE_TOKEN` | 空 | backend / collector | 是 | 后端与 Collector 共用的内部 Bearer Token。Collector 除 `/health` 外的接口均要求该 Token；production 必须使用至少 32 字符的独立随机值，完整 Compose 启动时必须显式提供。 |
| `COLLECTOR_HTTP_ADDR` | `:3100` / `:3001` | collector | 否 | Collector 监听地址。 |
| `COLLECTOR_MAX_REQUEST_BODY_BYTES` | `1048576` | collector | 否 | Collector JSON 请求体上限，允许 1 KiB 至 16 MiB；非法配置回退为 1 MiB。 |
| `COLLECTOR_REQUEST_TIMEOUT_MS` | `15000` | collector | 否 | HTTP 请求接收超时，允许 1-120 秒；不限制 Playwright 页面采集执行时长。 |
| `COLLECTOR_HEADERS_TIMEOUT_MS` | `10000` | collector | 否 | HTTP 请求头接收超时，允许 1-60 秒。 |
| `COLLECTOR_GOTO_TIMEOUT_MS` | `45000` | collector | 否 | Playwright 页面打开超时。 |
| `COLLECTOR_HEADLESS` | `1` | collector | 否 | 是否无头浏览器运行；本地打开登录浏览器时可设为 `0`。 |
| `COLLECTOR_BROWSER_PROFILE_DIR` / `BROWSER_PROFILE_ROOT` | `collector/data/browser-profiles`（相对 collector 包根目录） | collector | 否 | 1688 持久化 Profile 根目录（1688 使用子目录 `1688`）。Docker 通常设为 `/workspace/data/browser-profiles`。 |
| `COLLECTOR_STORAGE_STATE_DIR` | `data/storage-states` | collector | 否 | Playwright storageState 导出目录（预留）。 |
| `COLLECTOR_1688_AUTH_PROBE_URL` | 注释示例 | collector | 否 | 登录态检测时用于探测的商品详情 URL。 |
| `COLLECTOR_USER_AGENT` | 注释示例 | collector | 否 | 可选 UA 覆盖。 |

本地 `pnpm dev` 会从根 `.env`（不存在时从 `backend/.env`）读取 Collector 配置；进程环境变量优先。若 `COLLECTOR_SERVICE_TOKEN` 为空，开发启动器只为当前 backend/collector 会话生成随机 Token，不会写回配置文件；单独启动两个服务时必须显式提供相同 Token。

## 队列与任务

| 变量前缀 | 示例变量 | 服务 | 说明 |
| --- | --- | --- | --- |
| `COLLECT_*` | `COLLECT_QUEUE_ENABLED`、`COLLECT_WORKER_CONCURRENCY`、`COLLECT_QUEUE_NAME`、`COLLECT_BATCH_MAX_URLS`、`COLLECT_BATCH_CONCURRENCY_1688`、`COLLECT_BATCH_DELAY_MIN_MS_1688`、`COLLECT_BATCH_DELAY_MAX_MS_1688`、`COLLECT_BATCH_RETRY_ON_BLOCKED`、`COLLECT_BATCH_RETRY_ON_TIMEOUT`、`COLLECT_BATCH_MAX_RETRIES_1688` | backend | 采集任务队列、批量 URL 限制、1688 批量节流与重试。settings **`collector`** 分组可覆盖 1688 批量项。 |
| `IMAGE_*` | `IMAGE_QUEUE_ENABLED`、`IMAGE_TASK_TIMEOUT_SECONDS` | backend | 图片任务队列与单次任务 Provider 调用超时。 |
| `TRANSLATE_FONT_PATH` | — | backend | 可选。图片文字翻译程序绘制所用字体（TTF/TTC）；未设置时自动查找 Noto CJK / 微软雅黑 / 内置英文字体。Docker 镜像默认安装 `fonts-noto-cjk`。 |
| `ORDER_SYNC_*` | `ORDER_SYNC_QUEUE_ENABLED`、`ORDER_SYNC_QUEUE_NAME` | backend | 平台订单同步任务。 |
| `CUSTOMER_MESSAGE_SYNC_*` | `CUSTOMER_MESSAGE_SYNC_QUEUE_NAME`、`CUSTOMER_MESSAGE_SYNC_WORKER_CONCURRENCY`、`CUSTOMER_MESSAGE_SYNC_TASK_TIMEOUT_SECONDS` | backend | 客服消息同步 Redis 队列基础设施；自动同步开关在 Admin「客服 / AI 自动回复」中管理。 |
| `CUSTOMER_AUTO_REPLY_*` | `CUSTOMER_AUTO_REPLY_QUEUE_NAME`（默认 `customer:auto:reply:tasks`）、`CUSTOMER_AUTO_REPLY_WORKER_CONCURRENCY`（默认 `1`） | backend | AI 客服自动回复的独立 Redis 队列基础设施。总开关和轮询间隔改由 Admin 页面持久化管理，默认关闭；仅低风险消息可自动发送，失败不自动重试。 |
| `PRODUCT_PUBLISH_*` | `PRODUCT_PUBLISH_QUEUE_ENABLED`、`PUBLISH_BATCH_MAX_PRODUCTS`（100）、`PUBLISH_BATCH_MAX_TARGETS`（20）、`PUBLISH_BATCH_MAX_TASKS`（300） | backend | 商品刊登任务队列与批量矩阵上限。L3 抖店草稿写要求 `PRODUCT_PUBLISH_QUEUE_ENABLED=true`。 |
| `INVENTORY_SYNC_*` | `INVENTORY_SYNC_QUEUE_ENABLED` | backend | 库存同步任务。 |
| `WORKER_*` | `WORKER_HEARTBEAT_ENABLED`、`WORKER_REAPER_ENABLED` | backend | 多实例 Worker 心跳、过期判断和回收。L3 抖店草稿写要求 `WORKER_REAPER_ENABLED=true`，以便进程中断后将未知平台结果转入人工核对。 |
| `TASK_ALERT_*` | `TASK_ALERT_SCAN_ENABLED`、`TASK_ALERT_SCAN_INTERVAL_SECONDS` | backend | 任务告警扫描。 |
| `PERFORMANCE_*` | `PERFORMANCE_TEST_MODE`、`PERFORMANCE_DATASET_MAX_ROWS`、`PERFORMANCE_TEST_MAX_VUS`、`PERFORMANCE_TEST_MAX_DURATION_SECONDS` | backend / scripts | P7 隔离性能测试与数据集保护；production 禁止开启测试模式。 |
| `PAGINATION_*` | `PAGINATION_DEFAULT_LIMIT`、`PAGINATION_MAX_LIMIT`、`PAGINATION_MAX_OFFSET`、`PAGINATION_CURSOR_SIGNING_KEY` | backend | P7 列表分页默认值、最大 limit、深 offset 保护与 Cursor HMAC 签名密钥；production 必须显式配置签名密钥。 |
| `RATE_LIMIT_*` | `RATE_LIMIT_ENABLED`、`RATE_LIMIT_MODE`、`RATE_LIMIT_FAIL_MODE`、`RATE_LIMIT_POLICY_VERSION` | backend | P7 HTTP 限流配置；production 禁用需显式审批变量。 |
| `CACHE_*` | `CACHE_ENABLED`、`CACHE_DEFAULT_TTL_SECONDS`、`CACHE_MAX_ENTRIES`、`CACHE_SINGLEFLIGHT_ENABLED` | backend | P7 缓存与 singleflight 治理基础配置。 |
| `EXPORT_*` | `EXPORT_BATCH_SIZE`、`EXPORT_MAX_ROWS`、`EXPORT_MAX_BYTES`、`EXPORT_MAX_CONCURRENT` | backend | P7 导出批量、行数、字节数和并发上限。 |
| `PPROF_*` | `PPROF_ENABLED`、`PPROF_INTERNAL_ONLY` | backend | P7 Profiling 安全开关；production 禁止 public pprof。 |

新增队列变量时，还要同步健康检查说明、任务中心页面和 `docs/PROGRESS.md`。

数据库自动备份、加密、保留、PITR、备份告警和恢复演练由云数据库与运维平台配置，不再通过 TradeMind backend 环境变量管理。仓库内 P10 预生产备份/隔离恢复目标变量只服务于外部部署级发布验收，不会恢复已退役的应用内管理能力。

## Docker 镜像与端口

完整 Compose 默认构建并使用 `local/trademind-*:local`。切换到 GitHub Actions 发布的预构建镜像时设置：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `TRADEMIND_BACKEND_IMAGE` | `local/trademind-backend:local` | backend 完整镜像引用；受控部署使用 `image@sha256:<manifest-digest>`。 |
| `TRADEMIND_ADMIN_IMAGE` | `local/trademind-admin:local` | admin 完整镜像引用；受控部署使用 `image@sha256:<manifest-digest>`。 |
| `TRADEMIND_COLLECTOR_IMAGE` | `local/trademind-collector:local` | collector 完整镜像引用；受控部署使用 `image@sha256:<manifest-digest>`。 |

这三个变量只供 Docker Compose 解析镜像地址，不会作为业务配置传入容器。GHCR 标签、manifest digest 和版本来源见 [Docker 部署说明](docker-deployment.md#ghcr-预构建镜像)。

生产部署使用 `deploy/production/compose.yml`，不提供本地镜像默认值，也不执行 Docker build。三个 `TRADEMIND_*_IMAGE` 变量必须是同一份 Release 清单中的 `ghcr.io/lien0219/trademind@sha256:<manifest-digest>`；生产校验脚本会拒绝旧 Package、`local/*` 和可移动标签。数据库、Redis、上传文件和 Collector 浏览器状态使用生产专用命名卷，密码、JWT、主密钥和 Collector Token 只在运行时注入。

`.env.example` 支持以下宿主机端口覆盖：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `ADMIN_PUBLISH_PORT` | `8000` | 管理端宿主机端口。 |
| `BACKEND_PUBLISH_PORT` | `8080` | 后端 API 宿主机端口。 |
| `POSTGRES_PUBLISH_PORT` | `5432` | PostgreSQL 宿主机回环端口，仅绑定 `127.0.0.1`。 |
| `REDIS_PUBLISH_PORT` | `6379` | Redis 宿主机回环端口，仅绑定 `127.0.0.1`。 |

完整 Compose 不发布 Collector 宿主机端口；backend 仅通过 Compose 内部网络的 `http://collector:3001` 访问。Collector 容器以非 root 用户运行，浏览器 Profile 与 Storage State 分别保存在 `trademind_full_collector_profiles`、`trademind_full_collector_storage_states` 命名卷中。需要本地交互式登录时，单独启动本地 Collector 并使用与本地 backend 一致的 `COLLECTOR_SERVICE_TOKEN`。

## 新增变量检查清单

新增或修改环境变量时必须检查：

- `.env.example`
- `docker-compose.yml`
- `docker-compose.full.yml`
- `docs/env.md`
- `docs/development.md`
- `docs/docker-deployment.md`
- `README.md` / `README.en.md` 中的启动说明
- 相关代码默认值与安全校验
# P10 Pre-production Contract

P10 reuses `APP_ENV=staging` as the only pre-production profile. Do not introduce a second `preproduction` runtime value. `.env.example` is the canonical non-secret contract; the target host copies it to `.env`, while database, Redis, application-key, JWT, and immutable-image values are injected externally.

For pre-production, copy `.env.example` to `.env`, set `APP_ENV=staging`, and fill the target host's non-secret identities. The P10 preflight requires explicit, pairwise-distinct identities for development/test, pre-production, and production database and Redis resources. It also requires a distinct session namespace, non-overlapping cookie domains, distinct Admin/API endpoints, a non-local staging storage mode, a matching credentialed CORS origin, explicit migration/backup/restore targets, previous immutable images, and external secret references. Inline secret values, missing or unknown environments, and production targets fail closed.

`P10_PRODUCTION_RESTORE_ENABLED` must remain `false`. All real Provider/network/read/write, mutation, queue/worker, and automatic business retry flags remain disabled at L0. L3 exists only as an externally approved single-tenant, single-allowlisted-shop Douyin platform-draft write profile; it does not allow publishing online, inventory mutation, automatic business retry, unreviewed execution, or multi-shop expansion. Run the non-secret contract check with:

```bash
node scripts/preproduction-preflight.mjs --mode config
```

Operational values must be supplied by the target host or managed secret system and must never be committed or printed in evidence.

历史阶段报告与冻结证据已从当前工作树清理；需要追溯时使用 Git 历史。当前运行说明只以 `.env`、`.env.example` 和本文件为准。

## 测试资源

- `TEST_DATABASE_URL` 与 `TEST_REDIS_URL` 只用于显式隔离的测试资源或 CI service container，不属于生产运行配置。
- 本地测试数据库可不存在；项目不会自动创建或重建 `trademind_test`。
- GitHub Actions 可以在作业内临时创建同名数据库，作业结束后随 service container 销毁。
- 未提供安全隔离 URL 时，不在本地运行对应集成测试，且绝不回退到开发库或生产资源。

## Repository-side Runtime Controls

The project is in production maintenance, while repository runtime controls remain fail closed at `L0` unless separately approved and activated externally. The following variables are present in `.env.example` and the backend service in `docker-compose.full.yml`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `P10_CURRENT_ALLOWED_LEVEL` | `L0` | Runtime capability level. Repository default is L0; L3 is accepted only for the separately approved platform-draft write profile. |
| `P10_OFFLINE_OAUTH_ENABLED` | `false` | Enables development/test-only offline OAuth fixtures. Forbidden in staging/production. |
| `P10_LOCAL_CREDENTIAL_KEY` | empty | Development/test-only local key material. Never commit a value; forbidden in staging/production. |
| `P10_LOCAL_CREDENTIAL_KEY_REF` | `local-development-v1` | Non-secret local key reference. |
| `P10_OAUTH_STATE_TTL_SECONDS` | `600` | Single-use OAuth state lifetime; valid range 60-1800 seconds. |
| `P10_OAUTH_REDIRECT_ALLOWLIST` | empty | Comma-separated exact redirect URI allowlist. |
| `P10_DOUYIN_API_BASE_URL` | empty | Trusted config only; when present it must be official HTTPS host `openapi-fxg.jinritemai.com`. |
| `P10_PROVIDER_REQUEST_TIMEOUT_SECONDS` | `30` | Whole provider request timeout. |
| `P10_PROVIDER_CONNECT_TIMEOUT_SECONDS` | `5` | Provider connection/TLS timeout foundation. |
| `P10_PROVIDER_RESPONSE_HEADER_TIMEOUT_SECONDS` | `15` | Provider response-header timeout. |
| `P10_PROVIDER_MAX_RESPONSE_BYTES` | `2097152` | Strict response body limit. |
| `P10_PROVIDER_CONCURRENCY` | `2` | Per-host connection/concurrency bound. |
| `P10_SKU_PAGE_SIZE` | `50` | Local publication page size, capped at 100. |
| `P10_PAGINATION_LIMIT` | `100` | Maximum pages per manual read run. |
| `P10_REAL_PROVIDER_ENABLED` | `false` | Real Provider feature flag; rejected when true at L0. |
| `P10_REAL_PLATFORM_NETWORK_ENABLED` | `false` | Real network feature flag; rejected when true at L0. |
| `P10_REAL_CREDENTIALS_ENABLED` | `false` | Real credential feature flag; rejected when true at L0. |
| `P10_REAL_INVENTORY_READ_ENABLED` | `false` | Real read feature flag; rejected when true at L0. |
| `P10_REAL_PRODUCT_DRAFT_WRITE_ENABLED` | `false` | Allows only reviewed Douyin `save_as_platform_draft` writes at L3. It never permits online publish or inventory mutation. |
| `P10_INVENTORY_MUTATION_ENABLED` | `false` | Inventory mutation guard; must remain false. |
| `P10_BACKGROUND_WORKER_ENABLED` | `false` | P10 production Worker guard; required only for the approved L3 platform-draft profile. |
| `P10_AUTOMATIC_RETRY_ENABLED` | `false` | Automatic business retry guard. It must remain false for real platform-draft writes. |

The committed template remains L0 and fail closed. A release work order may set L3 only after CI, backup/restore/rollback rehearsal, real-platform acceptance and two different administrators acting as Owner and Technical Lead have approved the same gray scope. L3 startup additionally requires the Provider/network/credential/draft-write/Worker flags, `PRODUCT_PUBLISH_QUEUE_ENABLED=true`, `WORKER_REAPER_ENABLED=true`, an enabled one-tenant/one-shop allowlist, an active gray policy, and provider/tenant/shop/write kill switches all released. `P10_AUTOMATIC_RETRY_ENABLED` and `P10_INVENTORY_MUTATION_ENABLED` remain false. Configuration alone does not mean the feature is live; the database controls and runtime guards are re-evaluated before every platform call.
