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
| `GET` | `/api/v1/image/tasks/:id/items` | 任务子项列表（源图→结果图、评分 JSON）。 |
| `POST` | `/api/v1/image/tasks/:id/apply` | 将成功任务结果写入 `product_images`（不覆盖原图）。 |
| `GET` | `/api/v1/image/tasks/monitor` | 队列与任务监控快照。 |
| `POST` | `/api/v1/ai/image/tasks` | 创建 AI 图片任务（与 `/image/tasks` 等价）。 |
| `GET` | `/api/v1/ai/image/tasks` | AI 图片任务列表。 |
| `GET` | `/api/v1/ai/image/tasks/:id` | AI 图片任务详情。 |
| `POST` | `/api/v1/ai/image/task-items/:id/save-to-product` | 将任务子项结果保存为新商品图（`applyMode`: main/detail/marketing/ai_generated）。 |
| `POST` | `/api/v1/ai/image/task-items/:id/set-as-main` | 将任务子项结果设为主图（`is_best_main`）。 |
| `POST` | `/api/v1/ai/image/score` | 同步商品图评分（返回 overall/clarity/cleanliness 等维度）。 |

`translate_image_text`（图片文字翻译）读取「设置 → 图片 AI 设置」里的 OCR 配置：`ai_vision` 使用当前 AI 设置中的视觉模型；`paddleocr` 使用本地 PaddleOCR 服务；`aliyun` 会真实调用阿里云 OCR；`tencent` 会真实调用腾讯云 OCR，支持 `GeneralBasicOCR` 与 `GeneralFastOCR`。该任务采用严格 OCR 模式：配置哪个 OCR Provider 就必须实际调用哪个 Provider；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败，不会自动改用其他 OCR。详情输出会包含 `ocr.provider`、`ocr.apiName`、`ocr.configuredOcrProvider`、`ocr.actualOcrProvider`、`ocr.textBlocksCount`、`ocr.averageConfidence`、`ocr.filteredBlocksCount`、`ocr.errorMessage`、`ocr.blocks`、`ocr.groups`、`layout.layoutTemplate` 与 `renderQuality`。每个 OCR block 会补充 `blockClass`、`standardTranslation` 与 `compactTranslation`；顶层会补充 `blockClassifications`、`eraseBBoxCount`、`layoutBBoxCount`、`badgeCount`、`abnormalBadgeCount`、`backgroundPatchScore`、`overlapScore` 与 `finalQualityStatus`。`layout` 还包含 `eraseMode`、`eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies`、`simulation` 等渲染诊断；顶层同步输出 `configuredOcrProvider`、`actualOcrProvider`、`ocrBlocksCount`、`ocrAverageConfidence`、`detected_source_blocks`、`translated_blocks`、`rendered_blocks`、`target_language_present`、`source_language_residue`、`overflow_blocks`、`style_mismatch_count`、`patch_area_ratio`、`render_quality_score`、`overall_confidence` 便于任务详情和批量排查。`renderQuality` 包含 `textAppliedScore`、`sourceTextRemovedScore`、`layoutScore`、`styleConsistencyScore`、`readabilityScore`、`productPreservationScore`、`commercialUsabilityScore`、`passed` 与 `warnings`；当出现异常 badge、文字重叠、背景补丁、原文残留、版面失衡或商用评分不达标时，任务会以 `low_quality` 返回，不应推荐保存到商品图片或设为主图/详情图。

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
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`settings` 分组 **`pricing`**：默认加价方式/比例、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

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

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
