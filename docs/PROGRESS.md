# TradeMind 开发进度记录

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最后更新**：2026-05-15（全局 AI 任务 API + 管理端 `/ai/tasks`）

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **路线图阶段** | **第 3 阶段「AI 文本能力」主体已落地**：**AI Gateway**、**默认 `product_title_optimize` + `product_description_generate`**、**标题/描述生成 API**、**应用 AI 标题/AI 描述**、**`ai_tasks`**、**全局 AI 任务查询 API + 后台「AI 任务记录」页**、**Prompt 管理**、商品详情 **AI 标题 + AI 描述** 区块。**未做**：AI 图片、AI 客服 |
| **MVP 闭环** | 登录 → 配置 AI → 采集/草稿 → **AI 优化标题（`ai_title`）** 与 **AI 生成描述（`ai_description` 需手动应用）** 已具备（依赖有效的大模型与系统 AI 设置） |
| **产物形态** | Monorepo 可构建；本地需 **PostgreSQL**；AI 调用走 **后端 Gateway**，前端 **不直连** 第三方模型 |

---

## 2. 阶段目标（第 1 阶段 — 项目地基）

本阶段（v0.1.0）的验收方向（与规则一致）：

- 项目可启动；**管理端可登录**（至少管理员）
- **统一 API 返回**、**系统设置可读可写**（`settings` 表 + 敏感字段加密）
- **本地存储**与**上传 API**（`POST /api/v1/files/upload`）已落地；`public_base` 与 **`GET /static/*`** 对齐；docker-compose 支撑 **PostgreSQL + Redis** 不变
- 后台 **系统设置页** 与配置后端连通；**不**在前端直连第三方 AI

> 说明：后端与管理端已具备 **操作日志**、**本地文件上传与列表**、**settings CRUD**、**test-ai / test-storage**；管理端 **登录 JWT、Bearer、401、access** 与各设置页、**操作日志 / 文件管理** 页已绑定 API。

---

## 3. 已完成事项

### 3.1 仓库与工程

- **Monorepo（pnpm）**：`pnpm-workspace.yaml`，根脚本 `dev:admin` / `build:admin` / `dev:collector` 等；**禁止使用 npm workspaces 与 package-lock 混用**（以 `pnpm-lock.yaml` 为准）。
- **Docker Compose**：本地 **PostgreSQL 16 + Redis 7**（根目录 `docker-compose.yml`）。
- **环境变量模板**：根目录 `.env.example`（含 backend / Redis / collector 等）。

### 3.2 后端（`backend/`）

- **Go + Gin** 可启动；**统一响应** `internal/pkg/response`（`code/message/data/traceId`）。
- **中间件**：RequestID（UUID）、**Recovery**（JSON 错误体，不泄露 panic 细节）、访问日志（slog）；**JWT Bearer** `middleware.BearerAuth`（受保护路由）。
- **配置**：`internal/config` 从环境变量加载（DB、Redis、**JWT**、`APP_MASTER_KEY`、**`UPLOAD_MAX_MB`（默认 10）**、**ADMIN_BOOTSTRAP_*** 等）；**生产环境**强制非默认 `JWT_SECRET`。
- **日志**：`internal/logger`（development 文本 / production JSON）。
- **数据库**：GORM，默认 **PostgreSQL**（`DB_DRIVER` 默认 `postgres`；未设置 `DB_PORT` 时默认 **5432**，MySQL 为 **3306**）；启动时 **Ping**；失败则进程退出。
- **迁移**：启动时 `database.AutoMigrate` — `admin_users`、`settings`、**`operation_logs`**、**`files`**、**`products`、`product_images`、`product_skus`、`collect_tasks`**、**`ai_prompts`、`ai_tasks`**；启动后 **`aiprompt.EnsureDefaults`** 写入默认 **`product_title_optimize`**、**`product_description_generate`**（各仅在缺失时插入）
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查）。
- **ID 约定**：管理员等域表主键 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）；`settings` 表为 **`BIGINT` 自增**（与规则文档一致）。
- **认证**：`admin_users` 模型；`POST /api/v1/auth/login`（bcrypt 口令、**JWT HS256**）；`GET /api/v1/auth/profile`、`POST /api/v1/auth/logout`（无状态，客户端弃 token）。
- **JWT 上下文**：`BearerAuth` 写入 `ctxkey.AdminID` 与 **`ctxkey.AdminUsername`**（供审计与业务使用）。
- **操作日志**：`operation_logs` 表；模块 `internal/modules/operationlog`；**`GET /api/v1/operation-logs`**（分页；**action / username / resource / start / end（RFC3339）** 筛选）。写入场景：**登录成功/失败**、**logout**、**settings 批量保存成功/失败**、**test-ai / test-storage 成功/失败**（消息不落敏感配置明文）；**采集**：创建任务 / 成功 / 失败 / 重试；**商品**：手工 CRUD 与 **采集入库创建草稿**；**AI 标题**：优化成功/失败、**应用 AI 标题**；**AI 描述**：**`ai.description_generate.success` / `ai.description_generate.failed`**、**`ai.description.apply`**（不写 API Key 与完整 Prompt）
- **存储 Provider**：`internal/providers/storage` 接口 **Put / GetURL / Delete**；**本地实现** `internal/providers/storage/local`（`settings.storage`：`kind`、`local_root`、`public_base`）；**`GET /static/*filepath`** 按当前 `local_root` 提供只读文件（防 `..` 穿越）；上传仅当 **`kind=local`**，其它 kind 返回明确错误（云上传后续接 Provider）。
- **文件**：`files` 表；**`POST /api/v1/files/upload`**（`multipart` 字段名 **`file`**；**jpg/jpeg/png/webp/gif**；**objectKey = 日期目录/UUID.ext**；大小默认 **10MB**，环境变量 **`UPLOAD_MAX_MB`**）；**`GET /api/v1/files`**（分页、`contentType`）；**`DELETE /api/v1/files/:id`**（删库 + Provider 删对象）。**`MaxMultipartMemory`** 与配置上限对齐。
- **配置中心**：`settings` 模型与 `GET/PUT /api/v1/settings`；`item_value` 在 `is_encrypted=true` 时 **AES-GCM**（`APP_MASTER_KEY`）存储；列表接口 **脱敏**（`****` 规则）；PUT 若密文占位含 `****` 则 **不覆盖**原密钥，可更新 remark / value_type 等。
- **连通性测试**：`POST /api/v1/settings/test-ai`（读取 `ai` 组解密后请求 OpenAI 兼容 `POST /chat/completions`，`max_tokens:1`）；`POST /api/v1/settings/test-storage`（`local` 校验 `local_root` 可写；`s3/cos/oss/r2/minio` 校验必填字段完整）。`ai` / `storage` 组键名约定：`base_url`、`api_key`、`kind`、`local_root` 等（**snake_case item_key**）。
- **默认管理员**：库中无管理员时，按 `ADMIN_BOOTSTRAP_USERNAME`（默认 `admin`）与 `ADMIN_BOOTSTRAP_PASSWORD`（**非 production** 空密码时 Fallback `changeme` 并打日志；**production** 无用户则必须配置密码）插入一条记录。
- **商品草稿**：模块 `internal/modules/product`；模型含 **`tenant_id`、`created_by`、JSONB `raw_data`** 及图片/SKU 子表；**`GET/POST /api/v1/products`**、**`GET/PUT/DELETE /api/v1/products/:id`**；列表支持 **分页与 `status` / `source` / `keyword`**；详情 **Preload** `images`、`skus`。删除为 **软删除**（`deleted_at`）。
- **AI Provider**：`internal/providers/ai` — **`ChatRequest` / `ChatResponse`**、**`Gateway`**（只读 **`settings.ai`**：`provider`（仅 **`openai_compatible` / `openai`** 首版落地）、`base_url`、`api_key` AES-GCM 解密、`model`、`temperature`、`max_tokens`、`timeout_sec`）；**业务仅调 Gateway**；**`openai_compatible/`** 实现 HTTP **`/chat/completions`**，**Context 超时** + **http.Client Timeout**；日志与响应 **不落 api_key**
- **Prompt 模板**：模块 `internal/modules/aiprompt`；表 **`ai_prompts`**；**`EnsureDefaults`** 插入默认 **`product_title_optimize`**（变量含 **`{{title}}` `{{category}}`…**）与 **`product_description_generate`**（变量 **`{{title}}` `{{originalTitle}}` `{{aiTitle}}` `{{attributes}}` `{{skus}}` `{{language}}` `{{platform}}` `{{tone}}`**，JSON 输出 **description / highlights / specifications / packageIncludes / notes / reason**）；API：**`GET/POST /api/v1/ai/prompts`**、**`GET/PUT/DELETE .../:id`**、**`POST .../:id/enable|disable`**
- **AI 调用记录**：模块 `internal/modules/aitask`；表 **`ai_tasks`**（`task_type` / `provider` / `model` / `prompt_code` / status **`pending|running|success|failed`** 等）；**标题优化**（`task_type` **`title_optimize`**）与 **描述生成**（**`product_description_generate`**）各写一条；**`raw_response`** 仅存提供商返回 JSON 裁剪字段，**不含密钥**。**只读查询**：**`GET /api/v1/ai/tasks`**（分页；筛选 **taskType / status / provider / model / promptCode / productId / start|end（RFC3339）**；列表 **不返回** `input`/`output`/`raw_response`）；**`GET /api/v1/ai/tasks/:id`**（详情含 **input/output/rawResponse**，响应前对 JSON 内 **api_key 等敏感键** 做 **`[REDACTED]`** 脱敏）；均 **JWT**、统一 **envelope**
- **商品 AI 标题**：**`POST /api/v1/products/:id/ai/optimize-title`**（body：`language` / `platform` / `maxLength`；**不自动改 `title`**）；**`POST /api/v1/products/:id/apply-ai-title`**（`aiTitle` + `taskId`，校验任务归属，**仅更新 `products.ai_title`**）；操作日志：**`ai.title_optimize.success` / `ai.title_optimize.failed` / `ai.title.apply`**（消息 **不含密钥与完整 Prompt**）
- **商品 AI 描述**：**`POST /api/v1/products/:id/ai/generate-description`**（`language` / `platform` / `tone`，默认 en / TikTok Shop / professional；**Preload `images`+`skus`**；**不自动改 `products.description`**）；**`POST /api/v1/products/:id/apply-ai-description`**（`aiDescription` + `taskId`，**仅更新 `products.ai_description`**）；**`GET /api/v1/products/:id/ai/tasks`**（详情页最近任务，列表 **省略大体量 JSON 列**，含 **`title_optimize`** 与 **`product_description_generate`**）；操作日志：**`ai.description_generate.success` / `ai.description_generate.failed` / `ai.description.apply`**（同上）
- **采集任务**：模块 `internal/modules/collect`；表 **`collect_tasks`**（**JSONB `raw_result`**、统一状态 **pending / running / success / failed / cancelled / retrying**）；**`POST /api/v1/collect/tasks`**（body：`source` + `url`）同步编排 Collector、成功后写入商品草稿；**`GET /api/v1/collect/tasks`**（分页 + **status / source / keyword**）、**`GET .../:id`**、**`POST .../:id/retry`**（仅 **failed**）。
- **Collector HTTP 客户端**：`collector_client.go`；配置 **`COLLECTOR_BASE_URL`**（默认 `http://127.0.0.1:3100`，须与 Collector 监听端口一致）、**`COLLECTOR_TIMEOUT_SECONDS`**（默认 **60**）；**请求级 context + HTTP Client Timeout**；422/`ok:false` 映射为任务 **failed** 并写入 **`error_message`**；成功时 **`raw_result` 保存完整归一化 JSON**（内含 **`raw`** 字段）。
- **分层**：业务 Orchestration 在 **collect.Service**，采集解析仍在 **Node Collector**；Go **不写死** 1688 解析逻辑。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **登录与鉴权**：`/user/login` 调用 `POST /api/v1/auth/login`；**JWT** 存入 `localStorage`（`AUTH_TOKEN_KEY`）；**`request` 拦截器**自动附加 `Authorization: Bearer`；**HTTP 401**（除登录请求外）清 token 并 **整页跳转**登录页带 `redirect`；**`access.canAdmin`** 控制侧栏与业务路由；**`getInitialState`** 用 token 拉取 `/api/v1/auth/profile`。
- **布局**：右上角展示当前管理员与**退出**（`POST /auth/logout` + 清 token + 更新 initialState）。
- **系统 / AI / 存储 / 采集服务 / 安全** 设置页：`GET/PUT /api/v1/settings`，按 **groupKey**（`system`、`ai`、`storage`、`collector`、`security`）读写 **snake_case** 的 `itemKey`；敏感项（`api_key`、`secret_key`、`ops_webhook_secret`）`isEncrypted: true`，依赖后端列表脱敏与 masked 不覆盖语义；**AI / 存储**页 **测试连接** 分别调用 **`POST .../settings/test-ai`**、**`POST .../settings/test-storage`**；**存储页**含 **上传测试**（经 **`src/services/request.ts`** 的 **`postFormData`** → **`/api/v1/files/upload`**，展示 URL 与预览）。
- **存储页保存策略**：按当前 `kind` 仅提交相关键，避免 **local** 模式用空字段覆盖云上密钥。
- **操作日志页**：**`ProTable`** → **`GET /api/v1/operation-logs`**；只读、可筛选。
- **文件管理页**：**`ProTable`** → **`GET /api/v1/files`**；图片预览；删除 **`DELETE /api/v1/files/:id`**。
- **开发代理**：`.umirc.ts` 将 **`/static`** 代理到后端，便于 **`public_base=/static`** 时预览。
- **商品草稿**：路由 **`/product/drafts`**，`ProTable` → **`GET /api/v1/products`**；**`/product/drafts/:id`** 详情（**`description` / `aiDescription`**、**AI 标题优化** 与 **AI 描述生成** 弹窗、**最近 AI 任务** 表）；**`/ai/prompts`** **Prompt 模板**管理（**ProTable + ModalForm**）；**`/ai/tasks`** **AI 任务记录**（**ProTable** → **`GET /api/v1/ai/tasks`**，**Drawer** 详情 → **`GET .../ai/tasks/:id`**，多维筛选与 **创建时间范围**）
- **采集任务**：顶部表单 **来源（默认 1688）+ URL** → **`POST /api/v1/collect/tasks`**；表格 **`GET /api/v1/collect/tasks`**；列含 **source、source_url、status、result_product_id、error_message、started_at、finished_at、created_at**；**failed** 行 **重试** → **`POST .../retry`**。
- **请求封装**：`src/services/request.ts`、**`aiPrompts.ts`**、**`aiTasks.ts`**、`settings.ts`、`auth.ts`、**`operationLogs.ts`**、**`files.ts`**、**`products.ts`**（**optimizeTitle / apply-ai-title / generateDescription / apply-ai-description / ai/tasks**）、**`collectTasks.ts`**。
- **常量**：`src/constants/status.ts`（商品状态、任务状态枚举，与规则对齐）。

### 3.4 采集服务（`collector/`）

- **Playwright + TypeScript**，独立进程，**不直连主业务库**。
- **`CollectorProvider` 接口** + **注册表**；**1688Provider 占位**（域名校验、`page.goto`、取 title，统一 `NormalizedProduct`，`raw` 必有）。
- **任务编排**：`runCollectTask`（唯一推荐入口）。
- **HTTP**：`GET /health`、`POST /v1/collect`（body：`source` + `url`）。
- **浏览器**：`BrowserManager` 单例 Chromium，`withPage` 保证关闭 page/context。
- **与 Go 对接**：主 API **HTTP 同步调用**上述 **`POST /v1/collect`**；采集逻辑仍在 Collector，主库仅由 Go 写入。

### 3.5 文档

- **本文件**：`docs/PROGRESS.md`（进度与决策单一事实来源之一，与 `README` 互补）。

---

## 4. 未完成事项（相对「地基」验收以外的路线图）

> 「未完成」聚焦 AI / 云存储 / 采集结构化深化及异步编排：地基阶段的条目已全部勾选。

### 4.1 后端

- [x] **认证**：`POST /api/v1/auth/login`、**JWT**、管理员模型、`profile` / `logout`
- [x] **Settings 业务**：`settings` 表与 `GET/PUT /api/v1/settings`、**AES-GCM（APP_MASTER_KEY）**、脱敏与 masked 更新语义
- [x] **迁移**：启动时 GORM **AutoMigrate**（地基表 + **商品 / 采集** + **`ai_prompts` / `ai_tasks`**）
- [x] **操作日志**：表 + **`GET /api/v1/operation-logs`**；登录 / logout / settings / test-ai / test-storage / **采集关键节点 / 商品 CRUD（含采集入库）/ AI 标题优化与应用**
- [x] **本地存储与文件 API**：Storage **Put/GetURL/Delete**、**`POST /api/v1/files/upload`**、**`GET/DELETE /api/v1/files`**、**`/static`** 只读
- [x] **settings 连通性测试**：`test-ai`、`test-storage`（见上）
- [x] **商品草稿 API**：§3.2 **商品草稿**；**AI 标题与 AI 描述** 生成/应用 / 任务列表（见上）
- [x] **采集任务 API + Collector Client**：§3.2 **采集任务** / **Collector HTTP 客户端**
- [x] **AI 文本（标题+描述）**：§3.2 **AI Provider / Prompt / ai_tasks / 商品 AI 标题与 AI 描述**；**全局** **`GET /api/v1/ai/tasks`、`GET /api/v1/ai/tasks/:id`**（见 §3.2 **AI 调用记录**）；**未做** AI 图片、AI 客服

### 4.2 管理端

- [x] 登录页与 **access 模型**（@umijs/max access）；**Bearer** 请求拦截与 **401** 处理
- [x] **系统 / AI / 存储 / 采集服务 / 安全** 设置页与 **真实 settings API**；**test-ai / test-storage**；**存储页上传测试**；**操作日志**与**文件管理**页（**ProTable**）
- [x] **商品草稿 / 采集任务**：分页列表 API、筛选、采集提交表单与失败 **重试**
- [x] **Prompt 模板页**；**商品详情 AI 标题 + AI 描述** 与最近 **AI 任务**列表；**全局 AI 任务记录页**（`/ai/tasks`）

### 4.3 采集服务

- [ ] 1688 **真实结构化解析**（SKU、主图、详情图、属性等）与反爬策略
- [x] 与 **Go 任务编排**对接：**HTTP 同步调用**，由 Go 写 **`collect_tasks`** 与 **`products`**（Collector **不写主库**）

### 4.4 跨模块

- [x] **Go ↔ Collector**：HTTP **超时**、422/`ok:false` → **`collect_tasks.status=failed`**
- [x] e2e（本地）：提交合法 **1688 详情链接** → **占位解析** → **草稿入库**（字段完整度取决于 Collector）

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
│       ├── providers/       # storage（**local**）+ **ai（Gateway + openai_compatible）**
│       └── modules/         # auth、admin、settings、**operationlog**、**files**、**product**、**collect**、**aiprompt**、**aitask**
├── admin/                   # Ant Design Pro（Umi Max）
│   ├── .umirc.ts            # 含 proxy `/api` 与 **`/static`** → 8080
│   ├── config/routes.ts
│   └── src/
│       ├── pages/           # Dashboard / **System/OperationLogs** / **Files** / Settings / **AI/Prompts**、**AI/Tasks** / **Product（草稿、详情）** / **Collect/Tasks**
│       ├── services/        # request、**aiPrompts**、**aiTasks**、settings、auth、**operationLogs**、**files**、**products**、**collectTasks**
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
| 文件存储（MVP） | **上传到后端**；**object_key** 与 **public_url** 入库；本地 **`kind=local`** 已可用；**云 kind** 上传走 Provider **后续迭代** |
| AI 文本（当前） | **业务仅调用 `AI Gateway`**；**OpenAI-compatible** HTTP 适配在 **`openai_compatible/`**；**`ai_prompts` / `ai_tasks`**；标题优化 **不自动改 `products.title`**，应用只写 **`ai_title`**；描述生成 **不自动改 `products.description`**，应用只写 **`ai_description`** |

---

## 7. 当前遗留问题 / 风险

1. **401 处理**：采用**整页跳转**登录以清空 initialState；后续可改为无刷新同步 `setInitialState`。
2. **S3 等云存储**：`test-storage` 仅校验字段完整性；**不**发起真实列举/上传；**文件上传**在 **`kind≠local`** 时仍会失败（符合当前范围）。
3. **静态访问**：生产环境需自行用 **反代 / CDN** 暴露 **`/static`** 或改写 **`public_base`**；开发依赖 admin **`/static` 代理** 或直连后端端口。
4. **1688 采集** 仍为 **Collector 内占位**：主图/SKU/详情图多为空，**raw** 仅保留页面级快照；风控/登录/验证码未处理，**不可视为生产可用**。
5. **采集同步耗时**：`POST /collect/tasks` **同步阻塞**直至 Collector 返回；长超时依赖 **`COLLECTOR_TIMEOUT_SECONDS`**；后续可改为队列异步（Redis）而不改 Collector 契约。
6. **端口对齐**：**`COLLECTOR_BASE_URL`**（Go）必须与 **`COLLECTOR_HTTP_ADDR`**（Collector）监听端口一致（模板默认 **3100**）；`.env.example` 已备注。
7. **Admin 与 Backend / Collector**：本地需 **Go :8080 + Collector + Postgres**；admin dev 代理 **`/api`** → `8080`。
8. **Collector** 首次需 `pnpm install:collector:browsers`（Chromium）。
9. **MySQL 可选驱动**：当前 JSON 字段迁移以 **PostgreSQL `JSONB`** 为主路径；若使用 **MySQL**，需自行核对 GORM 对 `JSON`/`JSONB` 标签的兼容性（默认开发仍为 Postgres）。
10. **`settings.ai.provider`**：后端 Gateway 首版仅接受 **`openai_compatible` / `openai`**（兼容接口统一走 `openai_compatible` HTTP 实现；**DeepSeek / Qwen / Ollama** 等后续可增独立适配或扩展 accepted 名称）。
11. **AI 图片处理**：**未完成**（路线图第 6 阶段）。
12. **AI 客服**：**未完成**。
13. **多 AI Provider**：**`settings.ai.provider`** 与 Gateway 实际仍以 **openai_compatible** 为主路径（条目 10）；其它厂商后续可加适配。
14. **1688 真实结构化解析**：Collector 仍为占位为主，**SKU / 主图 / 详情图 / 属性** 等结构化抽取**未完成**（与条目 4、§4.3 一致）。
15. **`ai_tasks` / AI 描述**：标题与描述生成均 **`running → success|failed`**；描述任务依赖模型输出 **合法 JSON**；失败写入 **`ai_tasks`** 与操作日志。

---

## 8. 下一步开发计划（建议顺序）

1. **商品详情编辑增强**：**ProForm** 深度编辑、SKU / 图片管理与归档策略。
2. **Collector 深化**：1688 **结构化解析**（SKU、主图、详情图、属性）与稳定性；必要时 **异步队列**（Redis）承载长任务。
3. **AI 图片任务预留**（路线图第 6 阶段）：表结构 / Provider 抽象 / 管理页骨架，避免 MVP 一次性做重。
4. **存储能力收尾**：**S3/COS/OSS** Provider、**非 local** 上传与 **`files`** 串联。
5. **下一版 AI**：多 Provider 适配、客服预览（按路线图阶段执行，避免 MVP 过度扩展）。

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
| 2026-05-15 | **操作日志**：`operation_logs` + **`GET /api/v1/operation-logs`**；登录/失败、logout、改 settings、test-ai、test-storage 落库；**JWT** 写入 **username** 上下文；管理端 **操作日志 ProTable** |
| 2026-05-15 | **存储与文件**：**Storage Put/GetURL/Delete**、**local Provider**、**`files` 表**、**`/api/v1/files/upload|list|delete`**、**`GET /static/*`**；**`UPLOAD_MAX_MB`**；管理端 **文件管理**、**存储页上传测试**；admin 代理 **`/static`**；**`.env.example`** 补充上传配置 |
| 2026-05-15 | **商品草稿 + 采集闭环**：`products` / `product_images` / `product_skus`、`collect_tasks`（JSONB）；商品 CRUD 与采集 API；**Go Collector HTTP 客户端**（`COLLECTOR_BASE_URL`、`COLLECTOR_TIMEOUT_SECONDS`）；归一化结果入库与操作日志；管理端 **商品列表/详情**、**采集表单 + 任务表 + 重试**；`.env.example` 补充 Collector 编排变量 |
| 2026-05-15 | **AI 文本（第 3 阶段主线）**：`providers/ai` Gateway + **openai_compatible**；**`ai_prompts`/`ai_tasks`**、默认 **product_title_optimize**；商品 **optimize-title / apply-ai-title / ai/tasks** API；管理端 **`/ai/prompts`** 与详情页 **AI 标题**；操作日志 **ai.title_*** |
| 2026-05-15 | **AI 描述**：默认 **`product_description_generate`**；**`POST .../ai/generate-description`**、**`POST .../apply-ai-description`**；**`ai_tasks.task_type=product_description_generate`**；商品详情 **AI 描述** 区块；操作日志 **`ai.description_generate.*` / `ai.description.apply`**；**PROGRESS** 同步遗留与下一步 |
| 2026-05-15 | **全局 AI 任务**：**`GET /api/v1/ai/tasks`**（分页筛选，列表无大体量 JSON）、**`GET /api/v1/ai/tasks/:id`**（详情 **input/output/rawResponse** + 敏感键脱敏）；管理端 **`/ai/tasks`**、**`services/aiTasks.ts`**；**PROGRESS** 更新下一步与遗留对齐 |