# TradeMind 开发进度记录

> **用途**：记录仓库当前真实进度，供后续会话（含 Cursor）快速对齐上下文，避免重复造轮子、偏离架构或漏掉已做决策。  
> **维护规则**：每完成一个**阶段**、一个**独立模块**，或一次**较大的代码修改**后，须同步更新本文件（含日期与变更摘要）。

**最后更新**：2026-05-16（**采集 Provider 通用化**：**Collector/List + Go JWT 代理 **`/collect/providers`**、管理端 **`/collect`** 入口；**内部手工订单 + 会话关联**：表 **`orders` / `order_items` / `order_shipments`**；**JWT** **`/api/v1/orders*`** CRUD（头表软删）与 **明细行 / 发货**嵌套写入；**`customer_conversations.order_id`**、`PUT …/conversations/:id` **`orderId`（含 `""` 解绑）**、详情 **`orderSummary`**；**`customer_reply_generate`** Prompt 增补 **`{{orderInfo}}`**、**`{{orderItems}}`**、**`{{shipmentInfo}}`**、**`{{customerProfile}}`**，生成时注入关联订单快照与 **风险调高**逻辑；操作日志 **`order.*`、`customer.conversation.link_order`**；管理端 **`/orders`**、`services/orders.ts`；工作台 **选单关联 / 展示订单与物流概要 + Alert 风险提示**；**COS/OSS 云存储**等见历史记录。）

---

## 1. 当前阶段

| 维度 | 状态 |
|------|------|
| **路线图阶段** | **第 5 阶段（采集）**保持；**第 6 阶段（AI 图片）**保持（见 §3.2）；**AI 客服 MVP（§3.2 `customerchat`）**：手工会话 + **AI 建议回复**（**不自动外发**） |
| **MVP 闭环** | 登录 → 配置 AI → 采集/草稿 → **AI 标题/描述** → **可选：录入客服消息 → 关联内部手工订单（可选）→ AI 生成建议（含订单/物流快照）→ 人工采纳（仅入库，不外发）**（依赖有效大模型） |
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
- **迁移**：启动时 `database.AutoMigrate` — `admin_users`、`settings`、**`operation_logs`**、**`files`**、**`products`、`product_images`、`product_skus`、`image_tasks`**、**`orders`、`order_items`、`order_shipments`**、`collect_batches`、`collect_tasks`、`collect_task_events`、**`ai_prompts`、`ai_tasks`**、**`customer_conversations`**（含可空 **`order_id`**）、**`customer_messages`、`customer_reply_suggestions`**；启动后 **`aiprompt.EnsureDefaults`**（含 **`migrateCustomerReplyGenerateOrderContext`** 对存量 **`customer_reply_generate`** 占位符升级）、写入默认 **`product_title_optimize`**、**`product_description_generate`**、**`customer_reply_generate`**（各仅在缺失时插入）；**`settings.EnsureImageDefaults`** 写入 **`image` 分组**默认键（**`provider=noop`**；**`removebg_*`**；**`openai_image_*`**；**`comfyui_base_url`（`http://127.0.0.1:8188`）**、**`comfyui_api_key`（加密，可选）**、**`comfyui_workflow_json`（json，空）**、**`comfyui_prompt_node_id` / `comfyui_image_node_id` / `comfyui_output_node_id`**、**`comfyui_timeout_sec`（180）**、**`comfyui_poll_interval_seconds`（2）**、**`comfyui_max_poll_seconds`（180）**；**`timeout_sec`**；**仅缺失键插入**）
- **Redis**：`internal/rdb`（go-redis），连接失败 **仅告警**，服务继续（健康检查体现 `redis: skipped/degraded`）。
- **健康检查**：`GET /health`、`GET /api/v1/health`（含 DB/Redis 检查；**`data.collectQueue`**：`enabled`、`name`、`redisAvailable`、`depth`、`workerEnabled`、`workerConcurrency`；**`data.imageQueue`**：**`enabled`、`name`、`redisAvailable`、`depth`、`workerEnabled`、`workerRunning`、`concurrency`**；**不对 Collector 做 HTTP 探测**以免拖慢健康接口）。队列开启且 Redis **Ping 正常但 LLEN 不可得**时整体 **`status` 可标记 `degraded`**。
- **ID 约定**：管理员等域表主键 **UUID**（`internal/pkg/model` + `internal/pkg/id`；GORM `char(36)`）；`settings` 表为 **`BIGINT` 自增**（与规则文档一致）。
- **认证**：`admin_users` 模型；`POST /api/v1/auth/login`（bcrypt 口令、**JWT HS256**）；`GET /api/v1/auth/profile`、`POST /api/v1/auth/logout`（无状态，客户端弃 token）。
- **JWT 上下文**：`BearerAuth` 写入 `ctxkey.AdminID` 与 **`ctxkey.AdminUsername`**（供审计与业务使用）。
- **操作日志**：`operation_logs` 表；模块 `internal/modules/operationlog`；**`GET /api/v1/operation-logs`**（分页；**action / username / resource / start / end（RFC3339）** 筛选）。写入场景：**登录成功/失败**、**logout**、**settings 批量保存成功/失败**、**test-ai / test-storage 成功/失败**（消息不落敏感配置明文）；**采集**：…；**商品**：…；**AI 标题**：…；**AI 描述**：…；**内部订单**：**`order.create` / `order.update` / `order.delete`（软删）**、**`order.item.{create|update|delete}`**、**`order.shipment.{create|update|delete}`**；**客服（MVP）**：**`customer.conversation.create` / `customer.conversation.update` / `customer.conversation.close`（含软删会话）**、**`customer.conversation.link_order`**、**`customer.message.create`**、**`customer.conversation.replied`**、**`customer.reply_generate.success` / `customer.reply_generate.failed`**、**`customer.reply_suggestion.edit` / `accept` / `discard`**（**不落客户正文全量**；可含 **conversationId / suggestionId / platform / 长度摘要**）；**图片任务**：…
- **存储 Provider**：`internal/providers/storage` 接口 **Put / GetURL / Delete / Get**（调用方 **`Close` Get 返回体**）；**本地** `providers/storage/local`；**S3 兼容** `providers/storage/s3store`（**AWS SDK v2**）；**腾讯云 COS** **`providers/storage/cos`**（**`cos-go-sdk-v5`**）；**阿里云 OSS** **`providers/storage/oss`**（**`aliyun-oss-go-sdk`**）；**工厂** **`settings.storage.kind`** → **`local` / `s3` / `r2` / `minio` / `cos` / `oss`**（按 **`PlainByGroup`** 解密：**`s3_*`、`cos_*`、`oss_*`** 密钥）；**对象键** **`keypath.NormalizeSafe`** 防 **`..` 与非法路径段**；**不写密钥到日志**。**`GET /static/*filepath`** 仍仅 **`kind=local`**。
- **文件**：`files` 表；**`POST /api/v1/files/upload`**（`multipart` 字段名 **`file`**；**jpg/jpeg/png/webp/gif**；**objectKey = 日期目录/UUID.ext**；**`storage_kind` 落库**，**云端 `public_url`** 取自 **`GetURL`**）；**本地行为不变**。**`GET /api/v1/files`**（分页、`contentType`）；**`DELETE /api/v1/files/:id`**（**先做 Provider.Delete（按 `storage_kind`，缺省回落当前 settings.kind）**，成功后再删 DB，避免「库删了对象残留」）。
- **配置中心**：`settings` 模型与 `GET/PUT /api/v1/settings`；`item_value` 在 `is_encrypted=true` 时 **AES-GCM**（`APP_MASTER_KEY`）存储；列表接口 **脱敏**（`****` 规则）；PUT 若密文占位含 `****` 则 **不覆盖**原密钥，可更新 remark / value_type 等。
- **连通性测试**：`POST /api/v1/settings/test-ai`（…）；`POST /api/v1/settings/test-storage`（**`local`** 目录可写；**`s3`/`r2`/`minio`** **HeadBucket + 短时 context**；**`cos`** **COS Bucket HEAD**；**`oss`** **OSS `ListObjects` MaxKeys=1**；**均无真实 PUT 测试对象**。`storage` 优先 **`s3_*`**（兼容遗留 **`endpoint`/`bucket`/`access_key`/`secret_key`/`region`**）。
- **默认管理员**：库中无管理员时，按 **`ADMIN_BOOTSTRAP_EMAIL` 与/或 `ADMIN_BOOTSTRAP_PHONE`**（至少填一项）及 `ADMIN_BOOTSTRAP_PASSWORD`（**非 production** 空密码 Fallback `changeme` 并告警；**production** 无用户则必须配置密码）插入一条记录；**不支持用「用户名」登录**，仅邮箱或手机号 + 密码；内部 `username` 列为随机 ID，由实现自行分配。
- **商品草稿**：模块 `internal/modules/product`；模型含 **`tenant_id`、`created_by`、JSONB `raw_data`** 及 **`product_images` / `product_skus`**（SKU **`attrs`、`raw_data` JSONB**，**前端不可改 raw_data**）。**列表**：**`GET/POST /api/v1/products`**；**详情**：**`GET/PUT/DELETE /api/v1/products/:id`**；**`PUT` 可编辑**：`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；**一并支持 JSON snake_case**（如 `original_title`、`ai_title`）；**不写** `id` / `created_by` / `created_at`；**不通过 PUT 修改** `source` / `source_url` / `raw_data`（采集来源与归一快照只读）。**`status`** 枚举校验：`draft` / `ai_processing` / `ready` / `published` / **`archived`**。删除仍为 **软删除**（`products.deleted_at`）。**子资源**：**`POST/PUT/DELETE /api/v1/products/:id/skus`、`:skuId`**；**`POST/POST(reorder)/PUT/DELETE /api/v1/products/:id/images`、`:imageId`、`/images/reorder`**；图片 **`image_type`** 支持 **`main` / `detail` / `sku`**（并接受旧值 **`description` 归入 detail**）；**可按 `files.id`（`fileId`）关联本地上传**；**删除 `product_images` 仅断开关联**，默认 **不删除 `files`** 存储对象。**采集入库**：新草稿详情图默认写入 **`detail`**（历史中可能仍有 **`description`** 行）。
- **AI Provider**：`internal/providers/ai` — **`ChatRequest` / `ChatResponse`**、**`Gateway`**（只读 **`settings.ai`**：`provider`（仅 **`openai_compatible` / `openai`** 首版落地）、`base_url`、`api_key` AES-GCM 解密、`model`、`temperature`、`max_tokens`、`timeout_sec`）；**业务仅调 Gateway**；**`openai_compatible/`** 实现 HTTP **`/chat/completions`**，**Context 超时** + **http.Client Timeout**；日志与响应 **不落 api_key**
- **Prompt 模板**：模块 `internal/modules/aiprompt`；表 **`ai_prompts`**；**`EnsureDefaults`** 插入默认 **`product_title_optimize`**（变量含 **`{{title}}` `{{category}}`…**）、**`product_description_generate`**（变量 **`{{title}}`… `{{tone}}`**，JSON 输出 **description / highlights / …**）与 **`customer_reply_generate`**（**scene `customer_service`**；变量 **`{{customerMessage}}` `{{conversationHistory}}`**、遗留 **`{{productInfo}}`**、**结构化 `{{orderInfo}}`、`{{orderItems}}`、`{{shipmentInfo}}`**、**`{{customerProfile}}`**、 **`{{language}}` `{{tone}}` `{{platform}}` `{{shopPolicy}}`**；内置 System **禁止捏造物流/退款事实**并强调 **UNKNOWN**；JSON 输出 **reply / intent / sentiment / riskLevel / notes**）；API：**`GET/POST /api/v1/ai/prompts`**、**`GET/PUT/DELETE .../:id`**、**`POST .../:id/enable|disable`**
- **AI 调用记录**：模块 `internal/modules/aitask`；表 **`ai_tasks`**（可选 **`product_id`**、**`conversation_id`** / 客服关联；`task_type` / `provider` / `model` / `prompt_code` / status **`pending|running|success|failed`** 等）；**标题优化**（**`title_optimize`**）、**描述生成**（**`product_description_generate`**）、**客服建议**（**`customer_reply_generate`**）各写一条；**`raw_response`** 仅存提供商返回 JSON 裁剪字段，**不含密钥**。**只读查询**：**`GET /api/v1/ai/tasks`**（分页；筛选 **taskType / status / provider / model / promptCode / productId / conversationId / start|end（RFC3339）**；列表 **不返回** `input`/`output`/`raw_response`）；**`GET /api/v1/ai/tasks/:id`**（详情含 **input/output/rawResponse**，响应前对 JSON 内 **api_key 等敏感键** 做 **`[REDACTED]`** 脱敏）；均 **JWT**、统一 **envelope**
- **内部手工订单（无平台对接）**：模块 **`internal/modules/order`**；表 **`orders`**（订单号唯一、枚举 **status/paymentStatus/fulfillmentStatus**、可选 **shopId / externalOrderId / rawData**、金额与时标、软删）、**`order_items`**（可关联 **`products`/`product_skus`** 的快照行）、**`order_shipments`**（承运商、追踪号、`status`、`trackingUrl` 等）；**JWT API**：**`GET/POST /api/v1/orders`**（列表筛选 **`orderNo` / `customerName` / …**）、**`GET/PUT/DELETE /api/v1/orders/:id`**；嵌套：**`POST/PUT/DELETE …/:id/items(/:itemId)`**、**`POST/PUT/DELETE …/:id/shipments(/:shipmentId)`**；详情 **Preload 行与子表**。**`ConversationSummary`** + **`BuildAIContext`**：供会话详情与 **`generate-reply`** 拼装 **`orderInfo` / `orderItems` / `shipmentInfo`**；**不涉及** TikTok/Shopee 等平台订单同步。
- **AI 客服（MVP，不外发）**：模块 **`internal/modules/customerchat`**；注入 **`Orders *order.Service`**；表 **`customer_conversations`**（同上 + 可空 **`order_id` → 内部手工订单 UUID**）、**`customer_messages`**、**`customer_reply_suggestions`**（不变）。**`GET …/conversations/:id`** 带出 **`orderId`/`orderSummary`（简略 + 多条 `shipments`）**。**PUT …/conversations/:id**：**`UpdateConversationBody`** 可选 **`orderId`**（UUID 绑定；**空字符串解除关联**）；**不存在订单**报错；仅改关联时 **`metaChanged`**，避免与一般 update 日志重复。**`POST …/:id/ai/generate-reply`**：若已关联订单，拼装 **订单/明细/发货 JSON** 写入 Prompt 占位符与 **`ai_tasks.input` 子集**（脱敏：`customer_profile` **不含邮箱/电话**）；**`adjustCustomerReplyRisk`** 遇 **退款/取消/高风险物流空**等 **调高 riskLevel**。**其余 API**：同前。**管理端**：`/customer/conversations`、`/customer/conversations/:id`，**工作台** Modal 检索 **`queryOrders`** 绑定订单 + **风险提示 Alert**。**`services/customer.ts`** 扩展 **`updateConversation(..., orderId?)`**。
- **AI 图片任务**：模块 **`internal/modules/imagetask`**；表 **`image_tasks`**（**`task_type`**：`remove_background` / …；**`status`**：**`pending` / `running` / `retrying` / `success` / `failed` / `cancelled`**；**`retry_count` / `max_retries` / `next_retry_at` / `retry_enqueued_at`**；**JSONB `input` / `output`**；**`source_image_id`**：**`files.id` / `product_images.id`**；**源解析**：**`source_resolver.go`** — 优先 **`storage_kind` + `object_key` → Provider `Get`**；失败则 **`public_url`/`origin_url`：`httppublic.IsPublicHTTPURL` → remove.bg `image_url`**；**`/static/...` 或 loopback `/static/...` → 本地 `object_key`**（见 §7 风险）；**`source_image_url`**：公网则 `image_url`，否则静态映射 / **`files.public_url` 精确匹配**再 `Get`；**`result_file_id` / `result_url`**；**不落库源图二进制**）。**Image Provider**：**`internal/providers/image`**：**`noop`**；**`removebg`**（**`image_file` 优先，其次 `image_url`**；**`internal/pkg/httppublic`**）；**`openaiimage`**；**`comfyui`**（**`POST /prompt`、`/history`、`/view`、`/upload/image`**；**日志不打 API Key / 完整 workflow**）；**`factory.NewForTask`：`noop` | `removebg` | `openai_image` | `comfyui`**，读 **`settings.image`**（密钥 **解密不写日志**，**不回退 `settings.ai.api_key`**）。**`remove_background`**：**强制 `provider=removebg`**。**`generate_scene` + `openai_image`**：**`prepareGenerateSceneHints` → `assembled_prompt`**；**+ `comfyui`**：同 **hints** + **模板变量**。**`generate_scene`**：**`openai_image` / `comfyui` 可无源图**。**`replace_background`**：**`openai_image`**（**后端 `resolveRemoveBGSource` → multipart `/images/edits`**；**`prepareReplaceBackgroundHints` → `assembled_prompt`**）；**`comfyui`**（须 **workflow + output 节点**；**`image` 节点** 用于上行源图）。**`IMAGE_QUEUE_ENABLED` + Redis**、**Worker**（认领 **`pending`** 或已调度入队的 **`retrying`**，条件 **`next_retry_at IS NULL`**）、**503 回滚**、**`GET /api/v1/image/tasks/monitor`**（**`retry` / `recentRetrying` / `recentFailures`**）、**人工 retry**；**`IMAGE_AUTO_RETRY_ENABLED`**（**.env 默认 true**；**`IMAGE_MAX_RETRIES` / `IMAGE_RETRY_*_DELAY`**）与 **`StartImageRetryScheduler`**（约 **5s**、到期 **CAS** **`LPUSH`** **`image:tasks`**）；可重试错误 **`IsRetryableImageTaskError`**（**5xx** / **429** / 超时 / 网络类；**缺 Key、workflow/JSON、源图不可读且非公网、`not implemented` 等**不重试）；操作日志 **`image.task.create` / `retry` / `success` / `failed` / `auto_retry_scheduled` / `auto_retry_enqueued` / `retry_exhausted`**（**不写密钥与完整 workflow**）；**Comfy 成功 `output`**：**`promptId`/`workflow`（空）** 等；**执行超时** 对 **`comfyui`** 不低于 **`comfyui_max_poll_seconds` + `comfyui_timeout_sec`**，再与 **`IMAGE_TASK_TIMEOUT_SECONDS`** 取 cap。**管理端**：**`/settings/image`**（**ComfyUI 大文本 workflow**）、**`/ai/image-tasks`（可选 `sourceImageId`；`replace_background` + `openai_image` 文案提示后端代传）**、**商品详情图片 Tab**（**`replace_background`：`openai_image` / `comfyui`**）。**其它**：noop **resize/enhance**；removebg **仅 remove_background**；openai **`generate_scene` + `replace_background`**。
- **商品 AI 标题**：**`POST /api/v1/products/:id/ai/optimize-title`**（body：`language` / `platform` / `maxLength`；**不自动改 `title`**）；**`POST /api/v1/products/:id/apply-ai-title`**（`aiTitle` + `taskId`，校验任务归属，**仅更新 `products.ai_title`**）；操作日志：**`ai.title_optimize.success` / `ai.title_optimize.failed` / `ai.title.apply`**（消息 **不含密钥与完整 Prompt**）
- **商品 AI 描述**：**`POST /api/v1/products/:id/ai/generate-description`**（`language` / `platform` / `tone`，默认 en / TikTok Shop / professional；**Preload `images`+`skus`**；**不自动改 `products.description`**）；**`POST /api/v1/products/:id/apply-ai-description`**（`aiDescription` + `taskId`，**仅更新 `products.ai_description`**）；**`GET /api/v1/products/:id/ai/tasks`**（详情页最近任务，列表 **省略大体量 JSON 列**，含 **`title_optimize`** 与 **`product_description_generate`**）；操作日志：**`ai.description_generate.success` / `ai.description_generate.failed` / `ai.description.apply`**（同上）
- **采集任务与批次**：模块 `internal/modules/collect`。表与 **`collect_task_events`**、`GET …/tasks/:id/events`、`COLLECT_*` Worker/队列、`GET …/monitor` **与历史一致**。新增 **Provider 驱动契约**：**`GET /api/v1/collect/providers`**（JWT，优先 **`Collector` `GET /v1/providers`**，失败用 **内置兜底**）；**POST …/collect/tasks** 仅 **`provider.status===available`**；**POST …/collect/batches** 额外要求 **`batchSupported`**；**`source`** **大小写不敏感**对齐注册表；**URL** 仅接受 **`http`/`https`**，平台语义交给 Collector **`canHandle`**。**Collector 即时失败码**：**`INVALID_REQUEST`/`INVALID_URL`/`PROVIDER_NOT_FOUND`/`PROVIDER_NOT_IMPLEMENTED`/`PAGE_BLOCKED_OR_VERIFY_REQUIRED`** → **不进行自动退避重试**；**`COLLECT_FAILED`/`NAVIGATION_FAILED`**** 等**仍按 **`COLLECT_*` Retry** 阶梯执行。
- **Collector HTTP 客户端**：`collector_client.go`：沿用 **`POST /v1/collect`**；新增 **`FetchProviders`** 调 **`GET /v1/providers`**（**≈3s**，与单笔采集 **`http.Client.Timeout`**** 解耦）；422/`ok:false` → **`CollectorRejectedError`**；成功 **`raw_result`** 写 **`NormalizedProduct` JSON**。
- **分层**：业务 Orchestration 在 **collect.Service**，采集解析仍在 **Node Collector**；Go **不写死** 1688 解析逻辑。

### 3.3 管理端（`admin/`）

- **@umijs/max**（脚本使用 **`max`**，禁止用 **`umi`** 跑 Max 配置，否则配置键会报错）。
- **登录与鉴权**：`/user/login` 调用 `POST /api/v1/auth/login`；**JWT** 存入 `localStorage`（`AUTH_TOKEN_KEY`）；**`request` 拦截器**自动附加 `Authorization: Bearer`；**HTTP 401**（除登录请求外）清 token 并 **整页跳转**登录页带 `redirect`；**`access.canAdmin`** 控制侧栏与业务路由；**`getInitialState`** 用 token 拉取 `/api/v1/auth/profile`。
- **布局**：右上角展示当前管理员与**退出**（`POST /auth/logout` + 清 token + 更新 initialState）。
- **系统 / AI / 存储 / 采集服务 / 安全 / 图片 AI** 设置页：`GET/PUT /api/v1/settings`，按 **groupKey**（`system`、`ai`、`storage`、`collector`、`security`、**`image`**）读写 **snake_case** **`itemKey`**；**`image` 分组**详见 §3.2 后端条目；敏感项 **脱敏 + masked PUT 语义**。**存储页**：**`kind` 切换**（**本地** `public_base`/`local_root`；**`s3`/`r2`/`minio`**：**`s3_*`** — Endpoint/Region/Bucket、密钥 **密码输入 + 脱敏 + masked 不覆盖**、Path-style、`s3_use_ssl`、可选 **`s3_presign_*`、`s3_public_base`**；**COS**：**`cos_bucket`/`cos_region`/`cos_secret_*`**（可选 **`cos_app_id`/`cos_endpoint`/`cos_public_base`/`cos_use_https`**）；**OSS**：**`oss_endpoint`/`oss_bucket`/`oss_access_key_*`**（可选 **`oss_public_base`/`oss_use_https`**）— **前端不经由浏览器直连 COS/OSS**。**测试**：`test-ai` / **`test-storage`（local / S3-compat HeadBucket / COS HeadBucket / OSS List(max1)）**；**上传测试** **`POST /api/v1/files/upload`**（亦 **不经由浏览器直连第三方 AI/Image**）。
- **存储页保存策略**：按当前 `kind` 仅提交相关键（**local / s3compat / cos / oss** 各一套字段）。
- **操作日志页**：**`ProTable`** → **`GET /api/v1/operation-logs`**；只读、可筛选。
- **文件管理页**：**`ProTable`** → **`GET /api/v1/files`**；图片预览；删除 **`DELETE /api/v1/files/:id`**。
- **开发代理**：`.umirc.ts` 将 **`/static`** 代理到后端，便于 **`public_base=/static`** 时预览。
- **商品草稿**：路由 **`/product/drafts`**，`ProTable` → **`GET /api/v1/products`**；**`/product/drafts/:id`** **Tabs**（基础、AI 标题/描述、**图片管理**（上传、`createProductImage`、**AI 图片任务**：**resize/noop**、**remove_background**、**replace_background（`openai_image` / `comfyui`）**、**generate_scene（openai_image / comfyui）**、Prompt/背景/style、**可无源场景图**、异步提示 + **`/ai/image-tasks`**、reorder）、**SKU 表**、最近 AI 任务）；**`/ai/prompts`**、**`/ai/tasks`**、**`/ai/image-tasks`**（约 **4s** 轮询、**`document.visibilityState` 隐藏时暂停**；**新建任务可选 `sourceImageId`**）。**`products.ts` / `imageTasks.ts`** 封装 API。
- **采集**：侧栏分组 **采集**：**`/collect`** **采集中心**（多采集器卡片、状态 Tag、能力与 URL 示例、跳转 **`/collect/tasks|batches`**）、**`/collect/tasks`**（**采集平台下拉**来自 **`GET /api/v1/collect/providers`**，**不可用项禁用**）、**`/collect/batches`**（仅 **batchSupported 且可用**的平台）、**`/collect/monitor`**；批量 **多行链接**与 **5s** 轮询；**采集任务 Timeline** **`CollectTaskEventDrawer`** **不变**；**`services/collectProviders.ts`** + **`collectTasks`** / **`collectBatches`** / **`collectMonitor`**。
- **内部订单**：路由 **`/orders`**；**ProTable + Drawer**：头表表单、Tabs **明细行（`order_items`）与发货（`order_shipments`）** 及独立新增/编辑 Modal；状态 Tag 映射 **`ORDER_*`**；**`services/orders.ts`**。
- **客服（AI MVP）**：路由 **`/customer/conversations`**（**ProTable** 会话列表 + 新建）与 **`/customer/conversations/:id`**（**关联订单卡片**【选单 Modal / 解绑确认】、**订单与物流概要**；**消息时间线 + AI 建议区**：**风险提示 Alert**、生成 / 编辑 / 采纳 / 废弃 / 复制；**不自动外发**）；**`services/customer.ts`**。
- **常量**：`src/constants/status.ts`（商品状态、**采集任务 / 批次**状态枚举、**订单与支付 / 履约 / 发货**）。

### 3.4 采集服务（`collector/`）

- **Playwright + TypeScript**，独立进程，**不直连主业务库**。
- **`CollectorProvider` 接口**（含 **`meta`**：名称、**`status`**、**`batchSupported`**、**`urlPatterns`**、**`features`**）+ **有序注册表**；**1688** **`available`**、**结构化解析不变**（非法 offer / 非 1688 仍 **`INVALID_URL`**；人机验证且无字段时用 **`PAGE_BLOCKED_OR_VERIFY_REQUIRED`**）。**占位 Provider**：**`pdd` / `taobao` / `aliexpress` / `shein_temu` / `custom`**，`collect` 返回 **`PROVIDER_NOT_IMPLEMENTED`**，**不伪造 **`NormalizedProduct`****。**统一错误码**： **`COLLECT_FAILED`、`PROVIDER_*`、`INVALID_*` 等**，`runCollectTask` **前缀映射**。
- **任务编排**：`runCollectTask`（唯一 HTTP 编排入口）。
- **HTTP**：`GET /health`（契约不变）；**`GET /v1/providers`**（注册表 **`listProviderPublicMetas()`**）；`POST /v1/collect`（body：`source` + `url`）。
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
- [x] **迁移**：启动时 GORM **AutoMigrate**（地基表 + **商品 / 采集** + **`ai_prompts` / `ai_tasks` / `image_tasks` / 客服三表**）
- [x] **操作日志**：表 + **`GET /api/v1/operation-logs`**；登录 / logout / settings / test-ai / test-storage / **采集关键节点 / 商品 CRUD / AI 标题与描述 / 图片任务 / 客服 MVP（`customer.*`）**
- [x] **对象存储与文件 API**：Storage **factory**（**`local` + `s3`/`r2`/`minio`（S3 兼容）+ `cos` + `oss`（独立 SDK）**）**Put / GetURL / Delete / Get**；**`POST /api/v1/files/upload`**（**`storage_kind` 入库**）；**`GET/DELETE /api/v1/files`**（删 **云端对象先于 DB**，按 **`files.storage_kind`**）；**`/static`** 仅 **本地** 只读
- [x] **settings 连通性测试**：`test-ai`、`test-storage`（**local·S3-compat·COS·OSS**；见 §3.2）
- [x] **商品草稿 API**：§3.2 **商品草稿**；**AI 标题与 AI 描述** 生成/应用 / 任务列表（见上）
- [x] **采集任务 API + Collector Client**：§3.2 **采集任务** / **Collector HTTP 客户端**
- [x] **AI 文本（标题+描述 + 客服建议）**：§3.2 **AI Provider / Prompt / ai_tasks**（含 **`customer_reply_generate`**、**`conversation_id`**、**`GET /ai/tasks?conversationId=`**）/ **商品 AI 标题与 AI 描述** / **AI 客服 MVP**（§3.2）
- [x] **AI 图片任务**：**remove.bg `image_file` + `image_url`** + **OpenAI Image `generate_scene` + `replace_background`（multipart `/images/edits`）** + **ComfyUI**（**`generate_scene` / 基础 `replace_background`**）+ **Redis Worker** + **自动退避重试**（详见 §3.2）

### 4.2 管理端

- [x] 登录页与 **access 模型**（@umijs/max access）；**Bearer** 请求拦截与 **401** 处理
- [x] **系统 / AI / 存储 / 采集服务 / 安全 / 图片 AI** 设置页与 **settings API**；**test-ai / test-storage**（**local · S3-compat · COS · OSS**）；**存储页 COS/OSS 独立表单 + `s3_*` + 本地**；**上传测试** **`/files/upload`**；**操作日志**与 **文件管理**（**ProTable**）
- [x] **商品草稿 / 采集任务（含批量采集 `/collect/batches`）**：分页列表 API、筛选、**单链接**与**批量**表单；失败 **重试** / 批次 **重试失败**
- [x] **Prompt 模板页（`/ai/prompts`）**；**商品详情编辑页（`/product/drafts/:id`）**：**Tabs**（基础表单、保留 **AI 标题/描述** 弹窗、**图片 ModalForm/Reorder** 与 **AI 图片任务入口**、**SKU `EditableProTable`**、最近 AI 任务）；**`/ai/tasks` 全局 AI 任务记录页**（**`conversationId` 列**，可见 **`customer_reply_generate`**）；**`/ai/image-tasks` 图片任务页**；**`/settings/image` 图片 AI 设置**；**`/customer/conversations` + `/customer/conversations/:id`** **客服工作台**

### 4.3 采集服务

- [x] 1688 **结构化解析落地（首版）**：主图 **`mainImages`**、详情 **`descriptionImages`**、**`attributes`**、**`skus`**（含 **`properties`/价格/库存/可选图**）；**降级不抛解析异常**（仅非法 URL、导航失败、非 offer 跳转、验证码页且全无结构化字段时 **`INVALID_URL`** 失败）。
- [ ] **反爬与稳定性深化**（人机验证绕过、SKU 多维长期可用、异步详情 iframe 全覆盖等）。
- [x] 与 **Go 任务编排**对接：**HTTP 异步队列**（Redis list + Worker **`POST /v1/collect`**），由 Go 写 **`collect_tasks`** 与 **`products`**（Collector **不写主库**）

### 4.4 跨模块

- [x] **Go ↔ Collector**：HTTP **`POST /v1/collect`**（Worker，`NormalizedProduct` 不变）；**`GET /v1/providers`** 元数据；422/`ok:false` → 任务 **`failed`/退避策略**（见 §3.2）。
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
│       ├── providers/       # storage（**local + s3store**）+ **ai（Gateway + openai_compatible）** + **image（noop + removebg/openai client + factory）**
│       └── modules/         # auth、admin、settings、**operationlog**、**files**、**product**、**order**、**collect**、**aiprompt**、**aitask**、**imagetask**、**customerchat**
├── admin/                   # Ant Design Pro（Umi Max）
│   ├── .umirc.ts            # 含 proxy `/api` 与 **`/static`** → 8080
│   ├── config/routes.ts
│   └── src/
│       ├── pages/           # … **Collect/Hub**、**Collect/Tasks**、**Collect/Batches**、**Collect/Monitor** …
│       ├── services/        # … **`collectProviders`**、**`collectTasks`**、**`collectBatches`**、**`collectMonitor`**、**`imageTasks`** …
│       └── constants/       # 状态枚举
├── collector/               # Node 采集（Playwright）
│   └── src/
│       ├── browser/         # BrowserManager
│       ├── providers/       # `registry` + **source1688** + **stub/placeholders**（**meta**、`/v1/providers`）
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
| 文件存储（MVP） | **上传到后端**；**object_key / public_url**；**factory**：**`local` / `s3` / `r2` / `minio`**（AWS SDK **S3 兼容**）+ **`cos`（COS SDK）+ `oss`（OSS SDK）**；**非公网可读 Bucket** 依赖 **`*_public_base`（CDN/静态站）或后续按需签名 URL** |
| AI 文本（扩展） | **标题/描述/客服建议** 均走 **`AI Gateway`** 与 **`ai_tasks`**；**`customer_reply_generate`** **不写入/不返回第三方 API Key**；**采纳建议**仅写入 **`customer_messages`（`role=agent`, `source=manual`）**，**不调用任何平台外发 API** |
| AI 图片 | **`internal/providers/image`**：**`noop`** + **`removebg`**（**`image_file` / `image_url`**）+ **`openaiimage`**（**`/images/generations` + `/images/edits` multipart `image[]`**）+ **`comfyui`**（HTTP REST，**结果统一 PNG**）；**factory**：**`noop` \| `removebg` \| `openai_image` \| `comfyui`**；**异步队列 + 自动退避重试**（**`IMAGE_AUTO_RETRY_*`**）；**源解析**：**`imagetask/source_resolver`** + **Storage `Get`**（**`local` / `s3` / `r2` / `minio` / `cos` / `oss`**，`NewFromPlainForStoredKind`）+ **`httppublic.IsPublicHTTPURL`**；**OpenAI Image** 密钥 **`settings.image.openai_image_*`（不回退 `settings.ai.api_key`）**；**ComfyUI `replace_background`**：**可配置 workflow 基础链路** |

## 7. 当前遗留问题 / 风险

1. **401 处理**：采用**整页跳转**登录以清空 initialState；后续可改为无刷新同步 `setInitialState`。
2. **`s3_presign_enabled` 入库 URL**：启用预签名时 **`files.url`** 为**短时有效链接**，过期后预览/外链失效；生产推荐配置稳定 **`s3_public_base`**（或后续做按需重签）。
3. **COS / OSS 外链可读性**：**`files.public_url`** 取自 **`GetURL`**；若 Bucket **非公共读**，缺省 **`*_public_base`（CDN/自定义域名绑定）时外链可能无法在浏览器匿名访问**；（与 **S3 预签名**类似，后续可增强 **COS/OSS 按需签名**。）
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
17. **AI 图片（边界）**：**ComfyUI `replace_background`** 依赖用户 **API 工作流**（**非通用 guarantee**）；**OpenAI `images/edits`** 受 **模型/额度/合规** 与 **源图格式** 约束。
18. **公网 URL 启发式**：**`httppublic.IsPublicHTTPURL`** 按 **scheme/host 字面** 排除 **loopback / RFC1918 / 链路本地** 等；**普通域名默认视为公网**（**不做 DNS**）。若 **`public_base` / 签名 URL 主机名为内网域名但字面非上述范围**，可能被误判为「公网」而走 **`image_url`**（remove.bg 仍不可拉取）；此时应依赖 **`source_image_id` + `Get`** 路径。
19. **AI 客服与内部订单上下文**：工作台已支持 **绑定内部手工订单**（§3.2 **`customer_conversations.order_id`**、`orderSummary`、生成时 **订单/SKU/物流 JSON**）；**仍无** TikTok/Shopee 等平台 **实时消息收发** / **同步平台订单**，**不外发**，**无 WebSocket**。**手工录入**的 **`orders`** 与用户真实收银/库存可能脱节，仅能作 **运营草稿与 AI 上下文**。
20. **多 AI Provider**：**`settings.ai.provider`** 与 Gateway 实际仍以 **openai_compatible** 为主路径；其它厂商后续可加适配。
21. **1688 采集边界 / 反爬稳定性**：虽已 **DOM + script JSON 解析**，仍存在 **SKU 组合不全**、详情图异步、**`/offer` URL 误判**、**人机验证 / 风控**等边界；需在真实流量下持续补强选择与稳定性。
22. **多采集源占位**：拼多多 / **淘宝天猫** / **AliExpress** / **SHEIN·Temu** / **自定义链接** 仍为 **规划中** Collector Provider；**不真实解析**；**Go 不落平台 URL 正则**。**自定义规则编辑器**未完成。
23. **`ai_tasks` / AI 描述**：标题与描述生成均 **`running → success|failed`**；描述任务依赖模型输出 **合法 JSON**；失败写入 **`ai_tasks`** 与操作日志。

---

## 8. 下一步开发计划（建议顺序）

1. ~~**订单/会话上下文**~~：**内部 `orders`** + **`order_id`** + AI Prompt **`order*` 占位符** 已 MVP 落地（见 §3.2）；后续可补强 **租户隔离、`external_order_id` 回填规范、SKU 快照与草稿联动**。
2. **店铺授权 + 平台客服 API**：TikTok / Shopee / Lazada / Amazon 消息 **拉取/发送**（在 **不破坏不外发默认** 的策略上扩展）；**可选：平台订单与内部 `orders` 同步**。
3. **多实例 Worker**：**Redis heartbeat**（采集 / 图片 Worker §7）。
4. **Collector 演进（新源）**：择一 **`planned` Provider** 做 **首个真实结构化解析**（如 **拼多多**或 **AliExpress**）；占位输出 **对齐 **`NormalizedProduct`** 契约**。**自定义链接**的规则配置 **UI + 服务端契约**。**反爬与稳定性**：（SKU 多维、详情 iframe/async、人机验证路由；**逻辑留在 Collector**）。

（细化任务时仍以 `.cursor/rules/09-dev-workflow.mdc` 的阶段为准。）

---

## 9. Cursor 后续开发注意事项

1. **必读**：`docs/PROGRESS.md`（本文件）、`.cursorrules`、`.cursor/rules/*` 中与本任务相关的规则。
2. **开工前**：对照「**已完成 / 未完成**」，确认是否已有接口或占位，**避免重复实现**。
3. **改架构前**：核对「**已确认技术决策**」；若需变更，在本文件与相关架构文档中**写明原因与日期**。
4. **收工后**：若完成一整块功能或一次较大重构，**必须**更新本文件：勾选进度、补充遗留问题、调整「下一步」。
5. **前端**：继续 **`services/` 统一请求**；表格 **`ProTable`**、表单 **`ProForm`**；敏感字段 **脱敏**。
6. **后端**：Handler 薄、Service 编排、**外部调用带超时**；**采集 / 图片任务**均已接 **Redis 列表 + 进程内 Worker**（**可观测指标/告警**仍可加强）。
7. **采集**：新业务逻辑放在 **`collector` 对应 Provider**，**不要**塞进 Go 核心业务层。
8. **本地数据库**：遵守 **`.cursor/rules/11-local-dev-postgres.mdc`**，默认 **PostgreSQL**；勿默认生成 MySQL 专用迁移/compose。

---

## 变更记录（简短）

| 日期 | 说明 |
|------|------|
| 2026-05-16 | **采集 Provider 通用化**：**Collector **`GET /v1/providers`** + 占位 Provider **`pdd`|`taobao`|`aliexpress`|`shein_temu`|`custom`**；Go **`GET /api/v1/collect/providers`**、创建任务 **`available`/batchSupported**、**scheme** 前缀校验与非 **Collector** 报错脱敏兜底；1688 **`PAGE_BLOCKED_OR_VERIFY_REQUIRED`**；管理端 **`/collect` Hub**、`collectProviders.ts`；**PROGRESS §3–§8** 对齐 |
| 2026-05-16 | **内部手工订单 + 客服关联**：**`internal/modules/order`**（**`orders`/`order_items`/`order_shipments`** + JWT CRUD + **`order.*`** 日志）；**`customer_conversations.order_id`**、详情 **`orderSummary`**、**`customer.conversation.link_order`**；**`customer_reply_generate`** 增补 **`{{orderInfo}}`** 等 + **`migrateCustomerReplyGenerateOrderContext`**；生成回复 **载入订单快照与风险调高**；管理端 **`/orders`**、工作台 **选单/解绑与 Alert**；**PROGRESS** 对齐 |
| 2026-05-16 | **AI 客服 MVP**：**`internal/modules/customerchat`** + **三表** + **Prompt `customer_reply_generate`** + **REST API** + **`ai_tasks.conversation_id` / `task_type=customer_reply_generate`** + **管理端 `/customer/*`** + **操作日志 `customer.*`**；**PROGRESS** 全篇对齐 |
| 2026-05-16 | **腾讯云 COS / 阿里云 OSS 独立 Storage**：**`providers/storage/cos`、`oss`** + **factory**（**`kind`** + **`files.storage_kind`**）；**test-storage**：**COS HeadBucket** / **OSS ListObjects(max1)**；**管理端存储页** **COS·OSS**；**`keypath.NormalizeSafe`**；**.env.example / README**；**go.mod SDK** |
| 2026-05-16 | **OpenAI Image `replace_background`**：`openaiimage.ReplaceBackground`（**multipart `image[]` → `/images/edits`**）；**`imagetask` 编排 + `SaveProcessed` + 命名 `openai-replace-bg-*`**；**`output.taskType`**；**retry 分类**；**admin `/ai/image-tasks` + 商品详情**；**PROGRESS** §1/§3/§4/§6/§7/§8 |
| 2026-05-16 | **remove.bg 非公网源图**：Storage **`Get`**（**local / s3 / r2 / minio / cos / oss**）；**`imagetask/source_resolver`** + **`httppublic`**；remove.bg **`image_file`/`image_url`**；重试分类 **`source image is not readable…`** 等 **不可重试**；admin **`/ai/image-tasks`** **`sourceImageId`** + 文案；**PROGRESS** §1/§3/§4/§6/§7/§8 |
| 2026-05-16 | **图片任务自动退避重试**：**`image_tasks`** 重试字段与 **`retrying`**、**`IsRetryableImageTaskError`**、**`retry_scheduler.go`**（**`IMAGE_QUEUE_ENABLED` + `IMAGE_AUTO_RETRY_ENABLED`**）、**Worker** 认领规则、**monitor** **retry** 块、**`image.task.auto_retry_*` / `retry_exhausted`**；**`.env.example` / `config`** **`IMAGE_AUTO_RETRY` 默认 true**；管理端 **`/ai/image-tasks`**；**PROGRESS** §3/§4/§6/§7/§8 |
| 2026-05-16 | **ComfyUI Image Provider**：**`providers/image/comfyui`**（prompt/history/view/upload、变量替换、`generate_scene` + 基础 **`replace_background`**）；**`settings.image` / EnsureImageDefaults** 补全 **`comfyui_*`**；**`factory` / Worker / `files.SaveProcessed`**；**管理端** `/settings/image`、`/ai/image-tasks`、**商品详情**；**`golang.org/x/image/webp`**；PROGRESS 全篇对齐 |
| 2026-05-16 | **OpenAI Image Provider**：**`providers/image/openaiimage` HTTP Client + `factory` 适配 **`openai_image`**；**`settings.image`** 增补 **`openai_image_*`** 默认种子；**`generate_scene`**（**assembled_prompt**、可无源、`output.model`）；**`/settings/image`、`/ai/image-tasks`、商品详情图片 Tab** 联动；PROGRESS §1–§8 对齐 |
| 2026-05-16 | **图片任务异步化**：**`image:tasks`** + **`imagetask/worker`**（**`IMAGE_QUEUE_*`** / **`IMAGE_TASK_TIMEOUT_SECONDS`**）；入队 **`pending`** / **retry** / **503 队列不可用** / **`IMAGE_QUEUE_ENABLED=false`** 同步、`/image/tasks/monitor`、`/health` **`imageQueue`**；admin **轮询** |
| 2026-05-16 | **remove.bg**：**`providers/image/removebg` Client** + **`factory.NewForTask`**（noop/removebg）；**`settings.image`** **`removebg_base_url`** 种子；**`files.SaveProcessed`**；**imagetask** 持久化 **`result_file_id`/`result_url`/output**；admin **`/settings/image`**、**`/ai/image-tasks`**、商品详情 **Provider 可选 removebg**；**PROGRESS** §1/§3/§6/§7/§8 同步 |
| 2026-05-16 | **云存储 S3-compatible**：后端 **`internal/providers/storage/s3store`**（**AWS SDK v2**）、**factory**（`local`/`s3`/`r2`/`minio`，**COS/OSS 当时未接独立 SDK**，见本条记录之后续「COS/OSS」行）、**`files/upload|delete`** 与 **`test-storage` HeadBucket**；删除按 **`storage_kind`**；admin **存储设置 `s3_*`**；**`.env.example` / README** 存储说明；**go.mod** 引入 AWS SDK；**PROGRESS** 全篇对齐 |
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