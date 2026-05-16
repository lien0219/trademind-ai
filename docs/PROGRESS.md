# TradeMind 开发进度记录

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最后更新**：2026-05-16（**remove.bg Image Provider**：**`internal/providers/image/removebg`**（HTTP Client）+ **`factory.go` `NewForTask`**（**noop | removebg**）；**`settings.image`** 增补 **`removebg_base_url`**（默认种子空，运行时默认 **`https://api.remove.bg/v1.0`**）；**`remove_background`** 同步链路请求 **`multipart image_url`**（仅 **公网 http(s)** URL；非公网返回 **`source image URL is not publicly accessible`**）；结果 PNG 经 **`files.SaveProcessed`** → **Storage Put** + **`files`** 行；**`image_tasks.result_file_id` / `result_url` / `output` JSON**；管理端 **`/settings/image`**、**`/ai/image-tasks`**（复制 URL / 可选加入商品图）、商品详情 **图片 Tab** 可选 **removebg**。）

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **路线图阶段** | **第 5 阶段（采集）**保持；**第 6 阶段（AI 图片）**：**remove.bg 去背景** 已与 **`image_tasks` + Storage + files** **贯通**（仍为 **HTTP 内同步执行**）；**云对象存储**：**第 2 阶段** **S3-compatible** **已落地**，**COS / OSS** **占位** |
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
- **迁移**：启动时 `database.AutoMigrate` — `admin_users`、`settings`、**`operation_logs`**、**`files`**、**`products`、`product_images`、`product_skus`、`image_tasks`、`collect_batches`、`collect_tasks`、`collect_task_events`**、**`ai_prompts`、`ai_tasks`**；启动后 **`aiprompt.EnsureDefaults`** 写入默认 **`product_title_optimize`**、**`product_description_generate`**（各仅在缺失时插入）；**`settings.EnsureImageDefaults`** 写入 **`image` 分组**默认键（**`provider=noop`**、**`removebg_api_key`（可加密空）**、**`removebg_base_url`（明文空，运行时默认 API 根路径）**、**`openai_image_model` / `comfyui_*` / `timeout_sec`** 等占位）
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查；**`data.collectQueue`**：`enabled`、`name`、`redisAvailable`、`depth`、`workerEnabled`、`workerConcurrency`；**不对 Collector 做 HTTP 探测**以免拖慢健康接口）。队列开启且 Redis **Ping 正常但 LLEN 不可得**时整体 **`status` 可标记 `degraded`**。
- **ID 约定**：管理员等域表主键 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）；`settings` 表为 **`BIGINT` 自增**（与规则文档一致）。
- **认证**：`admin_users` 模型；`POST /api/v1/auth/login`（bcrypt 口令、**JWT HS256**）；`GET /api/v1/auth/profile`、`POST /api/v1/auth/logout`（无状态，客户端弃 token）。
- **JWT 上下文**：`BearerAuth` 写入 `ctxkey.AdminID` 与 **`ctxkey.AdminUsername`**（供审计与业务使用）。
- **操作日志**：`operation_logs` 表；模块 `internal/modules/operationlog`；**`GET /api/v1/operation-logs`**（分页；**action / username / resource / start / end（RFC3339）** 筛选）。写入场景：**登录成功/失败**、**logout**、**settings 批量保存成功/失败**、**test-ai / test-storage 成功/失败**（消息不落敏感配置明文）；**采集**：**`collect.batch.create` / `collect.batch.retry_failed`**、任务 **创建 / 成功 / 失败 / 单条重试**（**不写大批量 URL**，仅 `batchId`/`task_count` 等）；**商品**：手工 CRUD、**草稿字段更新（`product.update`）**、**SKU 增删改（`product.sku.*`）**、**图片增删改与排序（`product.image.*`）**与 **采集入库创建草稿**；**AI 标题**：优化成功/失败、**应用 AI 标题**；**AI 描述**：**`ai.description_generate.success` / `ai.description_generate.failed`**、**`ai.description.apply`**（不写 API Key 与完整 Prompt）；**图片任务**：**`image.task.create` / `image.task.retry` / `image.task.success` / `image.task.failed`**（**不写二进制、不写第三方密钥**；message 可含 **taskType / provider / productId**）
- **存储 Provider**：`internal/providers/storage` 接口 **Put / GetURL / Delete**；**本地** `providers/storage/local`；**S3 兼容** `providers/storage/s3store`（**AWS SDK v2**，**HTTPS/HTTP Endpoint**、`Path-style`、**配置化 Region**、`s3_public_base` **或可选预签名 GET**）；**工厂**根据 **`settings.storage.kind`** 选出实现（**`local` / `s3` / `r2` / `minio`**；**`cos` / `oss` 返回未实现错误**）；**解密** **`s3_access_key_id`、`s3_secret_access_key`**；**不写密钥到日志**。**`GET /static/*filepath`** 仍仅服务 **当前 `kind=local`**。
- **文件**：`files` 表；**`POST /api/v1/files/upload`**（`multipart` 字段名 **`file`**；**jpg/jpeg/png/webp/gif**；**objectKey = 日期目录/UUID.ext**；**`storage_kind` 落库**，**云端 `public_url`** 取自 **`GetURL`**）；**本地行为不变**。**`GET /api/v1/files`**（分页、`contentType`）；**`DELETE /api/v1/files/:id`**（**先做 Provider.Delete（按 `storage_kind`，缺省回落当前 settings.kind）**，成功后再删 DB，避免「库删了对象残留」）。
- **配置中心**：`settings` 模型与 `GET/PUT /api/v1/settings`；`item_value` 在 `is_encrypted=true` 时 **AES-GCM**（`APP_MASTER_KEY`）存储；列表接口 **脱敏**（`****` 规则）；PUT 若密文占位含 `****` 则 **不覆盖**原密钥，可更新 remark / value_type 等。
- **连通性测试**：`POST /api/v1/settings/test-ai`（读取 `ai` 组解密后请求 OpenAI 兼容 `POST /chat/completions`，`max_tokens:1`）；`POST /api/v1/settings/test-storage`（**`local`** 目录可写；**`s3`/`r2`/`minio`** **HeadBucket + 短时 context**；**`cos`/`oss`** **明确告知未接入独立 Provider**。**不发真实上传**。`storage` 优先 **`s3_*`**（兼容遗留 **`endpoint`/`bucket`/`access_key`/`secret_key`/`region`**）。
- **默认管理员**：库中无管理员时，按 **`ADMIN_BOOTSTRAP_EMAIL` 与/或 `ADMIN_BOOTSTRAP_PHONE`**（至少填一项）及 `ADMIN_BOOTSTRAP_PASSWORD`（**非 production** 空密码 Fallback `changeme` 并告警；**production** 无用户则必须配置密码）插入一条记录；**不支持用「用户名」登录**，仅邮箱或手机号 + 密码；内部 `username` 列为随机 ID，由实现自行分配。
- **商品草稿**：模块 `internal/modules/product`；模型含 **`tenant_id`、`created_by`、JSONB `raw_data`** 及 **`product_images` / `product_skus`**（SKU **`attrs`、`raw_data` JSONB**，**前端不可改 raw_data**）。**列表**：**`GET/POST /api/v1/products`**；**详情**：**`GET/PUT/DELETE /api/v1/products/:id`**；**`PUT` 可编辑**：`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；**一并支持 JSON snake_case**（如 `original_title`、`ai_title`）；**不写** `id` / `created_by` / `created_at`；**不通过 PUT 修改** `source` / `source_url` / `raw_data`（采集来源与归一快照只读）。**`status`** 枚举校验：`draft` / `ai_processing` / `ready` / `published` / **`archived`**。删除仍为 **软删除**（`products.deleted_at`）。**子资源**：**`POST/PUT/DELETE /api/v1/products/:id/skus`、`:skuId`**；**`POST/POST(reorder)/PUT/DELETE /api/v1/products/:id/images`、`:imageId`、`/images/reorder`**；图片 **`image_type`** 支持 **`main` / `detail` / `sku`**（并接受旧值 **`description` 归入 detail**）；**可按 `files.id`（`fileId`）关联本地上传**；**删除 `product_images` 仅断开关联**，默认 **不删除 `files`** 存储对象。**采集入库**：新草稿详情图默认写入 **`detail`**（历史中可能仍有 **`description`** 行）。
- **AI Provider**：`internal/providers/ai` — **`ChatRequest` / `ChatResponse`**、**`Gateway`**（只读 **`settings.ai`**：`provider`（仅 **`openai_compatible` / `openai`** 首版落地）、`base_url`、`api_key` AES-GCM 解密、`model`、`temperature`、`max_tokens`、`timeout_sec`）；**业务仅调 Gateway**；**`openai_compatible/`** 实现 HTTP **`/chat/completions`**，**Context 超时** + **http.Client Timeout**；日志与响应 **不落 api_key**
- **Prompt 模板**：模块 `internal/modules/aiprompt`；表 **`ai_prompts`**；**`EnsureDefaults`** 插入默认 **`product_title_optimize`**（变量含 **`{{title}}` `{{category}}`…**）与 **`product_description_generate`**（变量 **`{{title}}` `{{originalTitle}}` `{{aiTitle}}` `{{attributes}}` `{{skus}}` `{{language}}` `{{platform}}` `{{tone}}`**，JSON 输出 **description / highlights / specifications / packageIncludes / notes / reason**）；API：**`GET/POST /api/v1/ai/prompts`**、**`GET/PUT/DELETE .../:id`**、**`POST .../:id/enable|disable`**
- **AI 调用记录**：模块 `internal/modules/aitask`；表 **`ai_tasks`**（`task_type` / `provider` / `model` / `prompt_code` / status **`pending|running|success|failed`** 等）；**标题优化**（`task_type` **`title_optimize`**）与 **描述生成**（**`product_description_generate`**）各写一条；**`raw_response`** 仅存提供商返回 JSON 裁剪字段，**不含密钥**。**只读查询**：**`GET /api/v1/ai/tasks`**（分页；筛选 **taskType / status / provider / model / promptCode / productId / start|end（RFC3339）**；列表 **不返回** `input`/`output`/`raw_response`）；**`GET /api/v1/ai/tasks/:id`**（详情含 **input/output/rawResponse**，响应前对 JSON 内 **api_key 等敏感键** 做 **`[REDACTED]`** 脱敏）；均 **JWT**、统一 **envelope**
- **AI 图片任务**：模块 **`internal/modules/imagetask`**；表 **`image_tasks`**（**`task_type`**：`remove_background` / …；**`status`**：**`pending` / `running` / `success` / `failed` / `cancelled`**；**JSONB `input` / `output`**；**`source_image_id`** → **`files` / `product_images`** 解析 **`public_url`**；**`result_file_id` / `result_url`**；**不落库图片二进制**）。**Image Provider**：**`internal/providers/image`** + **`removebg`** 子包（**HTTP**，仅 **`RemoveBackgroundPNG`**）；**`factory.NewForTask`**：**`noop` | `removebg`**（读 **`settings.image`**：**`removebg_api_key`（解密）**、**`removebg_base_url`**、**`timeout_sec`**；缺 Key **明确报错**；**密钥不写日志**）。**`remove_background` + removebg**：仅接受 **公网可抓取** 的 **`image_url`**（非公网 **`source image URL is not publicly accessible`**）；响应 PNG → **`files.SaveProcessed`**（**`image-tasks/YYYYMMDD/<taskId>-removebg-<suffix>.png`**）→ **`storage_kind` / `public_url`**；**`output`** 含 **`resultUrl`、`resultFileId`、`provider`、`contentType`**。**其它任务类型**：noop 仍为 **`resize`/`enhance` 回显**；removebg 对其余 API **未实现**。**API**：**`POST /api/v1/image/tasks`**（body **`provider` 可空**，空则 **`settings.image.provider`**）、**同步执行**、**`retry`** 同链路。**操作日志**：**`image.task.*`**（不落密钥）
- **商品 AI 标题**：**`POST /api/v1/products/:id/ai/optimize-title`**（body：`language` / `platform` / `maxLength`；**不自动改 `title`**）；**`POST /api/v1/products/:id/apply-ai-title`**（`aiTitle` + `taskId`，校验任务归属，**仅更新 `products.ai_title`**）；操作日志：**`ai.title_optimize.success` / `ai.title_optimize.failed` / `ai.title.apply`**（消息 **不含密钥与完整 Prompt**）
- **商品 AI 描述**：**`POST /api/v1/products/:id/ai/generate-description`**（`language` / `platform` / `tone`，默认 en / TikTok Shop / professional；**Preload `images`+`skus`**；**不自动改 `products.description`**）；**`POST /api/v1/products/:id/apply-ai-description`**（`aiDescription` + `taskId`，**仅更新 `products.ai_description`**）；**`GET /api/v1/products/:id/ai/tasks`**（详情页最近任务，列表 **省略大体量 JSON 列**，含 **`title_optimize`** 与 **`product_description_generate`**）；操作日志：**`ai.description_generate.success` / `ai.description_generate.failed` / `ai.description.apply`**（同上）
- **采集任务与批次**：模块 `internal/modules/collect`。**表**：**`collect_batches`**（聚合与衍生 **`batch.status`** 同前）；**`collect_tasks`**：**JSONB `raw_result`**、状态 **pending / running / success / failed / cancelled / retrying**、**`retry_count` / `max_retries` / `next_retry_at` / `retry_enqueued_at`**（创建时 **`max_retries`** 默认取 **`COLLECT_MAX_RETRIES`**）。**`collect_task_events`**：采集任务状态流水（**PostgreSQL **`JSONB` `payload`**；**不写**密钥/Cookie/HTML/大体量 **`raw_result`**）；事件字段含 **`task_id`、`batch_id`、`event_type`、`from_status`、`to_status`、`message`、`error_message`、`retry_count`、`max_retries`、`next_retry_at`、`created_at`**；类型：**`task.created` / `task.enqueued` / `task.running` / `task.success` / `task.failed` / `task.auto_retry_scheduled` / `task.auto_retry_enqueued` / `task.retry_exhausted` / `task.manual_retry`**（**`task.cancelled` 预留**）；**写入**覆盖创建/每条入队/Worker 认领/成功/立即失败或不可重试/自动重试调度与再入队/用尽/**人工与批次 retry-failed**/**Redis 回落失败**。只读：**`GET /api/v1/collect/tasks/:id/events`**（**JWT**，**`page|pageSize`（默认 `pageSize=50`，≤100）**，**按 `created_at ASC`**）。**自动重试**：**`COLLECT_AUTO_RETRY_ENABLED`** 且错误非 **Collector `INVALID_URL` / `INVALID_REQUEST` / `PROVIDER_NOT_FOUND`** 时，若 **`retry_count < max_retries`** 则 **`retrying`**、**`retry_count+=1`**、**`next_retry_at=now+delay`**（阶梯 **30→60→120s…** 上限 **`COLLECT_RETRY_MAX_DELAY_SECONDS`**）、写 **`collect.task.auto_retry_scheduled`**（**操作日志**，与 **`collect_task_events` 并行**）；否则 **`failed`**、**`collect.task.retry_exhausted`**。**`StartRetryScheduler`**（约 **5s**）将到点任务 **CAS** 清空 **`next_retry_at`**、写入 **`retry_enqueued_at`** 后 **`LPUSH`**，记 **`collect.task.auto_retry_enqueued`**；**单进程**消费，重复入队风险低。**单链接 / 批量 / 查询 / 监控** API 同前；**`GET /collect/monitor`** 另含 **`retry{...}`**、**`recentRetrying`（10）**、**`tasks.retryingCount`**。**人工重试**：**`POST .../tasks/:id/retry`**（仅 **`failed`**）、**`POST .../batches/:id/retry-failed`**：**`retry_count=0`**，**`next_retry_at`/`retry_enqueued_at` 清空**，立即入队。**聚合**：**pending + retrying 计入 `pending_count`**（不变）。**Worker**：**`StartWorker`** + **`StartRetryScheduler`**（自动重试开启时）。
- **Collector HTTP 客户端**：`collector_client.go`；…；422/`ok:false` 按错误类型进入 **自动重试** 或 **立即 failed**（见上）；成功时 **`raw_result`** 保存完整归一化 JSON。
- **分层**：业务 Orchestration 在 **collect.Service**，采集解析仍在 **Node Collector**；Go **不写死** 1688 解析逻辑。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **登录与鉴权**：`/user/login` 调用 `POST /api/v1/auth/login`；**JWT** 存入 `localStorage`（`AUTH_TOKEN_KEY`）；**`request` 拦截器**自动附加 `Authorization: Bearer`；**HTTP 401**（除登录请求外）清 token 并 **整页跳转**登录页带 `redirect`；**`access.canAdmin`** 控制侧栏与业务路由；**`getInitialState`** 用 token 拉取 `/api/v1/auth/profile`。
- **布局**：右上角展示当前管理员与**退出**（`POST /auth/logout` + 清 token + 更新 initialState）。
- **系统 / AI / 存储 / 采集服务 / 安全 / 图片 AI** 设置页：`GET/PUT /api/v1/settings`，按 **groupKey**（`system`、`ai`、`storage`、`collector`、`security`、**`image`**）读写 **snake_case** `itemKey`；敏感项含 **`api_key`、`secret_key`、`s3_secret_access_key`、`s3_access_key_id`** 等 **`isEncrypted: true`**；**存储页**：**`kind` 切换**（**本地** `public_base`/`local_root`；**`s3`/`r2`/`minio`**：**`s3_*`** — Endpoint/Region/Bucket、密钥 **密码输入 + 脱敏 + masked 不覆盖**、Path-style、`s3_use_ssl`、可选 **`s3_presign_*`、`s3_public_base`**）；**COS/OSS** **占位标签 + Alert**。**测试** `test-ai` / **`test-storage`（云端 HeadBucket）**；**上传测试** **`POST /api/v1/files/upload`**（前端 **不直连** S3/R2/MinIO）。
- **存储页保存策略**：按当前 `kind` 仅提交相关键；**COS/OSS** 仅存 **`kind`**，避免无用键覆盖。
- **操作日志页**：**`ProTable`** → **`GET /api/v1/operation-logs`**；只读、可筛选。
- **文件管理页**：**`ProTable`** → **`GET /api/v1/files`**；图片预览；删除 **`DELETE /api/v1/files/:id`**。
- **开发代理**：`.umirc.ts` 将 **`/static`** 代理到后端，便于 **`public_base=/static`** 时预览。
- **商品草稿**：路由 **`/product/drafts`**，`ProTable` → **`GET /api/v1/products`**；**`/product/drafts/:id`** **商品草稿编辑页**：**Tabs**（**基础信息 ProTable type=form**、**AI 标题/描述**（原弹窗逻辑保留）、**图片管理**（上传走 **`/api/v1/files/upload`** + **`createProductImage`**、**AI 图片任务入口**、编辑类型与 sortOrder、**reorder**）、**SKU `EditableProTable`**、最近 **AI 任务**）；顶栏快捷 **标记 ready / 归档 / 软删草稿**。**`/ai/prompts`**、**`/ai/tasks`**、**`/ai/image-tasks`** 同前。**`products.ts`** 增补 **`updateProduct`、SKU/Image 全套 API 封装**；**`imageTasks.ts`** 封装图片任务 API。
- **采集**：路由 **`/collect/tasks`** **单链接** + **`/collect/batches`** **批量** + **`/collect/monitor`** **采集监控**：批量 **多行链接**、计数与 **5s** 轮询；任务表展示 **重试进度 / 下次自动重试时间**，**retrying** 为 **「等待重试」**，**任务行「事件」**打开 **`CollectTaskEventDrawer`**（**`Timeline` + 任务快照 + `GET .../tasks/:id/events`**，前 100 条）；**批次 Drawer** 内子任务表 **「事件」**复用同款 Drawer；监控页 **`recentFailures` / `recentRetrying`** 列表 **「事件」**同上；批次/任务 **失败重试** 与 **success → 草稿** 流程不变；服务 **`services/collectTasks.ts`**（**`queryCollectTaskEvents`**）、**`services/collectBatches.ts`**、**`services/collectMonitor.ts`**。
- **常量**：`src/constants/status.ts`（商品状态、**采集任务 / 批次**状态枚举）。

### 3.4 采集服务（`collector/`）

- **Playwright + TypeScript**，独立进程，**不直连主业务库**。
- **`CollectorProvider` 接口** + **注册表**；**1688Provider 已增强**：域名校验与 **offer 路径语义**，`goto` → 可选 **`networkidle` 短超时** + 标题区等待；合并 **DOM 主图/详情图/参数表**，多组 **兜底选择器**，**lazy 属性**（`src`/`data-src`/`data-lazy-src`/`data-original`/`data-img`）；从 **高危 script / `ld+json`** 截断片段解析 JSON，递归抽取 **`subject`/图链/`skuMap`/`skuProps`**；**SKU 行**对齐 Go：**`skuCode`、`price`、`stock`、`properties`（前端展示名由服务端从 `properties` 推导）、`image`、SKU 粒度 **`raw`**；顶层 **`raw`** 固定含 `title`、`url`、`mainImageCandidates`、`detailImageCandidates`、`attributeCandidates`、`skuCandidates`、`pageMeta`、`extractedAt`（及轻量 **`scriptDigest`/`jsonRootCount`**，**不包含整页 HTML**）。
- **任务编排**：`runCollectTask`（唯一推荐入口）。
- **HTTP**：`GET /health`、`POST /v1/collect`（body：`source` + `url`）。
- **浏览器**：`BrowserManager` 单例 Chromium，`withPage` 保证关闭 page/context。
- **与 Go 对接**：主 API **HTTP 同步调用**上述 **`POST /v1/collect`**；**NormalizedProduct JSON 契约未变**，`BuildImportSKU` 仍只吃 **`properties`**；采集逻辑仅在 Collector。
- **本地调试**：包内 **`pnpm collect:test -- --url "https://detail.1688.com/offer/..."`**，或 **`COLLECT_TEST_URL`**；根仓库 **`pnpm collect:test`** 透传到 `@trademind/collector`。

### 3.5 文档

- **本文件**：`docs/PROGRESS.md`（进度与决策单一事实来源之一，与 `README` 互补）。

---

## 4. 未完成事项（相对「地基」验收以外的路线图）

> 「未完成」聚焦 AI / 云存储 / 采集结构化深化及异步编排：地基阶段的条目已全部勾选。

### 4.1 后端

- [x] **认证**：`POST /api/v1/auth/login`、**JWT**、管理员模型、`profile` / `logout`
- [x] **Settings 业务**：`settings` 表与 `GET/PUT /api/v1/settings`、**AES-GCM（APP_MASTER_KEY）**、脱敏与 masked 更新语义
- [x] **迁移**：启动时 GORM **AutoMigrate**（地基表 + **商品 / 采集** + **`ai_prompts` / `ai_tasks` / `image_tasks`**）
- [x] **操作日志**：表 + **`GET /api/v1/operation-logs`**；登录 / logout / settings / test-ai / test-storage / **采集关键节点 / 商品 CRUD（含采集入库、`product.update`、SKU／图片子资源、`product.image.reorder`）/ AI 标题优化与应用 / 图片任务 `image.task.*`**
- [x] **对象存储与文件 API**：Storage **factory**（**`local`** + **`s3store`/`s3`|`r2`|`minio`**）**Put/GetURL/Delete**，**COS/OSS 未接入**；**`POST /api/v1/files/upload`**（**`storage_kind` 入库**）；**`GET/DELETE /api/v1/files`**（删 **云端对象先于 DB**，按 **`files.storage_kind`**）；**`/static`** 仅 **本地** 只读
- [x] **settings 连通性测试**：`test-ai`、`test-storage`（见上）
- [x] **商品草稿 API**：§3.2 **商品草稿**；**AI 标题与 AI 描述** 生成/应用 / 任务列表（见上）
- [x] **采集任务 API + Collector Client**：§3.2 **采集任务** / **Collector HTTP 客户端**
- [x] **AI 文本（标题+描述）**：§3.2 **AI Provider / Prompt / ai_tasks / 商品 AI 标题与 AI 描述**；**全局** **`GET /api/v1/ai/tasks`、`GET /api/v1/ai/tasks/:id`**（见 §3.2 **AI 调用记录**）；**未做** AI 客服
- [x] **AI 图片任务（remove.bg 落地）**：§3.2 **image_tasks / removebg Client + factory / SaveProcessed / imagetask API**；管理端 **`/ai/image-tasks`**（复制结果 URL、可选 **添加到商品详情图**）、**`/settings/image`（removebg 字段）**；**未做** OpenAI Image / ComfyUI / **异步队列消费** / **本地非公网文件直传 remove.bg**

### 4.2 管理端

- [x] 登录页与 **access 模型**（@umijs/max access）；**Bearer** 请求拦截与 **401** 处理
- [x] **系统 / AI / 存储 / 采集服务 / 安全 / 图片 AI** 设置页与 **真实 settings API**；**test-ai / test-storage**（S3-compat **HeadBucket**）；**存储页**：**`s3_*` / 密钥加密 / COS·OSS 占位**；**上传测试**走 **`/files/upload`**；**操作日志**与**文件管理**页（**ProTable**）
- [x] **商品草稿 / 采集任务（含批量采集 `/collect/batches`）**：分页列表 API、筛选、**单链接**与**批量**表单；失败 **重试** / 批次 **重试失败**
- [x] **Prompt 模板页（`/ai/prompts`）**；**商品详情编辑页（`/product/drafts/:id`）**：**Tabs**（基础表单、保留 **AI 标题/描述** 弹窗、**图片 ModalForm/Reorder** 与 **AI 图片任务入口**、**SKU `EditableProTable`**、最近 AI 任务）；**`/ai/tasks` 全局 AI 任务记录页**；**`/ai/image-tasks` 图片任务页**；**`/settings/image` 图片 AI 设置**

### 4.3 采集服务

- [x] 1688 **结构化解析落地（首版）**：主图 **`mainImages`**、详情 **`descriptionImages`**、**`attributes`**、**`skus`**（含 **`properties`/价格/库存/可选图**）；**降级不抛解析异常**（仅非法 URL、导航失败、非 offer 跳转、验证码页且全无结构化字段时 **`INVALID_URL`** 失败）。
- [ ] **反爬与稳定性深化**（人机验证绕过、SKU 多维长期可用、异步详情 iframe 全覆盖等）。
- [x] 与 **Go 任务编排**对接：**HTTP 异步队列**（Redis list + Worker **`POST /v1/collect`**），由 Go 写 **`collect_tasks`** 与 **`products`**（Collector **不写主库**）

### 4.4 跨模块

- [x] **Go ↔ Collector**：HTTP **`POST /v1/collect`**（Worker 发起，`NormalizedProduct` 契约不变）；**422/`ok:false` → `collect_tasks.status=failed`**
- [x] e2e（本地）：提交合法 **1688 详情链接** → **结构化解析** → **草稿入库**（**主图/SKU 等完整性仍受站点与风控影响**）

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
│       ├── providers/       # storage（**local + s3store**）+ **ai（Gateway + openai_compatible）** + **image（接口 + noop + removebg Client + factory）**
│       └── modules/         # auth、admin、settings、**operationlog**、**files**、**product**、**collect**、**aiprompt**、**aitask**、**imagetask**
├── admin/                   # Ant Design Pro（Umi Max）
│   ├── .umirc.ts            # 含 proxy `/api` 与 **`/static`** → 8080
│   ├── config/routes.ts
│   └── src/
│       ├── pages/           # … **Collect/Tasks**、**Collect/Batches**、**Collect/Monitor** …
│       ├── services/        # … **`collectTasks`**、**`collectBatches`**、**`collectMonitor`**、**`imageTasks`** …
│       └── constants/       # 状态枚举
├── collector/               # Node 采集（Playwright）
│   └── src/
│       ├── browser/         # BrowserManager
│       ├── providers/       # CollectorProvider + source1688（**parser / selectors / utils**）
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
| 文件存储（MVP） | **上传到后端**；**object_key** 与 **public_url** 入库；**`kind=local` / `s3` / `r2` / `minio`** **已走 Provider（AWS SDK v2 S3 兼容）**；**`cos`/`oss`** **独立适配未完成** |
| AI 文本（当前） | **业务仅调用 `AI Gateway`**；**OpenAI-compatible** HTTP 适配在 **`openai_compatible/`**；**`ai_prompts` / `ai_tasks`**；标题优化 **不自动改 `products.title`**，应用只写 **`ai_title`**；描述生成 **不自动改 `products.description`**，应用只写 **`ai_description`** |
| AI 图片 | **`internal/providers/image`** **接口** + **`noop`** + **`removebg`**（去背景）；**`factory.NewForTask`**；**`image_tasks`** 与 **`/api/v1/image/tasks`**（当前 **同步执行**）；**OpenAI Image / ComfyUI**、**Redis 异步 Worker**、**本地非公网图直传 remove.bg** **未完成** |

## 7. 当前遗留问题 / 风险

1. **401 处理**：采用**整页跳转**登录以清空 initialState；后续可改为无刷新同步 `setInitialState`。
2. **`s3_presign_enabled` 入库 URL**：启用预签名时 **`files.url`** 为**短时有效链接**，过期后预览/外链失效；生产推荐配置稳定 **`s3_public_base`**（或后续做按需重签）。
3. **COS / OSS 独立 Provider**：`kind` 为 **`cos` 或 `oss`** 仅占位；上传与 **`test-storage`** 会明确失败提示；若需 COS/OSS 原生语义（非 S3 API）须另加适配。
4. **静态访问**：生产环境需自行用 **反代 / CDN** 暴露 **`/static`** 或改写 **`public_base`**（**仅本地 `kind`**）；开发依赖 admin **`/static` 代理**或直连后端端口。
5. **1688 采集** 已升级为 **结构化首版**：多数商品页可从 DOM + JSON 抽到 **主图/详情图/属性/SKU**；**站点改版、登录/验证码/风控会导致字段缺失**，详情图若在 **跨域 iframe / 异步接口** 仍可能不完整；非生产 SLA。
6. **多实例 Worker / 编排观测**：**`collect_task_events` 流水与单任务 Timeline 已完成**（见 §3.2）；**多实例部署下的 Worker heartbeat / 注册未完成**（当前仅为 **单进程 `running` 标记**）。
7. **忘记密码未完成**：已在登录页占位，尚未实现后端逻辑。
8. **手机号注册/短信未完成**：注册仍仅限 **邮箱 + 验证码**；**登录**已支持邮箱或手机号（规范化数字，兼容 +86）；短信注册/找回未做。
9. **更多邮件服务商未完成**：当前仅完成了 SMTP 方式对接发送，尚未提供 Mailgun 等其它供应商实现。
10. **验证码风控可继续增强**：目前已做简单的时间窗与数量限制。
11. **历史管理员数据**：早期仅在内部 `username` 列有意义、未填 **`email` / `phone`** 的账号将无法按新规则登录；需在库里补齐邮箱或手机号，或清空表后重新 bootstrap。
12. **端口对齐**：**`COLLECTOR_BASE_URL`**（Go）必须与 **`COLLECTOR_HTTP_ADDR`**（Collector）监听端口一致（模板默认 **3100**）；`.env.example` 已备注。
13. **Admin 与 Backend / Collector**：本地需 **Go :8080 + Collector + Postgres**；admin dev 代理 **`/api`** → `8080`。
14. **Collector** 首次需 `pnpm install:collector:browsers`（Chromium）。
15. **MySQL 可选驱动**：当前 JSON 字段迁移以 **PostgreSQL `JSONB`** 为主路径；若使用 **MySQL**，需自行核对 GORM 对 `JSON`/`JSONB` 标签的兼容性（默认开发仍为 Postgres）。
16. **`settings.ai.provider`**：后端 Gateway 首版仅接受 **`openai_compatible` / `openai`**（兼容接口统一走 `openai_compatible` HTTP 实现；**DeepSeek / Qwen / Ollama** 等后续可增独立适配或扩展 accepted 名称）。
17. **AI 图片处理**：**remove.bg `remove_background`** 已与 **Storage + `files`**、**`result_file_id`/`result_url`** 贯通（§3.2）；**OpenAI Image / ComfyUI**、**异步队列化 Worker**、**本地非公网源图 multipart 上传 remove.bg（不经公网 URL）** 仍 **未完成**。
18. **AI 客服**：**未完成**。
19. **多 AI Provider**：**`settings.ai.provider`** 与 Gateway 实际仍以 **openai_compatible** 为主路径；其它厂商后续可加适配。
20. **1688 采集边界 / 反爬稳定性**：虽已 **DOM + script JSON 解析**，仍存在 **SKU 组合不全**、详情图异步、**`/offer` URL 误判**、**人机验证 / 风控**等边界；需在真实流量下持续补强选择与稳定性。
21. **`ai_tasks` / AI 描述**：标题与描述生成均 **`running → success|failed`**；描述任务依赖模型输出 **合法 JSON**；失败写入 **`ai_tasks`** 与操作日志。
22. **图片任务异步化**：当前 **`remove_background`（removebg）仍为 HTTP 请求内同步执行**；**Redis / 独立 Worker** 消费 **`image_tasks`** 未完成。

---

## 8. 下一步开发计划（建议顺序）

1. **图片任务异步化**：**Redis 队列**或内部 Worker；**running** 态与 **重试策略**；本地非公网图 **multipart** 直传 remove.bg。
2. **真实图片 Provider（剩余）**：**OpenAI Image**；**ComfyUI**（读取 **`settings.image`**）；**Flux / 即梦** 预留。
3. **COS / OSS 独立 Provider**：在 **腾讯云 / 阿里云** 原生 SDK（或专有域名）上与 **`files`/设置页**对齐，而不走 S3 兼容端点。
4. **多实例 Worker**：**Redis heartbeat** 或注册表（采集 Worker）。
5. **Collector 演进**：SKU **多维 prop**、详情 **iframe/async**、**人机验证**探测与指引（**不迁入 Go**）。

（细化任务时仍以 `.cursor/rules/09-dev-workflow.mdc` 的阶段为准。）

---

## 9. Cursor 后续开发注意事项

1. **必读**：`docs/PROGRESS.md`（本文件）、`.cursorrules`、`.cursor/rules/*` 中与本任务相关的规则。
2. **开工前**：对照「**已完成 / 未完成**」，确认是否已有接口或占位，**避免重复实现**。
3. **改架构前**：核对「**已确认技术决策**」；若需变更，在本文件与相关架构文档中**写明原因与日期**。
4. **收工后**：若完成一整块功能或一次较大重构，**必须**更新本文件：勾选进度、补充遗留问题、调整「下一步」。
5. **前端**：继续 **`services/` 统一请求**；表格 **`ProTable`**、表单 **`ProForm`**；敏感字段 **脱敏**。
6. **后端**：Handler 薄、Service 编排、**外部调用带超时**；采集异步已接 **Redis 列表 + Worker**（**可观测指标/告警**仍可加强）。
7. **采集**：新业务逻辑放在 **`collector` 对应 Provider**，**不要**塞进 Go 核心业务层。
8. **本地数据库**：遵守 **`.cursor/rules/11-local-dev-postgres.mdc`**，默认 **PostgreSQL**；勿默认生成 MySQL 专用迁移/compose。

---

## 变更记录（简短）

| 日期 | 说明 |
|------|------|
| 2026-05-16 | **remove.bg**：**`providers/image/removebg` Client** + **`factory.NewForTask`**（noop/removebg）；**`settings.image`** **`removebg_base_url`** 种子；**`files.SaveProcessed`**；**imagetask** 持久化 **`result_file_id`/`result_url`/output**；admin **`/settings/image`**、**`/ai/image-tasks`**、商品详情 **Provider 可选 removebg**；**PROGRESS** §1/§3/§6/§7/§8 同步 |
| 2026-05-16 | **云存储 S3-compatible**：后端 **`internal/providers/storage/s3store`**（**AWS SDK v2**）、**factory**（`local`/`s3`/`r2`/`minio`；`cos`/`oss` 未实现）、**`files/upload|delete`** 与 **`test-storage` HeadBucket**；删除按 **`storage_kind`**；admin **存储设置 `s3_*`**；**`.env.example` / README** 存储说明；**go.mod** 引入 AWS SDK；**PROGRESS** 全篇对齐 |
| 2026-05-16 | **GitHub Actions Go CI**：`.github/workflows/go.yml`（`main` 上 **push / pull_request**；`backend/` 内 **`gofmt -l` / `go vet` / `go test` / `go build`**；缺失 **`backend/`** 或 **`backend/go.mod`** 时显式失败；**`go-version-file: backend/go.mod`**）；**`go fmt`** 整理部分后端源文件以满足格式检查；**README** 增加「**CI / 自动检查**」 |
| 2026-05-16 | **AI 图片任务预留**：**`image_tasks`**、**`internal/providers/image` + `noop`**、**`POST|GET /api/v1/image/tasks`、详情、`retry`**、**`settings.EnsureImageDefaults`（`image` 分组）**、操作日志 **`image.task.*`**、管理端 **`/ai/image-tasks`**、**`/settings/image`**、商品详情 **图片 Tab 入口**；**PROGRESS** §1/§3/§6/§7/§8 同步 |
| 2026-05-16 | **`collect_task_events` + Timeline API + Admin Drawer**：新增表（**§3.2**）、节点写入、`GET /api/v1/collect/tasks/:id/events`（**JWT**、**ASC**、默认 **pageSize=50**）；**`CollectTaskEventDrawer`**（任务/批次/监控）；rollback 连带删事件；**§7 遗留（heartbeat/AI图/多云/Collector）§8 下一步** 重排 |
| 2026-05-16 | **采集队列可观测性**：**`GET /api/v1/collect/monitor`**（JWT；**`LLEN`**、任务/批次 **`GROUP BY status`**、**`recentFailures`**、**`oldestPendingSeconds`**、**Worker**、**Collector `/health` 短超时**）；**`/health` / `/api/v1/health`** **`collectQueue`**（无 Collector 探测）；**`ConfigureWorkerMonitor` + `SetCollectWorkersRunning`**；管理端 **`/collect/monitor`**（**5s**、**visibility** 暂停、失败任务 **Drawer**）；**`/collect/batches?batchId=`**、**`/collect/tasks?batchId=`** 深链；**§7 遗留 / §8 下一步** 按监控收尾后重排 |
| 2026-05-16 | **批量采集**：**`collect_batches`** + **`collect_tasks.batch_id`**；**`POST /api/v1/collect/batches`**（**URL 裁剪/去重、默认最多 50 条 `COLLECT_BATCH_MAX_URLS`、先入队失败后整批回滚**）；**批次列表 / 详情 / 子任务** API；任务列表 **`batchId`** 筛选；**Worker 与各阶段状态变更后以 `GROUP BY status` 重算批次**，**不设并发 +-1**；管理端 **`/collect/batches`**（**5s 轮询**、抽屉内任务列表 + **批次快照刷新**）；操作日志 **`collect.batch.create`** / **`collect.batch.retry_failed`**；**.env.example** 补 **`COLLECT_BATCH_MAX_URLS`**；**§7/§8** 对齐下一步与遗留 |
| 2026-05-16 | **管理员登录**：仅 **邮箱或手机号 + 密码**（不再接受用户名）；首启账号通过 **`ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PHONE`**（至少一项）配置；`admin_users.username` 为内部不透明 ID；`docs/PROGRESS`、`.env.example` 同步 |
| 2026-05-16 | **邮箱注册与通知**：UI 增加登录注册 Tab 切换与设计稿对齐；管理端增加 **Email 邮箱设置** 页并可测试连接（`test-email`）；后端实现 **Email Provider（SMTP）** 与 settings 写入，密码 AES-GCM；扩展 `admin_users` 邮箱与 **`account`** 登录链路；验证码限流与 TTL；注册入库并自动登录 |
| 2026-05-16 | **管理端 UI**：Ant Design 主题与 **mix 布局**（顶栏+侧栏）、登录分区样式、工作台快捷入口；各页去掉冗余说明与 Alert；与 PROGRESS 同步 |
| 2026-05-16 | **采集任务异步化**：Redis **`collect:tasks`**（`COLLECT_QUEUE_*`）；**Worker** 消费、`RunCollectJob`、**`operationlog.WriteBackground`**；`POST /collect/tasks` **非阻塞**；**retry 再入队**；**503** `Redis queue unavailable`；**main** 优雅关闭；管理端 **轮询** 与文案；**`ImportDraftWithContext`**；PROGRESS 同步 |
| 2026-05-16 | **1688 Collector 结构化解析**：`collector/src/providers/source1688/` 分拆 **parser/selectors/utils**；抽取 **标题/主图(≤10)/详情图(≤30)/attributes/skus**（`properties` 兼容 Go **`BuildImportSKU`**），**SKU 粒度 `raw`**；**顶层 `raw`** 结构化（候选图/属性/SKU、`pageMeta`、`extractedAt`、snippet 摘要，**不含整 HTML**）；**`pnpm collect:test`**；验证码且零字段时 **`INVALID_URL`**；PROGRESS §4.3/遗留/下一步更新 |
| 2026-05-15 | **商品详情编辑增强**：后端 **`PUT /products/:id`**（camelCase/snake_case、**status 枚举**、不写 source/raw）；**SKU / images / reorder API**；**操作日志 `product.sku.*` `product.image.*`**；前端 **`DraftDetail`** **Tabs + 图片 ModalForm + 可编辑 SKU**；采集入库详情图 **`detail`**；**PROGRESS** 同步遗留与下一步 |
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