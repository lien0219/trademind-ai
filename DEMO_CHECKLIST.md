# TradeMind 演示验收清单（Demo Checklist）

> 用于本地 / 预发环境快速验收核心 MVP 能力。按模块勾选。

## 抖店完整演示流程（MVP Demo，2026-06-07）

> **Phase 10.4**：发布状态 **Release Candidate**；无真实凭证时 E2E 脚本 exit `3`（`blocked_by_real_credentials`）。预检脚本：`scripts/douyin-e2e-preflight.ps1`（Windows）或 `.sh`。门禁：[`docs/DOUYIN_RELEASE_GATE.md`](docs/DOUYIN_RELEASE_GATE.md)。
> 真实抖店凭证 + 公网 Storage 环境下按顺序演示。详细每步预期/失败排查见 [`docs/DOUYIN_E2E_CHECKLIST.md`](docs/DOUYIN_E2E_CHECKLIST.md)。

| # | 步骤 | 入口 | 勾选 |
| --- | --- | --- | --- |
| 1 | 登录系统 | `/user/login` | [ ] |
| 2 | 查看平台开放配置 | 设置 → 平台开放配置 | [ ] |
| 3 | 配置抖店应用 | 抖店 Tab：App Key / Secret / 回调 / 开关 | [ ] |
| 4 | 授权抖店店铺 | 连接店铺 或 店铺管理 → OAuth | [ ] |
| 5 | 同步类目 | 平台开放配置 → 同步类目 | [ ] |
| 6 | 采集商品 | 采集中心（1688 / 拼多多 / 淘宝天猫） | [ ] |
| 7 | 商品草稿查看 | 商品 → 草稿 → 详情 | [ ] |
| 8 | AI 标题 / 描述 | 商品详情 → AI 优化 | [ ] |
| 9 | 定价规则 | 刊登 Tab → 应用定价规则 | [ ] |
| 10 | 图片处理 | 同步图片到 Storage；可选 AI 图片任务 | [ ] |
| 11 | 抖店刊登配置 | 刊登 Tab → 店铺 / 类目 / 属性 | [ ] |
| 12 | 上传图片 | 刊登 Tab → 上传图片到抖店 | [ ] |
| 13 | 创建抖店商品草稿 | 刊登 Tab → 创建抖店商品草稿（非直接上架） | [ ] |
| 14 | SKU 绑定校准 | 刊登 Tab → 校准 + 手动绑定 | [ ] |
| 15 | 订单同步 | 店铺管理 → 同步订单 | [ ] |
| 16 | 库存同步 | 商品详情 → 库存 Tab → 同步到抖店 | [ ] |
| 17 | 失败任务中心 | 运维 → 失败任务中心 → 筛选抖店相关 | [ ] |
| 18 | 商品运营看板 | 工作台 → 商品运营看板 | [ ] |

**演示前确认：** Storage `public_base` 为抖店可访问公网地址；`order_sync_enabled` 与 `inventory_sync_enabled` 已开启。

## 淘宝/天猫采集器（已可用，2026-06-06）

- [ ] 采集中心卡片显示 **淘宝/天猫采集器**，状态 **已可用**
- [ ] 卡片说明含标题、价格、主图、详情图、商品参数、商品规格
- [ ] **开始采集**、**批量采集**、**采集设置** 按钮可用
- [ ] 批量采集提示：逐条采集，建议每批不超过 20 条
- [ ] `item.taobao.com` 链接识别为 `taobao_tmall`（非 custom）
- [ ] `detail.tmall.com` / `detail.tmall.hk` / `chaoshi.tmall.com` / `ju.taobao.com` 识别为 `taobao_tmall`
- [ ] 淘宝店铺页/搜索页提交时提示 **UNSUPPORTED_TAOBAO_URL** 或自动跳过（批量）
- [ ] 未登录时批量任务 **不能开始**（提示先登录）
- [ ] 需要安全验证时提示 **VERIFY_REQUIRED**，引导在采集浏览器完成验证
- [ ] 登录后普通商品能采集 **标题、价格、至少 1 张主图**
- [ ] **批量采集** 逐条执行（并发 1，不高并发打开）
- [ ] 批量 20 条中部分失败时批次状态为 **partial_success**
- [ ] 成功的链接创建商品草稿：`source=taobao_tmall`、`currency=CNY`、`status=draft`
- [ ] 失败的链接进入 **失败任务中心**，可重试单条
- [ ] 商品详情页顶部显示淘宝/天猫来源提示
- [ ] 发布前检查拦截缺价、缺主图、SKU 不完整等
- [ ] 设置 → 采集服务 → 淘宝/天猫 **批量配置**（开关、每批上限、并发、间隔、重试、登录/验证暂停）
- [ ] 操作日志含批量创建、开始、完成、子任务成功/失败、跳过无效链接

### 演示流程（建议顺序）

1. **设置**：进入「设置 → 采集服务 → 淘宝/天猫」，点击「打开淘宝/天猫采集浏览器」完成登录，点「重新检测」确认已登录；确认批量采集开关已开启。
2. **单采**：采集中心 → 淘宝/天猫采集器 → 开始采集 → 粘贴商品链接 → 提交。
3. **批采**：采集中心 → 批量采集（或批量采集页选淘宝/天猫）→ 粘贴 2–5 条有效链接 → 提交 → 在批次详情查看逐条结果。
4. **草稿**：打开成功任务对应的商品草稿，核对标题、价格、主图、规格。
5. **失败回归**：未登录或已下架链接各测一条，在失败任务中心查看原因与重试。

详细链接验收表见 [`docs/collector-taobao-tmall-test-links.md`](docs/collector-taobao-tmall-test-links.md)。

## 其他采集器（回归抽查）

- [ ] 1688 单链接 / 批量仍可用
- [ ] 拼多多单链接 / 批量仍可用
- [ ] 自定义采集器对淘宝/天猫链接提示使用专用采集器
- [ ] 当前采集能力阶段性验收记录：1688 已可用、拼多多已可用、淘宝/天猫已可用、自定义链接基础可用、速卖通测试中、SHEIN/Temu 规划中

## 发布刊登生产级闭环

### AI 商品运营体验 Phase A1.2（2026-06-19）

- [x] 商品详情顶部 / 发布检查 / 采集 warning **不再直接显示** `DETAIL_IMAGES_INCOMPLETE` 等英文码
- [x] `ready` / `warning` / `draft` 等状态显示中文（如「已准备好」「建议检查」）
- [x] 刊登 Tab → **多平台刊登中心**：可选择多个平台、多个店铺
- [x] 未授权店铺提示去授权；未配置平台提示「尚未配置」
- [x] `local_draft_only` 平台提示「仅生成本地草稿」，创建后任务成功但不调用外部 API
- [x] 抖店目标仍可走「创建抖店商品草稿」原链路
- [x] 「检查所选目标」→「创建刊登草稿」→ 部分失败时批次 `partial_success`
- [x] 1366px 宽度下刊登 Tab 不溢出；技术详情默认折叠

## Phase A2 多商品批量刊登草稿

- [x] 商品草稿列表多选 →「批量创建刊登草稿」
- [x] 向导 5 步：确认商品 / 选择平台店铺 / 统一配置 / 单独覆盖 / 检查并创建
- [x] 矩阵检查：可创建 / 建议检查 / 暂不能创建；blocked 项不可强行提交
- [x] 创建后跳转批次详情；刊登任务页「刊登批次」Tab
- [x] 批次详情：子任务列表、重试失败项、取消等待项
- [x] 失败任务中心子任务可跳转批次详情
- [ ] 50 商品 × 2 目标性能人工验收（接口已支持最多 100 商品）

### AI 商品运营体验 Phase A1.1（2026-06-19）

> Phase A1.1 验收已通过，详见 [`docs/AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md`](docs/AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md)。

- [x] 至少抽取 20 个真实商品草稿样本，覆盖 1688、拼多多、淘宝/天猫、自定义链接
- [x] 商品列表完成度、当前步骤、继续处理入口在桌面宽屏 / 笔记本宽度 / 窄屏窗口下显示正常
- [x] 商品详情顶部进度卡、发布检查问题跳转、Tab + section 深链在多分辨率下可用
- [x] AI 标题 / 描述应用后可撤销；人工修改后再次应用或撤销会提示冲突，不静默覆盖
- [x] 本轮人工验收完成，可开启 Phase A2 评估

- [x] 商品草稿列表显示运营完成度、当前步骤、待处理数量和一个主要操作「继续完善」
- [x] 商品草稿列表可按待检查采集结果、待优化标题、待生成描述、待处理图片、待设置价格、发布检查未通过、可以生成刊登草稿筛选
- [x] 商品详情顶部显示商品运营进度；下一步按钮可直接打开对应 Tab（图片、规格、发布检查、刊登等）
- [x] 运营进度加载失败时只显示局部错误，商品内容仍可编辑并可重新加载
- [x] 发布前检查问题按 failed 优先、warning 其次展示，每条问题有建议操作和直达入口，技术详情默认折叠
- [x] AI 标题优化结果展示原始内容、AI 建议、准备应用内容；可编辑后应用，也可撤销最近一次安全应用
- [x] AI 描述生成结果展示原始内容、AI 建议、准备应用内容；可编辑后应用，也可撤销最近一次安全应用
- [ ] AI 应用遇到商品内容已被人工修改时提示冲突，不静默覆盖人工内容
- [ ] 抖店仍为 Release Candidate；本阶段不新增售后、财务、多仓、WMS、复杂 BI 或自动直接上架

- [ ] 分别采集 1 个 1688、拼多多、淘宝/天猫商品，并进入对应商品草稿
- [ ] 商品详情顶部显示采集 warning；普通用户不默认查看 raw，高级详情可展开原始调试数据
- [ ] 统一草稿字段完整：`source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`mainImages`、`descriptionImages`、`attributes`、`skuGroups`、`skus`、`costPrice`、`salePrice`、`currency`、`stock`、`collectWarnings`、`publishStatus`、`raw`
- [ ] 单商品应用定价规则：成本来源、加价方式、运费、佣金、汇率、利润、尾数规则均可预览
- [ ] 批量应用定价规则后写入 SKU 销售价，并能在操作日志看到记录
- [ ] AI 优化标题、AI 生成描述后可应用到草稿字段
- [ ] 发布 Tab 可同步主图、详情图或全部外链图片到当前 Storage Provider
- [ ] 发布前检查结果分为 `passed` / `warning` / `failed`；`failed` 不能创建刊登任务
- [ ] `warning` 创建刊登任务前必须人工确认
- [ ] 创建刊登任务后可查看任务状态、平台草稿快照、SKU 映射与刊登记录
- [ ] 刊登失败进入失败任务中心，错误码可区分 `PUBLISH_CHECK_FAILED`、`PRICE_INVALID`、`IMAGE_MISSING`、`SKU_INVALID`、`STORE_NOT_CONFIGURED`、`PLATFORM_AUTH_REQUIRED`、`PLATFORM_API_ERROR`、`UNKNOWN_PUBLISH_ERROR`
- [ ] 操作日志包含应用定价规则、修改售价、同步商品图片、执行发布前检查、创建刊登任务、刊登成功 / 失败、取消刊登任务（如有）

## 抖店真实平台闭环（下一阶段优先）

### 抖店 Phase 5 商品字段映射与刊登预览

- [x] 商品详情 → 刊登 Tab 可点击 **生成抖店刊登草稿**，预览抖店标题、描述、主图、详情图、SKU、价格、库存、类目和属性
- [x] `aiTitle` 优先进入抖店标题，`aiDescription` 优先进入抖店描述
- [x] 外链主图 / 详情图在草稿预览中标记 **待图片同步**，本阶段不调用抖店图片上传接口
- [x] 抖店刊登草稿可人工修改标题 / 描述 / 抖店要求填写的信息并保存，再次读取不被自动覆盖
- [x] 校验抖店刊登草稿时，标题缺失、主图缺失、类目缺失、必填属性缺失、SKU 价格无效、利润过低、库存无效会失败
- [x] 描述为空、详情图为空、图片待同步、SKU 规格不完整、库存未确认、采集来源复核会作为 warning
- [x] 抖店 Phase 5 只做字段映射、预览、保存和校验；不调用抖店创建商品接口，不写失败任务中心

### 抖店 Phase 6 图片上传

- [x] 商品详情 → 刊登 Tab 可查看主图 / 详情图的 Storage 状态、抖店上传状态、平台图片 ID、上传时间和失败原因
- [x] 点击 **上传图片到抖店** 会读取当前抖店刊登草稿图片；外链图片先同步到当前 Storage Provider，再上传到抖店素材中心
- [x] 已在当前 Storage Provider 的图片由后端读取对象后上传抖店，不依赖前端 URL，不向前端暴露 token / secret
- [x] 支持单张图片重试和重新上传全部图片；已上传且有 `platformImageId` 的图片默认不重复上传
- [x] 发布前检查中主图未上传 / 上传失败会失败，详情图部分失败会 warning
- [x] 抖店 Phase 6 只做图片上传；不调用抖店创建商品接口，不实现订单同步或库存同步

### 抖店 Phase 7 平台商品草稿创建

- [x] 商品详情 → 刊登 Tab 可点击 **创建抖店商品草稿**（默认 `save_as_platform_draft`，不直接上架）
- [x] 未授权店铺、未选类目、必填属性缺失、主图未上传、映射不存在、SKU 价格无效时不能创建
- [x] 存在 warning 时需二次确认后才能继续创建
- [x] 创建成功后返回 `platformProductId`，写入 `product_publications` 与 `product_publication_skus`
- [x] 刊登任务页可查看平台提交内容、抖店商品 ID、失败原因、requestId、重试
- [x] 创建失败进入失败任务中心，错误码含 `DOUYIN_CREATE_PRODUCT_FAILED`
- [x] 不调用订单 / 库存接口；token / secret 不出现在前端和日志

### 抖店 Phase 8 订单同步 MVP

- [x] 设置 → 平台开放配置可开启 **启用订单同步**（默认关闭）
- [x] 已授权抖店店铺可在店铺管理页 **同步订单**（复用 `POST /api/v1/shops/:id/sync-orders`）
- [x] 未授权店铺不能同步；token 过期可自动刷新后重试
- [x] 订单写入 `orders` / `order_items` / `order_shipments`；`platformSkuId` 可匹配 `product_publication_skus`
- [x] 未匹配 / 多候选 SKU 进入订单异常工作台；匹配成功可按策略扣减本地库存
- [x] 重复同步同一订单不重复扣库存；同步失败进入失败任务中心
- [x] 订单同步任务页可查看 SKU 匹配摘要与重试；买家敏感信息脱敏
- [x] 不调用抖店库存同步接口；不做售后 / 退款

### 抖店 Phase 8.1 订单同步分页收口

- [x] 单次同步任务按 `page` / `size` 自动拉取多页（默认最多 5 页或 500 条）
- [x] 平台开放配置支持 **订单同步最大页数**（`order_sync_max_pages`）；任务 body 可传 `maxPages` 覆盖
- [x] 单页失败记录 `page` 与错误原因；部分页成功时任务状态为 **partial_success**
- [x] 任务输出摘要含 `totalFetched` / `totalPages` / `successPages` / `failedPages` / `nextPage` / `createdOrders` / `updatedOrders` / `matchedItems` / `unmatchedItems` / `deductedStockItems`
- [x] 定时轮询仍默认关闭；不调用抖店库存 API；不做售后 / 退款

- [x] 设置 → 平台开放配置可配置 **抖店 / Douyin Shop** 应用信息，App Secret 脱敏展示
- [x] 抖店配置保存后，重新加载仍只展示 App Secret 脱敏值 `****`
- [x] 点击 **测试连接** 时校验配置完整性，抖店 Phase 2 不做商品 / 订单 / 库存真实调用
- [x] 可发起抖店店铺授权，授权成功后店铺状态为已授权
- [x] 支持抖店 token 加密保存、刷新授权、解除授权、店铺连接测试
- [x] 授权过期时提示重新授权，不泄露 access token / refresh token
- [x] Douyin Shop OpenAPI Client and signing layer are centralized; business code does not hand-write signatures
- [x] Douyin Shop connection test performs a real token refresh and calibrates shop basic info
- [x] 店铺管理页支持手动同步抖店店铺信息，失败时标记 need_check / expired / invalid
- [x] 可同步或搜索抖店类目，并读取类目必填属性
- [x] 商品刊登页可选择抖店店铺、抖店类目并补全必填属性
- [x] 必填类目 / 属性 / 店铺授权缺失时发布前检查失败
- [x] 商品图片先同步到 Storage Provider，再上传到抖店图片服务
- [x] 默认创建 **抖店商品草稿**，不默认直接上架
- [ ] 如后续支持直接上架，必须二次人工确认，并保留平台审核提示
- [x] 抖店刊登失败进入失败任务中心，错误码可区分授权、类目、属性、图片上传、创建草稿、权限和限流问题
### 抖店 Phase 9 库存同步 MVP

- [x] 设置 → 平台开放配置可开启 **启用库存同步**（默认关闭）
- [x] 已授权抖店店铺、且商品/SKU 已绑定平台 ID 时，可在商品详情 → 库存 Tab **同步库存到抖店**
- [x] 未授权店铺、未绑定抖店商品 ID、未绑定抖店 SKU ID、本地库存无效时不能同步
- [x] 复用 `POST /api/v1/product-publication-skus/:id/sync-inventory` 与 `POST /api/v1/products/:id/sync-inventory`
- [x] 库存同步任务页可查看状态、失败原因并重试；批量同步默认低并发
- [x] 库存预警页可对已刊登抖店 SKU 触发同步
- [x] 同步失败进入失败任务中心，错误码含 `DOUYIN_INVENTORY_SYNC_FAILED` / `DOUYIN_SKU_NOT_BOUND` 等
- [x] 操作日志包含 `douyin.inventory.sync.*`，且不记录 token / secret
- [x] 不做多仓库存、自动补货、默认定时自动同步

### 抖店 Phase 9.1 SKU 绑定校准

- [x] 创建抖店商品草稿后可 **校准抖店 SKU 绑定**（`product.detail` + 本地匹配）
- [x] 已绑定跳过；规格属性一致 / 规格名+价格一致可自动绑定；多候选 `ambiguous`；无候选 `unmatched`
- [x] 商品详情 → 刊登 Tab 可查看绑定状态与置信度

### 抖店 Phase 9.2 SKU 手动绑定兜底 + 整链路验收准备

- [x] 可查看抖店平台 SKU 候选（`platformSkus`）
- [x] ambiguous / unmatched 规格可 **手动绑定** 抖店 SKU
- [x] 可 **解除绑定** 与 **重新校准**
- [x] 库存同步前校验全部 SKU 绑定状态；存在 unmatched / ambiguous / failed 时禁用同步并提示
- [x] 手动绑定后 `external_sku_id` 回写，`bindStatus=bound`；解除绑定后库存同步禁用
- [x] 绑定冲突（同一抖店 SKU 绑定到多个本地规格）可拦截
- [x] 操作日志含 `douyin.sku.binding.manual_bind/unbind/recheck/conflict`，不记录 token / secret
- [ ] **整链路验收**（需真实抖店凭证）：见上文「抖店完整演示流程」与 [`docs/DOUYIN_E2E_CHECKLIST.md`](docs/DOUYIN_E2E_CHECKLIST.md)

- [x] 可同步抖店库存，失败任务进入失败任务中心
- [x] 操作日志包含抖店库存同步，且不记录 token / secret / 收货敏感信息明文

### 当前阶段不做

- [ ] 不多平台并行接入
- [ ] 不自动直接上架
- [ ] 不绕过平台审核
- [ ] 不做复杂售后退款
- [ ] 不做复杂财务结算
- [ ] 不做多仓 WMS
- [ ] 不做自动补货

## 备注

- 需要真实淘宝/天猫商品链接与可用登录态；部分商品遇风控需人工在采集浏览器完成验证。
- SKU / 库存 / 详情图 / 平台必填类目仍建议发布前人工复核。
- 批量采集默认低并发；超过 20 条请分批提交。
