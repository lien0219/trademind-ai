# AI 商品运营体验 Phase A1 审计

> 日期：2026-06-17  
> 阶段：AI 商品运营体验 Phase A1 / 商品草稿全链路体验收口  
> 范围：采集商品 → 采集质量检查 → AI 标题 → AI 描述 → 图片处理 → 定价 → 发布检查 → 平台刊登草稿

## Phase A1.2 记录（2026-06-19）

- **目标**：运营可见文案中文化 + 商品详情「刊登」Tab 改造为多平台刊登中心。
- **已完成**：
  - 发布检查 / 采集 warning / 运营进度 / 刊登状态等英文内部码后端统一映射为 `title` + `message`；`technicalDetails.rawCode` 供技术详情折叠。
  - 前端常量：`errorMessages.ts`、`productOperationLabels.ts`、`publishLabels.ts`、`platformLabels.ts`；`copywriting.commonStatusLabel` 扩展 `ready` / `warning` / `draft_created` 等。
  - 新增 `MultiPlatformPublishCenter` 组件：平台 / 店铺多选、多目标检查、批量创建刊登草稿、重试失败目标。
  - API：`publish-targets`、`publish-targets/check`、`publish-targets/create-drafts`；批次表 `product_publish_batches`。
- **未做（留下一阶段）**：多商品批量刊登、统一配置完整表单、跨境平台真实 API 草稿创建升级。
- **抖店**：现有图片同步 / 类目属性 / create-draft / SKU 校准 / 库存同步链路未改 OpenAPI 字段。

## Phase A1.1 补强记录（2026-06-19）

- 本轮未扩展新平台、未新增重型 ERP 能力，只对 Phase A1 已交付链路做稳定性补强。
- 已补强内容：
  - AI 标题 / 描述应用与撤销的事务内重读、条件更新、失败任务拒绝应用、快照冲突校验。
  - 商品列表 `operationStep` / `publishable` / `readinessBlocked` 过滤条件与真实进度规则对齐。
  - 商品详情 `tab + section` 深链与发布检查问题直达入口补强。
  - 多分辨率下的区段滚动与“继续处理”定位能力补强。
- 新增验收文档：[`AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md`](AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md)
- 当前状态：
  - 代码级补强已落地。
  - 自动化回归已覆盖关键路径。
  - 真实商品样本试跑与人工视觉验收仍待执行。
  - 因此 **Phase A2 暂不开闸**。

## 扫描结论

本轮按 Phase A1 要求扫描了以下入口：

- 后端商品主链路：`backend/internal/modules/product`
- 后端 AI 文本任务：`backend/internal/modules/aitask`、`backend/internal/modules/aioperationbatch`
- 后端图片任务：`backend/internal/modules/imagetask`
- 后端刊登：`backend/internal/modules/productpublish`
- 后端定价：`backend/internal/modules/pricing`
- 后端运营看板：`backend/internal/modules/operationdashboard`
- 后端失败中心：`backend/internal/modules/taskcenter`
- 后端发布检查：`backend/internal/modules/productcheck`
- 管理端商品页：`admin/src/pages/Product`
- 管理端 AI 页：`admin/src/pages/AI`
- 管理端看板：`admin/src/pages/Dashboard`
- 管理端服务、公共组件、常量：`admin/src/services`、`admin/src/components/ui`、`admin/src/constants`

说明：仓库中没有 `backend/internal/modules/ai` 与 `backend/internal/modules/image` 目录。实际 AI 文本能力在 `product`、`aitask`、`aioperationbatch`；实际图片能力在 `imagetask` 与管理端 `admin/src/pages/AI/ImageTasks`。

## 已有能力

- 商品草稿列表已有分页、状态、来源、关键词、看板深链筛选：`missingAiTitle`、`missingAiDescription`、`readiness=blocked`、`publishable=1`。
- 商品详情已有基础信息、SKU、图片、发布检查、刊登、抖店刊登草稿相关能力。
- AI 标题已有 `POST /api/v1/products/:id/ai/optimize-title`，结果写入 `ai_tasks`，应用接口为 `POST /api/v1/products/:id/apply-ai-title`。
- AI 描述已有 `POST /api/v1/products/:id/ai/generate-description`，结果写入 `ai_tasks`，应用接口为 `POST /api/v1/products/:id/apply-ai-description`。
- AI 标题和 AI 描述当前只写 `products.ai_title` / `products.ai_description`，不会直接覆盖人工 `title` / `description`。
- 图片 AI 任务已有统一 `imagetask`，支持去背景、换背景、文字翻译、图片评分、保存 AI 结果到商品等能力。
- 定价已有 `pricing` 模块和 `POST /api/v1/products/:id/pricing/apply`，只修改本地 SKU 销售价。
- 发布前检查已有 `productcheck` 模块，返回 `passed / warning / failed` 口径，且明确不调用平台 API、不写商品、不创建任务。
- 刊登草稿已有 `productpublish` 与抖店草稿链路，抖店当前仍是 Release Candidate。
- 操作日志已有 `operationlog`，商品、AI、定价、发布检查、刊登等已有部分事件。
- 公共 UI 已有 `TmPageContainer`、`SectionCard`、`StatusTag`、`EmptyState`、`TechnicalDetails`、`TaskJsonBlock`、`layoutTokens`、`copywriting`、`errorMessages`。

## 可直接复用能力

- 运营进度里的发布检查必须复用 `productcheck.CheckProductReadiness`，不在前端复制判断。
- 运营进度里的图片问题跳转复用现有图片管理 Tab 与 `imagetask`，不新增图片任务系统。
- 运营进度里的价格处理复用现有 SKU 编辑与定价规则入口。
- AI 结果展示复用 `ai_tasks` 的输出、模板、生成时间和任务归属。
- 商品列表深链筛选复用现有 `GET /api/v1/products`，只扩展轻量摘要字段，避免新增割裂列表接口。
- 发布检查问题直达使用现有商品详情 Tab，加稳定 `tab` 与 `section` query。
- 任务失败恢复继续复用 `taskcenter`，本阶段不新增复杂任务中心。

## 当前体验断点

- 商品列表只显示商品基础字段，没有“运营完成度 / 当前步骤 / 下一步”摘要。
- 商品详情缺少全局“商品运营进度”，用户需要自己在多个 Tab 中寻找下一步。
- 发布前检查虽然有检查项，但缺少统一的问题处理动作映射，用户无法一键跳到价格、图片、描述、参数等位置。
- AI 标题和 AI 描述的生成结果与原内容对比不统一。
- AI 应用接口没有 `expectedUpdatedAt` / 内容快照校验；如果 AI 任务生成后商品又被人工修改，当前无法明确阻止静默覆盖 `ai_title` / `ai_description`。
- AI 应用前没有专门的应用快照表，无法安全撤销最近一次应用。
- URL query 当前只显式处理了 `tab=readiness`，对图片、价格、刊登、发布检查等目标位置支持不完整。
- 进度加载失败没有独立的局部错误状态。

## 本阶段修改计划

1. 新增商品运营进度模型，实时计算完成度、当前步骤、下一步动作、阻断项和建议检查项。
2. 扩展 `GET /api/v1/products/:id/operation-progress`，并在商品列表返回轻量 `operationProgress` 摘要。
3. 商品列表继续走分页查询，批量加载图片、SKU、最近图片任务和必要发布检查数据，避免逐行 N+1。
4. 新增 AI 内容应用记录表，保存应用前后必要快照，用于安全撤销。
5. 扩展 AI 标题/描述应用接口，支持 `expectedUpdatedAt` 和 source snapshot 校验，冲突时拒绝静默覆盖。
6. 新增 AI 标题/描述撤销接口，只恢复最近一次仍安全可恢复的 `ai_title` / `ai_description`。
7. 商品详情顶部增加“商品运营进度”，下一步按钮可打开对应 Tab 或稳定 section。
8. 发布检查展示可处理清单，按 failed 优先、warning 其次，默认最多 5 条，可展开技术详情。
9. 商品列表增加完成度、当前步骤、待处理问题和“继续完善”主操作。
10. 同步 API、README、README.en、DEMO_CHECKLIST 与 PROGRESS 文档。

## 不修改范围

- 不修改抖店 Provider。
- 不修改抖店接口字段。
- 不新增真实平台。
- 不实现售后退款、财务结算、多仓、WMS、自动补货、复杂 BI。
- 不实现自动直接上架。
- 不让 AI 自动覆盖人工 `title` / `description`。
- 不重写图片 AI、发布检查、任务中心或状态字典。
- 不引入复杂状态机；运营进度优先由现有数据实时计算。

## Phase A1 落地结果

- 已新增只读商品运营进度接口 `GET /api/v1/products/:id/operation-progress`，并扩展商品列表 `operationProgress` 摘要。
- 商品详情顶部已展示完成度、当前步骤、阻断/建议数量和下一步入口。
- 商品草稿列表已展示完成度、当前步骤、待处理数量，并支持按运营步骤筛选。
- 发布检查继续复用既有 readiness 结果；问题直达统一映射到商品详情 Tab。
- AI 标题和 AI 描述已支持原始内容、AI 建议、准备应用内容对比，应用前可人工编辑。
- AI 应用接口已增加 `expectedUpdatedAt` / `sourceSnapshotHash` 冲突保护，冲突时拒绝静默覆盖。
- 已新增 `product_ai_content_applications` 应用快照，用于最近一次安全撤销。
- 本阶段未修改抖店 Provider 或抖店接口字段；抖店仍保持 Release Candidate。
- 本阶段未新增售后、财务、多仓、WMS、复杂 BI 或自动直接上架。
