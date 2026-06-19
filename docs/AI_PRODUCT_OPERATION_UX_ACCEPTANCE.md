# AI 商品运营体验 Phase A1.1 验收记录

> 日期：2026-06-19  
> 阶段：AI 商品运营体验 Phase A1.1 / 真实商品样本试跑、进度规则校准与多分辨率视觉验收  
> 目标：在不扩展重型功能、不新增平台的前提下，补齐 Phase A1 的稳定性、跳转一致性、冲突保护和验收口径。

## 结论摘要

- 当前仓库已完成 **Phase A1.1 代码级补强**，重点覆盖：
  - AI 标题 / 描述应用与撤销的并发保护、快照校验、失败任务拒绝应用。
  - 商品列表 `operationStep` / `publishable` / `readiness` 过滤条件与实际进度规则对齐。
  - 商品详情 `tab + section` 深链回跳与发布检查问题直达入口补强。
  - 多分辨率视觉验收所需的页面结构与锚点/滚动逻辑补强。
- 当前仓库**尚未完成真实 20 个商品样本试跑与人工视觉验收**，因此本文件只能给出“自动化与代码级通过 / 人工样本待执行”的状态。
- **Phase A2 开闸结论：暂不满足。**
  - 阻塞原因不是代码继续报错，而是缺少本地环境无法完成的真实样本与人工验收记录。

## 本轮已落地补强

### 1. AI 内容安全应用 / 撤销

- `apply-ai-title` / `apply-ai-description` 现在会：
  - 校验 `expectedUpdatedAt`
  - 校验 `sourceSnapshotHash`
  - 拒绝非 `success` 的 AI 任务
  - 在事务中重新读取商品并做条件更新，避免静默覆盖
- `undo` 现在会：
  - 只撤销最近一次仍可安全回退的应用记录
  - 校验当前 AI 字段值仍等于当时应用值
  - 使用条件更新避免双撤销或并发误撤销
- 新增 / 强化测试覆盖：
  - 失败任务不可应用
  - 页面已过期或源内容已变更时不可应用
  - 人工修改后不可静默撤销

### 2. 进度规则与列表筛选对齐

- 商品列表的 `operationStep`、`readinessBlocked`、`publishable` 查询已与实际进度判断统一：
  - 标题优先使用 `title`，为空时回退 `original_title`
  - 描述优先使用 `description`，为空时回退 `ai_description`
  - 图片判断与进度逻辑一致：允许非 SKU 的可用图片作为兜底
- 同时修正了本地测试数据库里暴露出的字段命名与软删假设偏差：
  - 明确 `products.ai_title`
  - 明确 `products.ai_description`
  - 去掉 `product_images` / `product_skus` 上不存在的 `deleted_at` 依赖

### 3. 商品详情跳转与视觉可达性

- 商品详情现在支持从 query 中读取 `tab` 与 `section`。
- 发布检查 / readiness 中的“去处理”动作现在不只是切换 Tab，还会尽量滚动到对应区段。
- 已补强的区段包括：
  - `title`
  - `description`
  - `collect-review`
  - `attributes`
  - `image-list`
  - `pricing`
  - `local-skus`
  - `publish-check`
  - `publish-config`

## 自动化验证结果

本地已完成：

- `pnpm --dir admin build`
- `go test ./internal/modules/product/...`

建议在完整收口时继续执行：

- `go test ./...`
- `go build ./cmd/server/...`
- `git diff --check`
- 视时间追加：
  - `go test ./internal/providers/platform/douyinshop/...`
  - `go test ./internal/modules/productpublish/...`
  - `go test ./internal/modules/ordersync/...`
  - `go test ./internal/modules/inventory/...`

## 人工验收待办

以下仍需在真实样本和浏览器中完成：

1. 选择至少 20 个真实商品草稿样本，覆盖：
   - 1688
   - 拼多多
   - 淘宝 / 天猫
   - 自定义链接
2. 在桌面宽屏、常见笔记本宽度、窄屏窗口下检查：
   - 商品列表完成度展示
   - 商品详情顶部进度卡
   - 发布检查问题跳转
   - AI 标题 / 描述对比与应用交互
   - 图片 / SKU / 库存 / 刊登页签可达性
3. 记录至少一轮人工冲突回归：
   - 先生成 AI 内容
   - 再人工改商品字段
   - 确认应用或撤销时提示冲突，而不是静默覆盖
4. 补齐截图、样本编号、通过 / 不通过结论。

## A2 开闸判断

当前判断：**不开闸**

原因：

- 自动化与代码级回归已经通过。
- 真实商品样本试跑、人工视觉验收、多分辨率截图证据尚未完成。
- 因此还不能把 Phase A1.1 视为“体验验收完成”，只能视为“代码补强完成，待人工验收收口”。

下一步建议：

1. 按 `DEMO_CHECKLIST.md` 和本文件完成样本验收。
2. 将样本结果回填到本文件。
3. 样本验收通过后，再评估是否进入 Phase A2。
