# TradeMind 开发进度记录

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最后更新**：2026-05-15（管理端登录与 settings 全量联调）

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **路线图阶段** | **第 1 阶段「项目地基」收尾中**：后端与**管理端登录 + 设置联调**已接通；操作日志等仍属缺口 |
| **MVP 闭环** | 未跑通；**管理端可登录**并可读写 **settings**、调用 **test-ai / test-storage** |
| **产物形态** | Monorepo 可构建：`backend`、`admin`、`collector` 均有可运行/可编译基线 |

---

## 2. 阶段目标（第 1 阶段 — 项目地基）

本阶段（v0.1.0）的验收方向（与规则一致）：

- 项目可启动；**管理端可登录**（至少管理员）
- **统一 API 返回**、**系统设置可读可写**（`settings` 表 + 敏感字段加密）
- **本地存储**与上传通路预留；docker-compose 支撑 **PostgreSQL + Redis**
- 后台 **系统设置页** 与配置后端连通；**不**在前端直连第三方 AI

> 说明：后端已完成**管理员认证**、**settings CRUD** 与 **test-ai / test-storage**；管理端已完成 **登录 JWT 落盘、请求拦截、401 回登录、access 控制与各设置页 API 绑定**。

---

## 3. 已完成事项

### 3.1 仓库与工程

- **Monorepo（pnpm）**：`pnpm-workspace.yaml`，根脚本 `dev:admin` / `build:admin` / `dev:collector` 等；**禁止使用 npm workspaces 与 package-lock 混用**（以 `pnpm-lock.yaml` 为准）。
- **Docker Compose**：本地 **PostgreSQL 16 + Redis 7**（根目录 `docker-compose.yml`）。
- **环境变量模板**：根目录 `.env.example`（含 backend / Redis / collector 等）。

### 3.2 后端（`backend/`）

- **Go + Gin** 可启动；**统一响应** `internal/pkg/response`（`code/message/data/traceId`）。
- **中间件**：RequestID（UUID）、**Recovery**（JSON 错误体，不泄露 panic 细节）、访问日志（slog）；**JWT Bearer** `middleware.BearerAuth`（受保护路由）。
- **配置**：`internal/config` 从环境变量加载（DB、Redis、**JWT**、`APP_MASTER_KEY`、**ADMIN_BOOTSTRAP_*** 等）；**生产环境**强制非默认 `JWT_SECRET`。
- **日志**：`internal/logger`（development 文本 / production JSON）。
- **数据库**：GORM，默认 **PostgreSQL**（`DB_DRIVER` 默认 `postgres`；未设置 `DB_PORT` 时默认 **5432**，MySQL 为 **3306**）；启动时 **Ping**；失败则进程退出。
- **迁移**：启动时 `database.AutoMigrate` — `admin_users`、`settings`。
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查）。
- **ID 约定**：管理员等域表主键 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）；`settings` 表为 **`BIGINT` 自增**（与规则文档一致）。
- **认证**：`admin_users` 模型；`POST /api/v1/auth/login`（bcrypt 口令、**JWT HS256**）；`GET /api/v1/auth/profile`、`POST /api/v1/auth/logout`（无状态，客户端弃 token）。
- **配置中心**：`settings` 模型与 `GET/PUT /api/v1/settings`；`item_value` 在 `is_encrypted=true` 时 **AES-GCM**（`APP_MASTER_KEY`）存储；列表接口 **脱敏**（`****` 规则）；PUT 若密文占位含 `****` 则 **不覆盖**原密钥，可更新 remark / value_type 等。
- **连通性测试**：`POST /api/v1/settings/test-ai`（读取 `ai` 组解密后请求 OpenAI 兼容 `POST /chat/completions`，`max_tokens:1`）；`POST /api/v1/settings/test-storage`（`local` 校验 `local_root` 可写；`s3/cos/oss/r2/minio` 校验必填字段完整）。`ai` / `storage` 组键名约定：`base_url`、`api_key`、`kind`、`local_root` 等（**snake_case item_key**）。
- **默认管理员**：库中无管理员时，按 `ADMIN_BOOTSTRAP_USERNAME`（默认 `admin`）与 `ADMIN_BOOTSTRAP_PASSWORD`（**非 production** 空密码时 Fallback `changeme` 并打日志；**production** 无用户则必须配置密码）插入一条记录。
- **分层占位**：`internal/providers/*` 接口仍为扩展预留；商品/采集等 **业务 CRUD 未完整实现**。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **登录与鉴权**：`/user/login` 调用 `POST /api/v1/auth/login`；**JWT** 存入 `localStorage`（`AUTH_TOKEN_KEY`）；**`request` 拦截器**自动附加 `Authorization: Bearer`；**HTTP 401**（除登录请求外）清 token 并 **整页跳转**登录页带 `redirect`；**`access.canAdmin`** 控制侧栏与业务路由；**`getInitialState`** 用 token 拉取 `/api/v1/auth/profile`。
- **布局**：右上角展示当前管理员与**退出**（`POST /auth/logout` + 清 token + 更新 initialState）。
- **系统 / AI / 存储 / 采集服务 / 安全** 设置页：`GET/PUT /api/v1/settings`，按 **groupKey**（`system`、`ai`、`storage`、`collector`、`security`）读写 **snake_case** 的 `itemKey`；敏感项（`api_key`、`secret_key`、`ops_webhook_secret`）`isEncrypted: true`，依赖后端列表脱敏与 masked 不覆盖语义；**AI / 存储**页 **测试连接** 分别调用 **`POST .../settings/test-ai`**、**`POST .../settings/test-storage`**。
- **存储页保存策略**：按当前 `kind` 仅提交相关键，避免 **local** 模式用空字段覆盖云上密钥。
- **其他页面**：工作台、商品草稿、采集任务仍为占位或未接业务 API。
- **请求封装**：`src/services/request.ts`（与后端 `Envelope` 对齐）、`src/services/settings.ts`、`src/services/auth.ts`。
- **常量**：`src/constants/status.ts`（商品状态、任务状态枚举，与规则对齐）。

### 3.4 采集服务（`collector/`）

- **Playwright + TypeScript**，独立进程，**不直连主业务库**。
- **`CollectorProvider` 接口** + **注册表**；**1688Provider 占位**（域名校验、`page.goto`、取 title，统一 `NormalizedProduct`，`raw` 必有）。
- **任务编排**：`runCollectTask`（唯一推荐入口）。
- **HTTP**：`GET /health`、`POST /v1/collect`（body：`source` + `url`）。
- **浏览器**：`BrowserManager` 单例 Chromium，`withPage` 保证关闭 page/context。

### 3.5 文档

- **本文件**：`docs/PROGRESS.md`（进度与决策单一事实来源之一，与 `README` 互补）。

---

## 4. 未完成事项（相对第 1 阶段与 MVP）

### 4.1 后端

- [x] **认证**：`POST /api/v1/auth/login`、**JWT**、管理员模型、`profile` / `logout`
- [x] **Settings 业务**：`settings` 表与 `GET/PUT /api/v1/settings`、**AES-GCM（APP_MASTER_KEY）**、脱敏与 masked 更新语义
- [x] **迁移**：启动时 GORM **AutoMigrate**（`admin_users`、`settings`）
- [ ] **操作日志**（登录、改设置等）— 规则要求的核心操作
- [x] **settings 连通性测试**：`test-ai`、`test-storage`（见上）

### 4.2 管理端

- [x] 登录页与 **access 模型**（@umijs/max access）；**Bearer** 请求拦截与 **401** 处理
- [x] **系统 / AI / 存储 / 采集服务 / 安全** 设置页与 **真实 settings API**；脱敏展示；**test-ai / test-storage** 按钮
- [ ] **商品草稿 / 采集任务** 列表接后端分页与状态

### 4.3 采集服务

- [ ] 1688 **真实结构化解析**（SKU、主图、详情图、属性等）与反爬策略
- [ ] 与 **Go 任务编排**对接（HTTP 回调或 Redis 队列），由 Go 写任务状态与结果

### 4.4 跨模块

- [ ] **Go ↔ Collector** 调用链与超时、错误码映射
- [ ] e2e：从「提交 1688 链接」到「商品草稿入库」闭环

---

## 5. 当前项目结构说明（高频路径）

```text
trademind-ai/
├── backend/                 # Go Gin 主 API
│   ├── cmd/server/          # 入口 main
│   └── internal/
│       ├── api/             # 路由
│       ├── config/          # 环境配置
│       ├── database/        # GORM
│       ├── rdb/             # Redis 客户端
│       ├── middleware/      # RequestID / Recovery / AccessLog
│       ├── pkg/             # response, id, ctxkey, model
│       ├── providers/       # 各 Provider 接口占位
│       └── modules/         # 业务模块占位（auth / product / collect）
├── admin/                   # Ant Design Pro（Umi Max）
│   ├── .umirc.ts            # 含 proxy /api -> 8080
│   ├── config/routes.ts
│   └── src/
│       ├── pages/           # Dashboard / Settings / Product / Collect
│       ├── services/        # 统一请求与 settings API 封装
│       └── constants/       # 状态枚举
├── collector/               # Node 采集（Playwright）
│   └── src/
│       ├── browser/         # BrowserManager
│       ├── providers/       # CollectorProvider + source1688
│       ├── tasks/           # runCollectTask
│       ├── http/            # HTTP 服务
│       └── types/           # NormalizedProduct
├── docs/                    # 架构与进度文档
├── data/uploads/            # 本地存储目录（默认）
├── docker-compose.yml
├── pnpm-workspace.yaml
└── .env.example
```

---

## 6. 已确认技术决策（勿随意推翻）

| 领域 | 决策 |
|------|------|
| Monorepo 包管理 | **pnpm** workspace；根目录脚本统一入口 |
| 管理端 CLI | **@umijs/max** 使用 **`max` 命令**（dev/build/setup） |
| API 形态 | 后端 JSON **`{ code, message, data, traceId? }`**；`code===0` 成功 |
| 主键 | **UUID**（应用内生成；DB `char(36)`）；**`settings` 表主键为 BIGINT 自增**（与 SQL 草案一致） |
| 认证 / 系统配置 | **JWT（HS256）** + `Authorization: Bearer`；**settings** 敏感值 **AES-GCM（APP_MASTER_KEY）**，接口侧 **脱敏** |
| 采集 | **独立 Node 服务**；统一输出 **NormalizedProduct**；**必须保留 `raw`** |
| 安全 | 第三方密钥、Token **不进前端明文**；日志 **不打全量密钥** |
| 架构 | 平台/采集/AI/存储 **走 Provider 抽象**；核心业务不直接绑定 1688/TikTok 等实现细节 |
| 主数据库 | **PostgreSQL** 为开发与 `docker-compose` 默认；仍支持 **`DB_DRIVER=mysql`** |

---

## 7. 当前遗留问题 / 风险

1. **401 处理**：采用**整页跳转**登录以清空 initialState；后续可改为无刷新同步 `setInitialState`。
2. **S3 等云存储**：`test-storage` 仅校验字段完整性；**不**发起真实列举/上传。
3. **商品、采集** 在后端多为**未实现**；设置与认证已端到端可用。
4. **1688 采集** 仅为占位：风控/登录/验证码未处理，**不可视为生产可用**。
5. **Admin 与 Backend 端口**：本地默认 admin dev 代理 `/api` → `8080`，需同时启动两端。
6. **Collector** 首次需 `pnpm install:collector:browsers`（Chromium）。

---

## 8. 下一步开发计划（建议顺序）

1. **操作日志**与核心审计（登录、改设置）。
2. **存储第 2 阶段**：本地 filesystem provider、上传 API、`data/uploads` 与 **storage.public_base** 配置对齐。
3. **采集与 Go 打通**：创建 collect task API → 调 collector → 写库；失败重试与任务状态。
4. **管理端**：商品草稿 / 采集任务列表接真实分页与状态。

（细化任务时仍以 `.cursor/rules/09-dev-workflow.mdc` 的阶段为准。）

---

## 9. Cursor 后续开发注意事项

1. **必读**：`docs/PROGRESS.md`（本文件）、`.cursorrules`、`.cursor/rules/*` 中与本任务相关的规则。
2. **开工前**：对照「**已完成 / 未完成**」，确认是否已有接口或占位，**避免重复实现**。
3. **改架构前**：核对「**已确认技术决策**」；若需变更，在本文件与相关架构文档中**写明原因与日期**。
4. **收工后**：若完成一整块功能或一次较大重构，**必须**更新本文件：勾选进度、补充遗留问题、调整「下一步」。
5. **前端**：继续 **`services/` 统一请求**；表格 **`ProTable`**、表单 **`ProForm`**；敏感字段 **脱敏**。
6. **后端**：Handler 薄、Service 编排、**外部调用带超时**；异步任务后续接 **Redis 队列**时需可观测状态。
7. **采集**：新业务逻辑放在 **`collector` 对应 Provider**，**不要**塞进 Go 核心业务层。
8. **本地数据库**：遵守 **`.cursor/rules/11-local-dev-postgres.mdc`**，默认 **PostgreSQL**；勿默认生成 MySQL 专用迁移/compose。

---

## 变更记录（简短）

| 日期 | 说明 |
|------|------|
| 2026-05-15 | 初版：记录地基进度、admin/collector/backend 基线与决策 |
| 2026-05-15 | **本地开发规则**：新增 **`.cursor/rules/11-local-dev-postgres.mdc`**（alwaysApply），同步 `.cursorrules` / `00` / `01` / `08` / `09` 中数据库表述为 **PostgreSQL 默认** |
| 2026-05-15 | **默认数据库改为 PostgreSQL**（compose、`.env.example`、`DB_DRIVER` 默认）；MySQL 仍可选 |
| 2026-05-15 | **管理端**：登录页（`/user/login`）、JWT 存储与 **Bearer** 拦截、**401** 回登录、**access**；系统/AI/存储/采集/安全设置接 **`GET/PUT /api/v1/settings`**；**test-ai / test-storage** 按钮；**后端**新增两测试接口与 **PlainByGroup** 解密探测（OpenAI 兼容最小 chat 请求；本地目录读写校验） |
