# TradeMind 演示验收清单（Demo Checklist）

> 用于本地 / 预发环境快速验收核心 MVP 能力。按模块勾选。

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

- [x] 设置 → 平台开放配置可配置 **抖店 / Douyin Shop** 应用信息，App Secret 脱敏展示
- [x] 抖店配置保存后，重新加载仍只展示 App Secret 脱敏值 `****`
- [x] 点击 **测试连接** 时校验配置完整性，抖店 Phase 2 不做商品 / 订单 / 库存真实调用
- [x] 可发起抖店店铺授权，授权成功后店铺状态为已授权
- [x] 支持抖店 token 加密保存、刷新授权、解除授权、店铺连接测试
- [x] 授权过期时提示重新授权，不泄露 access token / refresh token
- [x] Douyin Shop OpenAPI Client and signing layer are centralized; business code does not hand-write signatures
- [x] Douyin Shop connection test performs a real token refresh and calibrates shop basic info
- [x] 店铺管理页支持手动同步抖店店铺信息，失败时标记 need_check / expired / invalid
- [ ] 可同步或搜索抖店类目，并读取类目必填属性
- [ ] 商品刊登页可选择抖店店铺、抖店类目并补全必填属性
- [ ] 必填类目 / 属性 / 店铺授权缺失时发布前检查失败
- [ ] 商品图片先同步到 Storage Provider，再上传到抖店图片服务
- [ ] 默认创建 **抖店商品草稿**，不默认直接上架
- [ ] 如后续支持直接上架，必须二次人工确认，并保留平台审核提示
- [ ] 抖店刊登失败进入失败任务中心，错误码可区分授权、类目、属性、图片上传、创建草稿、权限和限流问题
- [ ] 可手动同步抖店订单，订单进入现有订单模块，未匹配 SKU 进入订单异常工作台
- [ ] 可同步抖店库存，失败任务进入失败任务中心
- [ ] 操作日志包含抖店授权、类目同步、图片上传、商品草稿创建、订单同步、库存同步，且不记录 token / secret / 收货敏感信息明文

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
