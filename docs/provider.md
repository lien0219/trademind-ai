# Provider 扩展机制

TradeMind 通过 Provider 抽象接入第三方和本地能力，避免业务模块直接依赖具体平台或 SDK。

## Provider 类型

```text
AI Provider
Storage Provider
Image Provider
Platform Provider
Collector Provider
```

## AI Provider

用于接入大模型服务。

当前重点：

- **OpenAI**（`openai`）
- **OpenAI-compatible**（`openai_compatible`）
- **DeepSeek**（`deepseek`，Chat Completions）
- **通义千问 / Qwen**（`qwen`，DashScope OpenAI 兼容模式）
- 共享 **`compatclient`** HTTP 实现，各 Provider 负责默认地址、错误码中文化与后续扩展入口
- Prompt 模板、AI 调用记录、标题优化、描述生成、客服建议回复

后续可扩展：

- DeepSeek / Qwen 专属错误码、多模态、Embedding、Rerank、用量统计
- 多 Provider 配置表（`settings.ai_providers`）
- Doubao、Gemini、Claude、Ollama（亦可经 `openai_compatible` 接入）

## Storage Provider

用于接入文件与对象存储。

当前支持或预留：

- local
- S3
- Cloudflare R2
- MinIO
- Tencent COS
- Aliyun OSS

敏感字段必须加密存储并脱敏展示。

## Image Provider

用于接入图片处理能力。

当前支持或预留：

- noop
- remove.bg
- OpenAI Image
- ComfyUI

图片任务应通过任务状态与队列执行，避免长请求同步阻塞。

`translate_image_text` 采用 OCR → 翻译 → 样式分组 → 确定性渲染链路。OCR 配置统一放在「设置 → 图片 AI 设置」，由图片文字翻译任务读取用户配置，不允许在代码中写死 Provider、服务地址或 API Key。当前下拉只显示生产可用 Provider：`ai_vision`（当前 AI 设置中的视觉模型）、`paddleocr`（本地 PaddleOCR 服务）、`aliyun`（阿里云 OCR）与 `tencent`（腾讯云 OCR）。图片文字翻译采用严格 OCR 模式：用户选择哪个 OCR Provider，任务就必须真实调用该 Provider；OCR 未配置、测试未通过、调用失败或未识别到文字时任务直接失败，不会自动切换到其他 OCR。腾讯云 OCR 支持 `GeneralBasicOCR` / `GeneralFastOCR`，SecretId / SecretKey 加密保存且前端仅脱敏展示；返回的 `TextDetections` 会转换为统一 OCR blocks，低于 `ocr_min_confidence` 的文字块会被过滤。任务详情输出 configuredOcrProvider、actualOcrProvider、ocrBlocksCount、ocrAverageConfidence 与错误信息。设置页提供 OCR 真实调用测试，阿里云与腾讯云都会真实调用服务并校验 blocks 与 bbox。文字会先聚合为 `main_title`、`badge`、`bottom_badge` 等 group，再按 `auto` / `title_badge` / `preserve_original` 等模板排版；黑底标签会重绘圆角胶囊背景，普通文本优先局部擦除并继承原图字重、颜色和对齐，不再默认用白色矩形覆盖所有区域。结果需输出 `renderQuality` 评分，低于商用阈值时标记 `success_with_warnings`。

## Platform Provider

用于接入跨境电商平台能力。

Douyin Shop (`douyin_shop`) Phase 3 adds a reusable OpenAPI client under `backend/internal/providers/platform/douyinshop`. Signing, common request construction, `param_json` body handling, response parsing, error mapping, safe request logging, token auto-refresh, and shop-info calibration are centralized in the provider package. Business services should call this client instead of hand-writing signatures or raw OpenAPI requests. Store connection testing and manual shop-info sync now use a real platform-side token refresh response to update `shops` / `shop_auth_tokens`; App Secret, access token, refresh token, and full sensitive raw responses must never be returned to the frontend or written to logs.

Douyin Shop Phase 4 adds category and category-attribute sync using official-doc-checked OpenAPI methods `shop.getShopCategory` (`/shop/getShopCategory`, recursive from `cid=0`) and `product.getCatePropertyV2` (`/product/getCatePropertyV2`, `category_leaf_id`). Category data is cached in `platform_categories` and attributes in `platform_category_attributes`; raw responses are stored for backend diagnostics but omitted from normal frontend views. Product Detail → Listing saves Douyin listing preparation to `product_platform_publish_configs` (`platform=douyin_shop`, `shopId`, `categoryId`, `categoryPath`, `platformAttributes`) instead of mutating collected raw data. Readiness checks validate store authorization, selected leaf category, required attributes, and stale cache warnings. Phase 4 deliberately does not implement Douyin product publishing, image upload, order sync, or inventory sync.

Douyin Shop Phase 5 adds internal product draft → Douyin listing draft mapping. Mapping is implemented in the product service layer and stored on `product_platform_publish_configs` as preview fields (`mappedTitle`, `mappedDescription`, `mappedImages`, `mappedSkus`, `mappedPrice`, `mappedStock`, `mappingWarnings`, `mappingErrors`, `lastMappedAt`). It supports AI title / AI description priority, main/detail image preview with `need_sync` status for external images, category attributes, SKU specs, price/profit checks, stock confirmation, manual adjustment, save, and readiness validation. Phase 5 still does not call Douyin product creation or image upload APIs; Phase 6 should handle Douyin image upload / image service sync through Provider abstractions.

当前重点平台：

- Douyin Shop（抖店，真实平台闭环优先）
- TikTok Shop
- Shopee
- Lazada
- Amazon

当前真实平台接入顺序优先跑通抖店，不要把抖店与 TikTok Shop 混用：抖店统一内部标识为 `douyin_shop`，TikTok Shop 仍代表跨境平台。已完成 Phase 1 平台配置与 Provider 注册、Phase 2 OAuth 店铺授权闭环、Phase 3 OpenAPI Client / 签名层 / 店铺信息校准、Phase 4 类目与属性缓存，以及 Phase 5 商品字段映射与刊登草稿预览。抖店后续 MVP 范围按阶段继续实现图片上传、商品草稿创建、订单同步和库存同步；多平台并行接入、自动直接上架、绕过平台审核、复杂售后退款、复杂财务结算、多仓 WMS 与自动补货均后置。

主要能力：

- 店铺授权
- 店铺信息
- 订单同步
- 商品刊登
- 库存同步
- 客服消息同步与人工发送

平台 App Secret、Access Token、Refresh Token 等必须加密存储。

## Collector Provider

用于接入商品采集来源。

当前重点：

- 1688
- AliExpress beta
- 自定义规则采集 beta

采集服务必须输出统一商品结构，包括标题、图片、属性、SKU、描述图与 raw 原始数据。

## 扩展建议

新增 Provider 时建议：

1. 先定义接口和统一数据结构。
2. 再实现具体 Provider。
3. 所有外部请求设置超时。
4. 不在日志中输出密钥。
5. 对错误进行可读归因，便于前端展示和任务重试。
6. 必要时同步更新 README、本文档和相关设置页面。

新增 Provider 前请复制或参考 [provider-template.md](provider-template.md)，并按 [module-map.md](module-map.md) 检查 settings、环境变量、API、前端页面、任务队列和文档联动。
