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
| `GET` | `/api/v1/products` | 商品草稿列表。 |
| `POST` | `/api/v1/products` | 创建商品草稿。 |
| `GET` | `/api/v1/products/:id` | 商品详情。 |
| `PUT` | `/api/v1/products/:id` | 更新商品草稿。 |
| `DELETE` | `/api/v1/products/:id` | 删除或归档商品。 |
| `POST` | `/api/v1/products/:id/apply-ai-title` | 应用 AI 标题。 |
| `POST` | `/api/v1/products/:id/apply-ai-description` | 应用 AI 描述。 |
| `POST` | `/api/v1/products/:id/images/select-best-main` | 自动评分并选择最佳主图；JSON `mode`: `score_only` / `recommend` / `auto_set`。 |
| `POST` | `/api/v1/products/:id/sync-images` | 将商品外链图片（如淘宝 alicdn）下载并保存到当前 Storage Provider；JSON `scope`: `all` / `main` / `detail`（默认 `all`）。 |
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`GET /api/v1/products/:id` 商品详情会返回统一商品草稿视图：基础字段 `source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；图片字段 `mainImages`、`descriptionImages`；结构字段 `attributes`、`skuGroups`、`skus`；价格 / 库存聚合字段 `costPrice`、`salePrice`、`stock`；采集与发布字段 `collectWarnings`、`publishStatus`；高级调试字段 `raw` / `rawData`。前端普通视图只展示标准字段与 warning，`raw` 仅用于高级详情。

`pricing.rule` 支持：`costSource`（`collected` / `manual`）、`manualCostPrice`、`markupType`（`fixed` / `percent` / `multiplier` / `none`）、`markupAmount`、`markupPercent`、`markupMultiplier`、`shippingCost`、`weight`、`shippingCostPerWeight`、`platformCommissionPercent`、`exchangeRate`、`minProfit`、`minMarginPercent`、`minPublishPrice`、`roundingMode`（`none` / `integer` / `.9` / `.95` / `.99` / `9.99` / `19.90`）。试算返回 `landedCost`、`commissionFee`、`estimatedProfit`、`profitMarginPercent`；应用后写入 `product_skus.price` 并写操作日志。

`settings` 分组 **`pricing`**：默认加价方式/比例/倍率、固定运费、按重量运费单价（预留）、平台佣金、最低利润、最低利润率、汇率、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

发布前检查 `GET /api/v1/products/:id/readiness` 返回兼容字段 `status=ready|warning|blocked`，并新增 `result=passed|warning|failed`。`failed` 阻止创建刊登任务；`warning` 可继续但前端必须人工确认。当前检查项包括标题、AI 标题建议、描述、主图、SKU、价格、售价低于成本、最低利润 / 利润率保护、库存、外链图片同步提示、平台必填字段、采集 warning。`platform=douyin_shop` 时还会读取已保存的抖店刊登草稿映射，校验抖店标题、主图、类目、必填属性、SKU、价格、库存、图片待同步和采集来源复核提示。

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

Provider 调用官方 `sku.syncStock`（`incremental=false` 全量更新）；受 `inventory_sync_enabled` 开关控制（默认关闭）。缺失平台 SKU ID 返回 `DOUYIN_SKU_NOT_BOUND`，不猜测同步。

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

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
