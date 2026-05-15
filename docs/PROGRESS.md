# TradeMind 开发进度记录

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最后更新**：2026-05-15

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **路线图阶段** | **第 1 阶段「项目地基」进行中**（见 `.cursor/rules/09-dev-workflow.mdc`） |
| **MVP 闭环** | 未跑通；当前处于「工程骨架 + 基础服务」向「可登录 + 可配置 + 可持久化」过渡 |
| **产物形态** | Monorepo 可构建：`backend`、`admin`、`collector` 均有可运行/可编译基线 |

---

## 2. 阶段目标（第 1 阶段 — 项目地基）

本阶段（v0.1.0）的验收方向（与规则一致）：

- 项目可启动；**管理端可登录**（至少管理员）
- **统一 API 返回**、**系统设置可读可写**（`settings` 表 + 敏感字段加密）
- **本地存储**与上传通路预留；docker-compose 支撑 **PostgreSQL + Redis**
- 后台 **系统设置页** 与配置后端连通；**不**在前端直连第三方 AI

> 说明：当前仅完成地基的**一部分**（服务骨架、页面入口、采集占位等），**登录与 settings 持久化尚未完成**。

---

## 3. 已完成事项

### 3.1 仓库与工程

- **Monorepo（pnpm）**：`pnpm-workspace.yaml`，根脚本 `dev:admin` / `build:admin` / `dev:collector` 等；**禁止使用 npm workspaces 与 package-lock 混用**（以 `pnpm-lock.yaml` 为准）。
- **Docker Compose**：本地 **PostgreSQL 16 + Redis 7**（根目录 `docker-compose.yml`）。
- **环境变量模板**：根目录 `.env.example`（含 backend / Redis / collector 等）。

### 3.2 后端（`backend/`）

- **Go + Gin** 可启动；**统一响应** `internal/pkg/response`（`code/message/data/traceId`）。
- **中间件**：RequestID（UUID）、**Recovery**（JSON 错误体，不泄露 panic 细节）、访问日志（slog）。
- **配置**：`internal/config` 从环境变量加载（DB、Redis、JWT 占位等）。
- **日志**：`internal/logger`（development 文本 / production JSON）。
- **数据库**：GORM，默认 **PostgreSQL**（`DB_DRIVER` 默认 `postgres`；未设置 `DB_PORT` 时默认 **5432**，MySQL 为 **3306**）；启动时 **Ping**；失败则进程退出。
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查）。
- **ID 约定**：业务主键统一 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）。
- **分层占位**：`internal/providers/*` 接口、`internal/modules/*` 占位；**无**完整业务 CRUD。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **布局 + 路由**：侧栏菜单与页面入口已建：
  - 工作台（Dashboard）
  - 系统设置 / AI 设置 / 存储设置（表单多为占位，**未接 API**）
  - 商品草稿、采集任务（**ProTable 占位**，空数据）
- **请求封装**：`src/services/request.ts`（与后端 `Envelope` 对齐）、`src/services/settings.ts`（接口路径已按规划预留）。
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

- [ ] **认证**：`POST /api/v1/auth/login`、JWT/Session、管理员模型
- [ ] **Settings 业务**：表结构与 API（`GET/PUT /api/v1/settings`）、**敏感字段加密**（`APP_MASTER_KEY`）
- [ ] **迁移**：GORM AutoMigrate 或 SQL migrations 落地
- [ ] **操作日志**（登录、改设置等）— 规则要求的核心操作

### 4.2 管理端

- [ ] 登录页与 **access 模型**（@umijs/max access）
- [ ] 各设置页与 **真实 API** 绑定、脱敏展示、**测试连接** 按钮落地
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
| 主键 | **UUID**（应用内生成；DB `char(36)`） |
| 采集 | **独立 Node 服务**；统一输出 **NormalizedProduct**；**必须保留 `raw`** |
| 安全 | 第三方密钥、Token **不进前端明文**；日志 **不打全量密钥** |
| 架构 | 平台/采集/AI/存储 **走 Provider 抽象**；核心业务不直接绑定 1688/TikTok 等实现细节 |
| 主数据库 | **PostgreSQL** 为开发与 `docker-compose` 默认；仍支持 **`DB_DRIVER=mysql`** |

---

## 7. 当前遗留问题 / 风险

1. **管理端页面未接登录**：任意可访问路由（后续需 access + 登录页）。
2. **Settings、商品、采集** 在后端多为**未实现**，前端调用会失败或需 mock。
3. **1688 采集** 仅为占位：风控/登录/验证码未处理，**不可视为生产可用**。
4. **Admin 与 Backend 端口**：本地默认 admin dev 代理 `/api` → `8080`，需同时启动两端。
5. **Collector** 首次需 `pnpm install:collector:browsers`（Chromium）。

---

## 8. 下一步开发计划（建议顺序）

1. **关闭第 1 阶段缺口**：用户模型、**登录 API + 登录页**、**settings 表与 CRUD**、**敏感配置加密与脱敏**。
2. **存储第 2 阶段预备**：本地 filesystem provider、上传 API、`data/uploads` 与 URL 配置对齐。
3. **Admin 设置页** 与 `GET/PUT /api/v1/settings`、`test-ai` / `test-storage` 对齐规则。
4. **采集与 Go 打通**：创建 collect task API → 调 collector → 写库；失败重试与任务状态。

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
