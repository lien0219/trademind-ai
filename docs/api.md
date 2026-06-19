# API 契约

本文件记录 TradeMind 后端 API 的公共约定。新增、删除或修改接口时，必须同步检查后端 handler / service / DTO、前端 services / types / 页面，以及本文档。

## 基础约定

- 基础路径：`/api/v1`
- 健康检查：`GET /health`、`GET /api/v1/health`
- 鉴权：管理端受保护接口使用 `Authorization: Bearer <token>`
- 返回格式：统一 JSON 响应，核心字段为 `code`、`message`、`data`、`traceId`
- 敏感信息：接口不得返回完整 API Key、Token、Secret、Cookie 或密码

示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "traceId": "request-id"
}
```

## 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | 管理员登录，支持邮箱或手机号。 |
| `POST` | `/api/v1/auth/logout` | 退出登录，客户端丢弃 token。 |
| `GET` | `/api/v1/auth/profile` | 当前管理员信息。 |

## 设置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 读取系统设置。 |
| `PUT` | `/api/v1/settings` | 保存系统设置，敏感字段必须加密。 |
| `POST` | `/api/v1/settings/test-ai` | 经 **AI Gateway** 测试 `settings.ai`（支持 `openai` / `openai_compatible` / `deepseek` / `qwen`）。各服务商 **`{provider}_api_key` / `{provider}_base_url` / `{provider}_model`** 独立存储；可选 JSON：`provider`、`base_url`、`model`、`api_key`（写入当前 provider 对应项；`****` 占位则沿用已保存密钥）、`timeout_sec`，用于**未保存前**用当前表单试连；空 body 仅用库内配置。成功 `data`：`ok`、`message`、`provider`、`model`、`latencyMs`。 |
| `POST` | `/api/v1/settings/test-storage` | 测试 Storage Provider 配置。 |
| `POST` | `/api/v1/storage/test-public-access` | 上传探针图片并通过匿名 HTTP 验证公网可访问性（HTTPS、`image/*`、无登录跳转）；失败返回 `STORAGE_PUBLIC_*` 错误码。 |
| `POST` | `/api/v1/settings/test-image` | 测试 `settings.image` 图片 Provider 配置。可选 JSON：`provider`、`testMode`（`config_only` \| `live`，默认 `config_only`）、`settings`（表单覆盖项，支持未保存先测；脱敏 `****` 占位符会忽略并沿用已保存密钥）。成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`supportedTasks`、`configStatus`。不返回 API Key。 |
| `POST` | `/api/v1/settings/test-ocr` | 测试 `settings.image` 中的 OCR 配置。可选 JSON：`provider`（`ai_vision` / `paddleocr` / `baidu` / `aliyun` / `tencent`）、`settings`（表单覆盖项，支持未保存先测；脱敏密钥占位符会忽略）。`paddleocr` 会用后端生成的测试图调用 OCR 服务，检查连通性、文字 `blocks` 与 `bbox`；成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`blocks`、`bboxOk`。 |

## 图片 AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/image/providers` | 图片 Provider 能力矩阵（`status` / `supportedTasks` / 难度等，不含密钥）。 |
| `POST` | `/api/v1/image/tasks` | 创建图片任务；创建时校验 Provider 与 `taskType` 组合。 |
| `GET` | `/api/v1/image/tasks` | 图片任务列表。 |
| `GET` | `/api/v1/image/tasks/:id` | 图片任务详情。 |
| `POST` | `/api/v1/image/tasks/:id/retry` | 重试失败任务。 |
| `GET` | `/api/v1/image/tasks/:id/translate-edit-state` | 图片文字翻译人工编辑态：返回原图、已擦除底图、结果图、图片尺寸与可编辑文字块（译文、排版框、擦除框、样式）。 |
| `POST` | `/api/v1/image/tasks/:id/manual-render` | 图片文字翻译人工兜底渲染：按人工编辑后的文字块重新擦除原文并规则重绘译文，结果上传 Storage Provider 并回写任务为 `success_with_review`。 |
| `GET` | `/api/v1/image/tasks/:id/items` | 任务子项列表（源图→结果图、评分 JSON）。 |
| `POST` | `/api/v1/image/tasks/:id/apply` | 将成功任务结果写入 `product_images`（不覆盖原图）。 |
| `GET` | `/api/v1/image/tasks/monitor` | 队列与任务监控快照。 |
| `POST` | `/api/v1/ai/image/tasks` | 创建 AI 图片任务（与 `/image/tasks` 等价）。 |
| `GET` | `/api/v1/ai/image/tasks` | AI 图片任务列表。 |
| `GET` | `/api/v1/ai/image/tasks/:id` | AI 图片任务详情。 |
| `GET` | `/api/v1/ai/image/tasks/:id/translate-edit-state` | 与 `/image/tasks/:id/translate-edit-state` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/tasks/:id/manual-render` | 与 `/image/tasks/:id/manual-render` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/task-items/:id/save-to-product` | 将任务子项结果保存为新商品图（`applyMode`: main/detail/marketing/ai_generated）。 |
| `POST` | `/api/v1/ai/image/task-items/:id/set-as-main` | 将任务子项结果设为主图（`is_best_main`）。 |
| `POST` | `/api/v1/ai/image/score` | 同步商品图评分（返回 overall/clarity/cleanliness 等维度）。 |

`translate_image_text`（图片文字翻译）读取「设置 → 图片 AI 设置」里的 OCR 配置：`ai_vision` 使用当前 AI 设置中的视觉模型；`paddleocr` 使用本地 PaddleOCR 服务；`aliyun` 会真实调用阿里云 OCR；`tencent` 会真实调用腾讯云 OCR，支持 `GeneralBasicOCR` 与 `GeneralFastOCR`。该任务采用严格 OCR 模式：配置哪个 OCR Provider 就必须实际调用哪个 Provider；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败，不会自动改用其他 OCR。详情输出会包含 `ocr.provider`、`ocr.apiName`、`ocr.configuredOcrProvider`、`ocr.actualOcrProvider`、`ocr.textBlocksCount`、`ocr.averageConfidence`、`ocr.filteredBlocksCount`、`ocr.errorMessage`、`ocr.blocks`、`ocr.groups`、`layout.layoutTemplate` 与 `renderQuality`。每个 OCR block 会补充 `blockClass`、`standardTranslation` 与 `compactTranslation`；顶层会补充 `blockClassifications`、`eraseBBoxCount`、`layoutBBoxCount`、`badgeCount`、`abnormalBadgeCount`、`backgroundPatchScore`、`overlapScore` 与 `finalQualityStatus` 分级：`success`（商用分≥85）、`success_with_review`（75–84，可下载，保存到商品前建议人工检查）、`failed_render_validation`（<65 或中文残留/溢出/遮挡商品主体等硬失败）。调试输出：`debugOriginalUrl`、`debugMaskUrl`、`debugErasedUrl`、`debugFinalUrl`（对应 original/mask/erased/final.png）。65–74 分同任务内自动质量重试一次（`qualityAutoRetried`）。人工兜底使用 `translate-edit-state` 读取可编辑块，再用 `manual-render` 基于原图/已擦除图重新擦除原文并规则重绘译文；输出会记录 `manualEdit`（baseImage、blocks、editedAt、editedBy、eraseMode 等），任务回写为 `success_with_review`。`layout` 还包含 `eraseMode`、`eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies`、`simulation` 等渲染诊断；顶层同步输出 `configuredOcrProvider`、`actualOcrProvider`、`ocrBlocksCount`、`ocrAverageConfidence`、`detected_source_blocks`、`translated_blocks`、`rendered_blocks`、`target_language_present`、`source_language_residue`、`overflow_blocks`、`style_mismatch_count`、`patch_area_ratio`、`render_quality_score`、`overall_confidence` 便于任务详情和批量排查。`renderQuality` 包含 `textAppliedScore`、`sourceTextRemovedScore`、`layoutScore`、`styleConsistencyScore`、`readabilityScore`、`productPreservationScore`、`commercialUsabilityScore`、`passed` 与 `warnings`；当出现异常 badge、文字重叠、背景补丁、原文残留、版面失衡或商用评分不达标时，任务会以 `low_quality` 返回，不应推荐保存到商品图片或设为主图/详情图。

## 文件

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/files/upload` | 上传文件。 |
| `GET` | `/api/v1/files` | 文件列表。 |
| `DELETE` | `/api/v1/files/:id` | 删除文件。 |

## 商品

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products` | 商品草稿列表；支持 `operationStep`（`collect_review` / `title` / `description` / `images` / `pricing` / `publish_check` / `ready`）筛选，并在列表行返回轻量 `operationProgress` 摘要。 |
| `POST` | `/api/v1/products` | 创建商品草稿。 |
| `GET` | `/api/v1/products/:id` | 商品详情。 |
| `GET` | `/api/v1/products/:id/operation-progress` | 商品运营进度摘要；只读聚合商品、图片、SKU 与既有发布前检查，不调用平台 API、不创建任务、不修改商品。 |
| `PUT` | `/api/v1/products/:id` | 更新商品草稿。 |
| `DELETE` | `/api/v1/products/:id` | 删除或归档商品。 |
| `POST` | `/api/v1/products/:id/apply-ai-title` | 应用 AI 标题；body 支持 `aiTitle`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`，不会静默覆盖人工修改。 |
| `POST` | `/api/v1/products/:id/undo-ai-title` | 安全撤销最近一次 AI 标题应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/apply-ai-description` | 应用 AI 描述；body 支持 `aiDescription`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/undo-ai-description` | 安全撤销最近一次 AI 描述应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/images/select-best-main` | 自动评分并选择最佳主图；JSON `mode`: `score_only` / `recommend` / `auto_set`。 |
| `POST` | `/api/v1/products/:id/sync-images` | 将商品外链图片（如淘宝 alicdn）下载并保存到当前 Storage Provider；JSON `scope`: `all` / `main` / `detail`（默认 `all`）。 |
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`GET /api/v1/products/:id` 商品详情会返回统一商品草稿视图：基础字段 `source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；图片字段 `mainImages`、`descriptionImages`；结构字段 `attributes`、`skuGroups`、`skus`；价格 / 库存聚合字段 `costPrice`、`salePrice`、`stock`；采集与发布字段 `collectWarnings`、`publishStatus`；高级调试字段 `raw` / `rawData`。前端普通视图只展示标准字段与 warning，`raw` 仅用于高级详情。

`operationProgress` 统一使用实际数据实时计算：采集结果、标题、描述、图片、价格、通用参数、发布检查、刊登草稿准备。返回字段包括 `completionPercent`、`currentStep`、`currentStepLabel`、`nextActionLabel`、`nextActionKey`、`nextActionUrl`、`completedSteps`、`pendingSteps`、`blockers`、`warnings`、`publishReady`、`updatedAt`。列表摘要只返回完成度、当前步骤、下一步入口、阻断/建议数量和可刊登状态；列表聚合批量读取图片、SKU 与图片任务状态，禁止逐行调用平台或自动创建任务。

`pricing.rule` 支持：`costSource`（`collected` / `manual`）、`manualCostPrice`、`markupType`（`fixed` / `percent` / `multiplier` / `none`）、`markupAmount`、`markupPercent`、`markupMultiplier`、`shippingCost`、`weight`、`shippingCostPerWeight`、`platformCommissionPercent`、`exchangeRate`、`minProfit`、`minMarginPercent`、`minPublishPrice`、`roundingMode`（`none` / `integer` / `.9` / `.95` / `.99` / `9.99` / `19.90`）。试算返回 `landedCost`、`commissionFee`、`estimatedProfit`、`profitMarginPercent`；应用后写入 `product_skus.price` 并写操作日志。

`settings` 分组 **`pricing`**：默认加价方式/比例/倍率、固定运费、按重量运费单价（预留）、平台佣金、最低利润、最低利润率、汇率、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

发布前检查 `GET /api/v1/products/:id/readiness` 返回兼容字段 `status=ready|warning|blocked`，并新增 `result=passed|warning|failed`，以及用户可见 `statusLabel` / `resultLabel`。每个 `checks[]` 项含 `title`、`message`、`severity`（同 `level`）与 `technicalDetails.rawCode`（内部码，前端默认折叠）。`failed` 阻止创建刊登任务；`warning` 可继续但前端必须人工确认。采集 warning 码（如 `DETAIL_IMAGES_INCOMPLETE`）在后端统一中文化。

**多平台刊登中心（Phase A1.2）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publish-targets` | 可刊登平台、店铺与能力分级（`real_draft_create` / `local_draft_only` / …） |
| `POST` | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查；body 含 `targets[]`、`commonConfig`、`targetConfigs` |
| `POST` | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建刊登草稿；形成 `product_publish_batches` + 子任务；支持 `onlyReady`、`retryFailedOnly` + `batchId` |

详见 [`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。

刊登任务 `POST /api/v1/products/:id/publish` 会保存 `product_publish_tasks`，任务字段包括 `productId`、`targetPlatform`、`targetStoreId`、`status`（队列态，兼容旧值）、`publishStatus`（业务态：`draft` / `checking` / `ready` / `publishing` / `success` / `failed` / `cancelled`）、`publishMode`、`title`、`description`、`images`、`skus`、`price`、`currency`、`checkResult`、`platformPayload`、`platformResult`、`errorCode`、`errorMessage`、`createdAt`、`updatedAt`。平台字段映射快照包含 `platformTitle`、`platformDescription`、`platformImages`、`platformSkus`、`platformPrice`、`platformStock`、`platformCategory`、`platformAttributes`。

## AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/ai/title-optimize` | AI 标题优化。 |
| `POST` | `/api/v1/ai/description-generate` | AI 描述生成。 |
| `POST` | `/api/v1/ai/chat` | AI 对话或客服建议。 |
| `GET` | `/api/v1/ai/tasks` | AI 任务列表。 |
| `GET` | `/api/v1/ai/tasks/:id` | AI 任务详情。 |

## 采集

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/collect/tasks` | 创建采集任务。`source=custom` 时若 URL 属于已有 **available/beta** 专用采集器域名，返回业务码 **40002**，`data.errorCode=CUSTOM_COLLECT_PROVIDER_CONFLICT`，含 `recommendedProvider` 与 `message`。 |
| `GET` | `/api/v1/collect/tasks` | 采集任务列表。 |
| `GET` | `/api/v1/collect/tasks/:id` | 采集任务详情。 |
| `POST` | `/api/v1/collect/tasks/:id/retry` | 重试采集任务。 |
| `POST` | `/api/v1/collect/rules/ai-generate` | AI 根据商品 URL 生成自定义采集规则（分析页面摘要 → AI → 校验 → 自动规则测试）。1688 / AliExpress 等 **available/beta** 专用平台返回 **40002**。规则非法返回 **40003** `AI_RULE_INVALID`。 |
| `POST` | `/api/v1/collect/rules/ai-generate-and-save` | 同上并直接保存为 `collect_rule`。 |
| `GET` | `/api/collector/providers/1688/auth-status` | 1688 采集浏览器登录态检测（同 `/api/v1/collector/...`）。 |
| `POST` | `/api/collector/providers/1688/open-login-browser` | 打开持久化 Playwright 浏览器供 1688 手动登录。 |
| `GET` | `/api/collector/providers/pinduoduo/auth-status` | 拼多多登录态检测（兼容 GET；内部走 check-login 逻辑）。 |
| `POST` | `/api/v1/collect/providers/pinduoduo/check-login` | 拼多多登录态检测（推荐）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；检测优先级：body.url → 最近失败任务 URL → 设置 `collect_pinduoduo_auth_check_url` → 仅 pifa 首页（`homepage_only`）。 |
| `POST` | `/api/collector/providers/pinduoduo/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/pinduoduo/open-login-browser` | 打开拼多多采集浏览器手动登录；body 可选 `{ "url": "商品或 pifa 链接" }`（勿传无参 `mobile.yangkeduo.com` 首页）。 |
| `POST` | `/api/v1/collect/providers/taobao_tmall/check-login` | 淘宝/天猫登录态检测（批量采集开始前也会调用）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；未登录返回业务错误文案；需安全验证时阻止批量开始。 |
| `POST` | `/api/collector/providers/taobao_tmall/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/taobao_tmall/open-login-browser` | 打开淘宝/天猫采集浏览器手动登录；body 可选 `{ "url": "商品链接" }`。 |

`GET /api/collector/providers/1688/auth-status` 返回示例：

```json
{
  "provider": "1688",
  "status": "ok",
  "loggedIn": true,
  "needVerification": false,
  "message": "1688 登录态正常",
  "lastCheckedAt": "2026-05-20T12:00:00.000Z",
  "profilePath": "/path/to/collector/data/browser-profiles/1688"
}
```

`status` 取值：`ok`（已登录）、`not_logged_in`（需要登录）、`wechat_auth_required`（微信扫码）、`app_redirect`（App 引导页）、`verification_required`（需验证）、`homepage_only`（仅首页可访问，无法确认登录）、`unknown`（暂时无法确认）。

拼多多 `check-login` 返回扩展字段（无 Cookie/HTML）：`profileKey`（`pinduoduo`）、`checkedUrl`、`finalUrl`、`accessStatus`、`urlType`（`wholesale_detail` | `goods_detail` | `homepage` | `app_redirect` | `unknown`）、`checkMode`、`evidence`（`hasProductTitle` / `hasPrice` / `hasMainImage` 等）。**仅当打开商品详情页且识别到标题/价格/主图之一，且无登录/微信/App 引导时** 才返回 `ok`；**pifa 首页可访问不判已登录**。

`POST open-login-browser` 与 `check-login` 使用同一 **`pinduoduo` Profile**（与 1688、custom 隔离）。采集浏览器登录窗口 **1280×900**。

## 店铺与平台

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/stores` | 店铺列表。 |
| `POST` | `/api/v1/stores/:platform/auth-url` | 生成平台授权地址。 |
| `GET` | `/api/v1/stores/:platform/callback` | 平台 OAuth 回调。 |
| `POST` | `/api/v1/stores/:id/refresh-token` | 刷新店铺授权 Token。 |

现行平台 Provider 与开放平台应用配置接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/providers` | 返回已注册平台 Provider、能力、状态、`appConfigSchema` 与设置分组。`douyin_shop` 已注册为抖店 / Douyin Shop Provider。 |
| `GET` | `/api/v1/platform/settings/:platform` | 读取平台开放应用配置 schema 与脱敏后的当前值。敏感字段只返回 `****`。 |
| `PUT` | `/api/v1/platform/settings/:platform` | 保存平台开放应用配置。敏感字段加密存储，传入 `****` 表示保留原值。`douyin_shop` 会校验 App Key、App Secret、回调地址、环境与超时时间；发起 OAuth 还需要 `service_id`。 |
| `POST` | `/api/v1/platform/settings/:platform/test-connection` | 测试已保存的平台开放应用配置。`douyin_shop` 应用配置测试校验配置完整性与授权可用性，不做商品 / 订单 / 库存调用。 |
| `GET` | `/api/v1/shops/oauth/douyin/start` | 发起抖店 OAuth；生成 Redis state（10 分钟，绑定管理员、`platform=douyin_shop`、可选 `shopId`），返回 `redirectUrl`。 |
| `GET` | `/api/v1/shops/oauth/douyin/callback` | 抖店授权公开回调；校验 state，处理 `code` / `error`，换取 token，创建或更新 `shops` / `shop_auth_tokens`，成功跳转 `/settings/platforms?platform=douyin_shop&auth=success`。 |
| `GET` | `/api/v1/shops/:id/oauth/douyin/authorize-url` | 已有抖店店铺重新授权，返回 `redirectUrl`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 使用加密保存的 refresh token 刷新抖店 access token，并用刷新响应校准店铺基础信息；失败时按场景标记 `expired` / `invalid`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/revoke` | 本地解除抖店授权，清理 / 失效 token，保留历史数据。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/test` | 真实测试抖店店铺连接：检查授权、必要时刷新 token、读取并校准店铺基础信息；不返回 token 明文。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/sync-shop-info` | 手动同步 / 校准抖店店铺基础信息，复用 Phase 3 OpenAPI Client 与 token 自动刷新能力。 |
| `GET` | `/api/v1/platform/douyin/categories` | 读取本地缓存的抖店类目树；支持 `keyword`、`parentId`、`onlyLeaf`、`refresh=false`、`shopId`（仅 `refresh=true` 时用于手动刷新）。 |
| `POST` | `/api/v1/platform/douyin/categories/sync` | 使用已授权抖店店铺 token 同步类目缓存，body/query 传 `shopId`；写入 `platform_categories`，幂等 upsert。 |
| `GET` | `/api/v1/platform/douyin/categories/stats` | 返回抖店类目缓存数量、叶子类目数量和最近同步时间，供平台开放配置页展示。 |
| `GET` | `/api/v1/platform/douyin/categories/:categoryId/attributes` | 读取某个抖店类目的本地属性缓存；返回必填、可选项、属性值选项和同步时间，不返回 raw。 |
| `POST` | `/api/v1/platform/douyin/categories/:categoryId/attributes/sync` | 使用已授权抖店店铺 token 刷新某个叶子类目的属性缓存，body/query 传 `shopId`；写入 `platform_category_attributes`，幂等 upsert。 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 抖店上线前生产预检（配置、授权、开关、Storage 公网、数据状态）；body 可选 `{ "liveTest": true }` 对首家已授权店铺做 Token 刷新联调。 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 读取最近一次预检结果（存于 settings `douyin_preflight.latest_result`）。 |
| `GET` | `/api/v1/platform/douyin/runtime-status` | 读取抖店运行状态（`normal` / `paused` / `emergency_disabled`）、原因与变更时间。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/pause` | 暂停抖店任务；body `{ "reason": "..." }` 必填；记录 `douyin.platform.pause` 操作日志。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/resume` | 恢复抖店运行；body `{ "reason": "..." }` 必填。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/emergency-disable` | 紧急停用；阻止 Worker 调用抖店写接口；body `{ "reason": "..." }` 必填。 |
| `GET` | `/api/v1/products/:id/platform-configs/:platform` | 读取商品的平台刊登准备配置；`douyin_shop` 返回 `shopId`、`categoryId`、`categoryPath`、`platformAttributes`，以及已保存的 `mapping` / `lastMappedAt`。 |
| `PUT` | `/api/v1/products/:id/platform-configs/:platform` | 保存商品的平台刊登准备配置；`douyin_shop` 会校验类目必须为本地缓存中的叶子类目，并记录抖店类目/属性操作日志。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/build-mapping` | 根据当前商品草稿、抖店店铺/类目/属性配置生成并保存抖店刊登草稿预览；不调用抖店创建商品或图片上传接口。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 读取已保存的抖店刊登草稿映射。 |
| `PUT` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 保存人工调整后的抖店刊登草稿映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/validate` | 校验抖店刊登草稿映射；可传入临时映射 body，也可不传 body 校验已保存映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/upload` | 上传当前抖店刊登草稿中的待上传图片到抖店素材中心。body：`imageTypes`（`main` / `detail`）、`retryFailed`、`force`。外链会先下载并写入当前 Storage Provider，再通过后端 Douyin Client 上传；不创建抖店商品。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/:imageKey/retry` | 重试单张抖店图片上传。`imageKey` 可用 `localImageId`、`main:0` / `detail:0`、`storageKey` 或已有 `platformImageId`。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/images/status` | 读取当前抖店图片上传状态、Storage 状态、平台图片 ID / URL、失败原因和统计。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/create-draft` | 根据已保存抖店映射与已上传素材图创建抖店平台商品草稿。body：`shopId`（必填）、`publishMode`（默认 `save_as_platform_draft`）、`force`（已有 platformProductId 时二次确认）。会先执行发布前检查；`failed` 阻止创建。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/publish-tasks` | 列出当前商品的抖店刊登任务（分页）。 |
| `POST` | `/api/v1/product-publish/tasks/:id/cancel` | 取消 pending/running 刊登任务。 |

抖店 SKU 绑定校准与手动兜底（Phase 9.1 / 9.2，`product_publications.id` 或 `product_publication_skus.id` 为路径参数）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publications/:id/douyin/sku-bindings` | 读取当前 `product_publication_skus` 绑定状态汇总（`bound` / `skipped` / `unmatched` / `ambiguous` / `failed` 计数与行明细）；含 `platformSkus` 平台候选、`inventorySyncReady` / `inventorySyncBlockReason`。 |
| `POST` | `/api/v1/product-publications/:id/douyin/sync-sku-bindings` | 调用官方 `product.detail`（`show_draft=true`）拉取抖店 SKU 列表并校准本地映射，回写 `external_sku_id`、`bindStatus`、`bindConfidence`、`bindMessage`、`lastSyncedAt`；更新 `product_publications.skuBindingSyncedAt` 与 `raw_data.platformSkus` 缓存。已绑定 SKU 跳过；多候选标记 `ambiguous` 不强行绑定。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/bind-sku` | 人工绑定抖店 SKU。body：`platformSkuId`（必填）、`platformSkuName`、`bindReason`（如 `manual`）。校验 publication 归属 `douyin_shop`、平台商品 ID 存在、SKU ID 非空、不与其他本地规格冲突；覆盖旧绑定时记录操作日志。成功后 `bindStatus=bound`、`bindConfidence=100`、`bindMessage=手动绑定`。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/unbind-sku` | 解除绑定。body：`reason`（如 `manual_unbind`）。清空 `external_sku_id`，`bindStatus=unmatched`、`bindMessage=已手动解除绑定`。 |

错误码：`DOUYIN_PRODUCT_DETAIL_FAILED`、`DOUYIN_PRODUCT_NOT_FOUND`、`DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED`、`DOUYIN_SKU_BINDING_SYNC_FAILED`、`DOUYIN_SKU_BINDING_UNMATCHED`、`DOUYIN_SKU_BINDING_AMBIGUOUS`、`DOUYIN_SKU_MANUAL_BIND_FAILED`、`DOUYIN_SKU_MANUAL_UNBIND_FAILED`、`DOUYIN_PLATFORM_SKU_ID_MISSING`、`DOUYIN_SKU_BINDING_CONFLICT`、`DOUYIN_SKU_BINDING_REQUIRED`。

操作日志：`douyin.sku.binding.manual_bind`、`douyin.sku.binding.manual_unbind`、`douyin.sku.binding.recheck`、`douyin.sku.binding.conflict`（不记录 token / secret）。

抖店库存同步（Phase 9，复用既有 inventory 模块，无新增割裂路径）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publication-skus` | 商品详情库存 Tab 读取刊登 SKU 映射与 `inventorySyncCapability`（`douyin_shop` 为 `beta`）。 |
| `POST` | `/api/v1/product-publication-skus/:id/sync-inventory` | 单 SKU 库存同步；body：`stock`、`options`、`fromInventoryAlert`。要求 `product_publications.external_product_id` 与 `product_publication_skus.external_sku_id` 已绑定。 |
| `POST` | `/api/v1/products/:id/sync-inventory` | 单商品多 SKU 库存同步；body：`shopId`、`skuIds[]`、`options`。 |
| `GET` | `/api/v1/inventory-sync/tasks` | 库存同步任务列表。 |
| `GET` | `/api/v1/inventory-sync/tasks/:id` | 任务详情。 |
| `POST` | `/api/v1/inventory-sync/tasks/:id/retry` | 重试 failed 任务。 |
| `POST` | `/api/v1/inventory-sync/batches` | 批量库存同步（默认低并发）。 |

Provider 调用官方 `sku.syncStock`（`incremental=false` 全量更新）；受 `inventory_sync_enabled` 开关控制（默认关闭）。缺失平台 SKU ID 或 `bindStatus=unmatched/failed` 返回 `DOUYIN_SKU_BINDING_REQUIRED`；`bindStatus=ambiguous` 返回 `DOUYIN_SKU_BINDING_AMBIGUOUS`；绑定冲突返回 `DOUYIN_SKU_BINDING_CONFLICT`；不猜测同步。库存同步前须全部 SKU 处于可同步绑定状态（bound / skipped 且已有 `external_sku_id`）。

通用刊登任务接口（含抖店）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publish/tasks` | 刊登任务列表 |
| `GET` | `/api/v1/product-publish/tasks/:id` | 任务详情（含 `platformPayload` 平台提交内容、`platformProductId` 抖店商品 ID、`retryable` 是否可重试） |
| `POST` | `/api/v1/product-publish/tasks/:id/retry` | 重试 failed 任务 |

`product_platform_publish_configs.mapped_images` 在抖店 Phase 6 保存扩展结构：

```json
{
  "mainImages": [
    {
      "localImageId": "",
      "sourceUrl": "",
      "storageUrl": "",
      "storageKey": "",
      "platformImageId": "",
      "platformImageUrl": "",
      "imageType": "main",
      "uploadStatus": "pending|processing|uploaded|failed|skipped",
      "errorCode": "",
      "errorMessage": "",
      "uploadedAt": "",
      "processed": false
    }
  ],
  "detailImages": []
}
```

抖店 OAuth / Client / 类目 / 映射 / 图片错误码：`DOUYIN_APP_CONFIG_INCOMPLETE`、`DOUYIN_OAUTH_STATE_INVALID`、`DOUYIN_OAUTH_DENIED`、`DOUYIN_OAUTH_CODE_MISSING`、`DOUYIN_TOKEN_EXCHANGE_FAILED`、`DOUYIN_TOKEN_REFRESH_FAILED`、`DOUYIN_SHOP_INFO_FAILED`、`DOUYIN_AUTH_EXPIRED`、`DOUYIN_PERMISSION_DENIED`、`UNKNOWN_DOUYIN_AUTH_ERROR`、`DOUYIN_API_ERROR`、`DOUYIN_RATE_LIMITED`、`DOUYIN_REQUEST_TIMEOUT`、`DOUYIN_RESPONSE_PARSE_FAILED`、`UNKNOWN_DOUYIN_ERROR`、`DOUYIN_CATEGORY_SYNC_FAILED`、`DOUYIN_CATEGORY_EMPTY`、`DOUYIN_CATEGORY_NOT_SELECTED`、`DOUYIN_CATEGORY_NOT_LEAF`、`DOUYIN_CATEGORY_ATTR_SYNC_FAILED`、`DOUYIN_REQUIRED_ATTR_MISSING`、`DOUYIN_CATEGORY_CACHE_STALE`、`DOUYIN_CATEGORY_PERMISSION_DENIED`、`DOUYIN_TITLE_MISSING`、`DOUYIN_TITLE_TOO_LONG`、`DOUYIN_DESCRIPTION_MISSING`、`DOUYIN_DESCRIPTION_NEEDS_REVIEW`、`DOUYIN_MAIN_IMAGE_MISSING`、`DOUYIN_MAIN_IMAGE_NOT_UPLOADED`、`DOUYIN_MAIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_DETAIL_IMAGE_UPLOAD_PARTIAL_FAILED`、`DOUYIN_IMAGE_NEED_UPLOAD`、`DOUYIN_IMAGE_UPLOAD_EXPIRED`、`DOUYIN_IMAGE_NEED_SYNC`、`DOUYIN_DETAIL_IMAGE_EMPTY`、`DOUYIN_DETAIL_IMAGE_NEED_SYNC`、`DOUYIN_ATTR_VALUE_INVALID`、`DOUYIN_SKU_MISSING`、`DOUYIN_SKU_PRICE_INVALID`、`DOUYIN_SKU_STOCK_UNCONFIRMED`、`DOUYIN_SKU_ATTR_INCOMPLETE`、`DOUYIN_PRICE_MISSING`、`DOUYIN_PRICE_INVALID`、`DOUYIN_PROFIT_TOO_LOW`、`DOUYIN_STOCK_UNCONFIRMED`、`DOUYIN_STOCK_INVALID`、`DOUYIN_COLLECT_NEEDS_REVIEW`、`IMAGE_URL_NOT_ACCESSIBLE`、`IMAGE_DOWNLOAD_FAILED`、`IMAGE_READ_FAILED`、`IMAGE_FORMAT_UNSUPPORTED`、`IMAGE_SIZE_TOO_LARGE`、`IMAGE_DIMENSION_INVALID`、`IMAGE_PROCESS_FAILED`、`STORAGE_UPLOAD_FAILED`、`DOUYIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_STORE_NOT_AUTHORIZED`、`DOUYIN_CREATE_PRODUCT_FAILED`、`DOUYIN_PRODUCT_PAYLOAD_INVALID`。API 错误响应 `data.errorCode` 返回业务码；callback 失败通过 `reason` query 返回。所有响应均不得返回 App Secret、access token 或 refresh token 明文。

## 抖店可观测性 / Health & Metrics（Phase 10.4）

> **不** 提供 Prometheus `/metrics`。抖店生产监控复用进程健康、任务中心、操作日志与运营看板。E2E 脚本见 `scripts/douyin-e2e-*`；门禁见 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)。

### 进程健康（含抖店相关队列）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 匿名；`data.status` 为 `up` / `degraded`；含 `checks.database`、`checks.redis` |
| `GET` | `/api/v1/health` | 同上 |

`data` 中与抖店 Worker 相关的块（队列启用时）：

| 字段 | 说明 |
| --- | --- |
| `orderSyncQueue` | 订单同步 Redis 队列深度、Worker 并发、`redisAvailable` |
| `productPublishQueue` | 商品刊登（含抖店草稿创建）队列 |
| `inventorySyncQueue` | 库存同步（含 `sku.syncStock`）队列 |
| `workers` | 各 Worker 心跳；`degraded=true` 时整体 `status=degraded` |

### 抖店运行态、健康与指标（Phase 10.4，无 Prometheus）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/douyin/runtime-status` | `normal` / `paused` / `emergency_disabled`、原因与时间 |
| `GET` | `/api/v1/platform/douyin/health` | 抖店聚合健康：`overallStatus`（`healthy` / `degraded` / `unhealthy` / `disabled`）、`config` / `auth` / `storage` / `tasks` / `api` 分区、`grayRelease`、`runtime`；快照写入 settings `health_snapshot` |
| `GET` | `/api/v1/platform/douyin/metrics-summary` | 滚动 24h 内存指标（API 成功率/耗时、Token 刷新、任务 stale、刊登/订单/库存/SKU 计数等）；**非** Prometheus `/metrics` |
| `GET` | `/api/v1/platform/douyin/release-gate` | Release Candidate 门禁清单：`overallConclusion`（默认 `Release Candidate`）、`items[]`（`key` / `label` / `status` / `message`）；`credentials` 项在无真实 E2E 时为 `blocked` |
| `POST` | `/api/v1/platform/douyin/run-health-check` | 执行健康聚合 + taskcenter 抖店告警 scan；返回与 `GET .../health` 相同结构并持久化快照 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 上线预检；`data.blockedByRealCredentials` 为 true 时表示无真实凭证 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 最近一次预检 JSON |

### 任务中心（失败 / 告警 / 摘要）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/task-center/summary` | 失败任务与告警计数摘要 |
| `GET` | `/api/v1/task-center/failures` | 失败任务列表；可用 `keyword=DOUYIN` 筛选抖店错误 |
| `GET` | `/api/v1/task-center/failures/:taskType/:id` | 失败详情（脱敏 raw） |
| `GET` | `/api/v1/task-center/alerts` | 站内告警列表 |
| `POST` | `/api/v1/task-center/alerts/scan` | 扫描并生成告警（dedupe） |
| `POST` | `/api/v1/task-center/alerts/:id/notify` | Webhook 通知（需配置） |
| `GET` | `/api/v1/task-center/failure-categories` | 含 `sub:douyin_*` 分类 |

### 操作日志与运营看板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/operation-logs` | 查询 `action`（如 `douyin.auth.success`）；不返回 Secret/Token |
| `GET` | `/api/v1/dashboard/product-operations` | 商品运营 KPI、漏斗、异常（只读 DB 聚合，不调抖店 OpenAPI） |

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
