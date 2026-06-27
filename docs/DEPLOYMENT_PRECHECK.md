# 部署前检查清单（Phase R1 Demo Release）

> **目标状态**：`MVP Demo Ready`（非 Production Ready）

## 1. 环境变量

- [ ] 复制 `.env.example` → `.env`，填写 `APP_MASTER_KEY`、`JWT_SECRET`
- [ ] `DB_DRIVER=postgres`，`DB_*` 指向可用 PostgreSQL
- [ ] `REDIS_*` 可用
- [ ] `ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PASSWORD` 已设（生产勿用弱密码）
- [ ] AI Provider：`AI_PROVIDER`、API Key（加密存储）
- [ ] 图片 Provider（可选）：`IMAGE_PROVIDER` 等
- [ ] 批量刊登上限：`PUBLISH_BATCH_MAX_*`（默认 100/20/300）
- [ ] Storage：`STORAGE_PROVIDER=local` 或云存储 + 公网 `public_base`

详见 [`docs/env.md`](env.md)

## 2. 数据库

- [ ] `docker compose up -d` 或外部 Postgres 就绪
- [ ] 后端启动自动 migration 无报错
- [ ] `product_publish_batches` 相关 migration 已执行（见 [`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)）
- [ ] migration 可重复启动（无 duplicate 致命错误）

## 3. 路由 Smoke

```powershell
.\scripts\demo-route-smoke.ps1
# 期望 docs/demo-route-smoke.json passed=true，无 404
```

- [ ] `/health` 200
- [ ] 已登录核心 API 200（见 `docs/demo-route-smoke.json`）
- [ ] 未登录 protected 路由 401/403

分项 smoke（可选）：

- [ ] `scripts/ai-text-route-smoke.ps1`
- [ ] `scripts/ai-image-route-smoke.ps1`

## 4. Redis / Worker

- [ ] Redis 连接正常（`/health` 队列块）
- [ ] 抖店刊登 worker：`PRODUCT_PUBLISH_QUEUE_ENABLED=true`（生产推荐）
- [ ] 采集 / 图片 worker 按需启动

## 5. Storage

- [ ] 本地上传目录可写
- [ ] 公网部署：`POST /api/v1/storage/test-public-access` 通过（抖店图片上传依赖）

## 6. Provider 配置

- [ ] 设置 → AI：连接测试通过
- [ ] 设置 → 图片 AI：Provider 与 OCR（译图）按需
- [ ] 设置 → 平台开放配置：抖店 App Key/Secret（演示可留空）

## 7. 网络与安全

- [ ] CORS：`CORS_ALLOWED_ORIGINS` 含管理端域名
- [ ] Nginx / HTTPS（预发/生产）
- [ ] 日志目录挂载，**不**输出完整 API Key / Token
- [ ] `safedownload` SSRF 防护：`go test ./internal/pkg/safedownload/...`

## 8. 备份与回滚

- [ ] Postgres 定期备份策略
- [ ] 回滚：保留上一版后端二进制 + admin `dist`；数据库 migration 回滚按 [`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md) 演练（抖店仍为 RC）

### Phase R1.1 本地预发等价环境记录（2026-06-27）

| 项 | 命令 / 路径 | 结果 |
| --- | --- | --- |
| 数据库备份 | `pg_dump -h 127.0.0.1 -U trademind -d trademind -F c -f backup/trademind-$(date +%Y%m%d).dump` | 待预发 systemd 环境执行 |
| Admin 静态备份 | 复制 `admin/dist` → `backup/admin-dist-$(date +%Y%m%d)/` | 本轮 `pnpm build:admin` 产物在 `admin/dist` |
| 后端二进制备份 | 复制 `backend/tmp/server.exe` → `backup/server-$(date +%Y%m%d)` | 本轮已构建 `backend/tmp/server.exe` |
| systemd 回滚 | `systemctl stop trademind-api && cp backup/server-* /opt/trademind/server && systemctl start trademind-api` | 预发 Linux 适用；本地 dev 用 `pnpm dev` 重启 |
| Nginx 配置备份 | `cp /etc/nginx/sites-available/trademind.conf backup/nginx-trademind-$(date +%Y%m%d).conf` | 本地 dev 无 Nginx；预发需人工勾选 |
| migration 回滚 | GORM AutoMigrate 无自动 down；按 [`PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md) / [`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md) 手工 SQL | 优先保留 DB 快照 |
| Redis 任务 | 队列 depth 见 `/health`；异常时可 `DEL` 对应 queue key 并重启 worker | 本轮 depth=0 |
| 日志路径 | 后端 stdout（dev）；生产挂载 `LOG_DIR` 或 journald | 启动日志无完整 Key/Token |

## 9. 构建验证

```bash
cd backend && go test ./... && go build ./cmd/server/...
pnpm build:admin
git diff --check
```

## 10. Release 标签建议

- Demo：`v0.9.0-mvp-demo-ready`
- **不要**标记 Production Ready，直至抖店真实 E2E + 灰度观察通过（见 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)）

## Phase R1.1 预发部署验收（2026-06-27）

| 检查项 | 结果 | 备注 |
| --- | --- | --- |
| `.env` 与 `.env.example` | ⚠️ 部分 | 本地 `.env` 缺 10 个可选键（后端有默认值）；核心 `DB_*` / `REDIS_*` / `APP_MASTER_KEY` 已设 |
| PostgreSQL + migration | ✅ | `/health` → `database: ok` |
| Redis + Worker | ✅ | 9 worker running；`product_publish` / `image` / `task_alert_scan` 正常 |
| Storage public access | ⚠️ 未测 | 本地 `STORAGE_PROVIDER=local`；抖店 E2E 需预发公网 `public_base` |
| AI 文案 Provider | ✅ | 文案复核对比弹窗可用；应用后工作台待办 758→753 |
| 图片 Provider | ⚠️ | 试跑 `passed_with_warning`（dashscope 白底图 Key 可选） |
| 抖店凭证 | ✅ RC | 无真实凭证；页面显示 Release Candidate / 待凭证 |
| Nginx / HTTPS | ⏭️ | 本轮为本地 dev（`:8080` API + `:8000` Admin）；预发需人工 |
| 路由 smoke | ✅ | `docs/demo-route-smoke.json` passed=true，8 路由无 404 |
| Demo 数据 | ✅ | `seed-demo-data.ps1` → 20 slot + 7 task samples |
| go test / build | ✅ | `go test ./...` + `pnpm build:admin` 通过 |
| Git tag | Tag pending | 建议 `v0.1.0-demo`；待预发 Nginx/HTTPS 人工勾选后打 tag |

## Phase R1.2 真实预发部署验收（2026-06-27）

> **环境说明**：仓库未配置真实预发机 SSH / HTTPS 域名；本机 **Docker 不可用**（`docker` 命令未安装），无法启动 `docker-compose.full.yml` 做 Nginx 代理模拟。本轮在 **本地 dev 等价环境** 复跑构建、smoke、Demo 数据与浏览器点检；**非 Production Ready**；**抖店仍 Release Candidate**。

| 检查项 | 结果 | 备注 |
| --- | --- | --- |
| 后端二进制 | ✅ | `backend/tmp/server.exe`（`go build -o tmp/server.exe ./cmd/server/...`） |
| Admin dist | ✅ | `admin/dist/`（`pnpm build:admin` 2026-06-27） |
| 真实预发 API HTTPS | ⏭️ 阻塞 | 无预发域名 / SSH；待运维提供后部署 |
| 真实预发 Admin HTTPS | ⏭️ 阻塞 | 同上 |
| Nginx proxy_pass | ⏭️ | 参考 `admin/nginx.conf`（Docker admin 服务）；本地 dev 由 Umi proxy 代管 |
| HTTP→HTTPS 跳转 | ⏭️ | 需真实预发证书 |
| 大请求体 / 超时 | ⏭️ | 预发 Nginx 需人工勾选 `client_max_body_size` / `proxy_read_timeout` |
| Admin SPA fallback | ✅ | dev 深链 `/ai/operation-workbench`、`/ops/task-center/failures` 可访问 |
| `/health` | ✅ | database=ok, redis=ok, 9 workers running |
| Storage public access | ⏭️ skipped | 原因：本地 `STORAGE_PROVIDER=local`，无公网 `public_base`；抖店 Demo 不依赖本轮 |
| 路由 smoke | ✅ | `docs/demo-route-smoke.preprod.json` passed=true（apiBase=`http://127.0.0.1:8080`） |
| Demo 数据 | ✅ | `docs/demo-dataset.preprod.json` — 20 slot + 7 task samples |
| 12 步 Demo 走查 | ✅ / ⚠️ | 浏览器复验工作台 + 失败任务中心；步骤 6 图片应用沿用 R1.1 警告 |
| 多分辨率 1366 / 1024 | ✅ | 浏览器 CDP 无异常横向溢出 |
| PostgreSQL 备份 | ⏭️ | 待真实预发 `pg_dump` |
| Admin / 二进制 / Nginx 备份 | ⏭️ | 产物已本地构建；预发机备份待 SSH |
| systemd 回滚 | ⏭️ | 见 Phase R1.1 命令模板 |
| go test / build | ✅ | 全量 + 抖店 / 刊登 / 工作台回归通过 |
| Git tag `v0.1.0-demo` | **Tag pending** | 待真实预发 HTTPS + Storage 公网 / 备份勾选后打 tag |

### Phase R1.2 部署产物（待上传预发机）

| 产物 | 路径 |
| --- | --- |
| 后端二进制 | `backend/tmp/server.exe`（Linux 预发需 `GOOS=linux go build`） |
| Admin 静态 | `admin/dist/` |
| Nginx 参考 | `admin/nginx.conf` |
| Smoke 报告 | `docs/demo-route-smoke.preprod.json` |
| Demo 数据集 | `docs/demo-dataset.preprod.json` |

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1.2 真实预发部署尝试：构建/smoke/Demo 数据/浏览器点检通过；Docker 不可用 + 无预发 SSH，HTTPS/tag 仍 pending |
| 2026-06-27 | Phase R1.1 本地预发等价部署验收与备份记录 |
| 2026-06-27 | Phase R1 部署前检查初版 |
