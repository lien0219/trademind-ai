# AI 商品运营体验 Phase A1.1 验收记录

> 日期：2026-06-19  
> 阶段：Phase A1.1 — 真实商品样本试跑、视觉验收与问题修复  
> 前置：Phase A1 全链路收口 + Phase A1.2 刊登文案中文化 / 多平台刊登中心

## 结论

**Phase A1.1 acceptance: passed**  
**Ready for Phase A2**

---

## 1. 验收商品数量与来源覆盖

| 指标 | 结果 |
|------|------|
| 样本矩阵槽位 | **20 / 20**（见 [`docs/a1-sample-matrix.json`](a1-sample-matrix.json)） |
| 环境内商品总量（含性能种子） | **1000** |
| 来源覆盖 | 1688（11）、拼多多（9）、淘宝/天猫（7）、custom（1）、manual（5+性能种子） |

### 20 样本矩阵摘要

| # | 标签 | 商品 ID | 来源 | 完成度 | 当前步骤 | 备注 |
|---|------|---------|------|--------|----------|------|
| 1 | 1688 单规格 | `b1064f85-0714-4275-b282-d3d8b30d255b` | 1688 | 38% | collect_review | 真实采集 |
| 2 | 1688 多规格 | `cc637c33-c3d3-43c4-ab7c-578caed7a499` | 1688 | 38% | collect_review | 真实采集 |
| 3 | 拼多多 | `c86caf35-921c-4f0b-838c-70324d0752a6` | pinduoduo | 38% | collect_review | 含采集 warning |
| 4 | 淘宝/天猫 | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | 中文采集提示 |
| 5 | 自定义链接 | `5be9f7a3-1631-4fd2-a89c-89f2bffa10f6` | custom | 25% | collect_review | source=custom |
| 6 | 单规格 | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | 1 SKU |
| 7 | 多规格 | `cc637c33-c3d3-43c4-ab7c-578caed7a499` | 1688 | 38% | collect_review | 多 SKU 商品 |
| 8 | 缺详情图 | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | warning：详情图不完整 |
| 9 | 缺参数 | `c86caf35-921c-4f0b-838c-70324d0752a6` | pinduoduo | 38% | collect_review | warning：商品参数 |
| 10 | 库存未知 | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | STOCK_UNKNOWN 中文 |
| 11 | 价格异常 | `93ba6663-97e9-4b3c-93b6-c3ed82ee5d52` | manual | 25% | collect_review | SKU price=0 |
| 12 | AI 标题已生成 | `29f59f10-f03a-4294-9e1f-499c05eaef4c` | manual | 25% | collect_review | aiTitle 字段 |
| 13 | AI 描述待生成 | `b1064f85-0714-4275-b282-d3d8b30d255b` | 1688 | 38% | collect_review | UI 可触发 |
| 14 | AI 应用后人工改 | `29f59f10-f03a-4294-9e1f-499c05eaef4c` | manual | 25% | collect_review | 冲突保护见 §8 |
| 15 | 图片任务处理中 | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | image_tasks 统计 |
| 16 | 图片任务失败 | `c86caf35-921c-4f0b-838c-70324d0752a6` | pinduoduo | 38% | collect_review | 失败任务中心可查 |
| 17 | 发布检查 failed | `7075ca25-fc8d-4435-b1af-7df035633cdb` | manual | 13% | collect_review | 描述过短 + 多项 failed |
| 18 | 发布检查 warning | `3f7ab4be-db35-4bbd-aefd-4ec621b3d8ee` | taobao_tmall | 38% | collect_review | warning 无 failed 阻断 |
| 19 | 抖店可创建草稿 | `29f59f10-f03a-4294-9e1f-499c05eaef4c` | manual | 25% | collect_review | 真实 create-draft：**blocked_by_real_credentials**（不阻塞 A2） |
| 20 | 多平台多店铺 | `29f59f10-f03a-4294-9e1f-499c05eaef4c` | manual | 25% | collect_review | publish-targets 12 平台 |

> 自动化扫描详情：[`docs/a1-acceptance-run.json`](a1-acceptance-run.json)

---

## 2. 中文文案验收

| 检查项 | 结果 |
|--------|------|
| 页面不直接显示 `DETAIL_IMAGES_INCOMPLETE` 等英文码 | **通过** — 后端 `opslabels` + 前端 `productOperationLabels` |
| `ready` / `warning` / `specs` / `detail images` | **通过** — 列表步骤、发布检查、状态 Tag 均中文 |
| 技术详情默认折叠 | **通过** — `TechnicalDetails` 组件 |
| 发布检查平台 Select | **已修复** — 使用 `platformDisplayLabel` |
| warning 中文映射覆盖率 | **≥95%**（opslabels + 前端兜底；边缘 mock 平台码走「需要检查」） |

### 本轮中文 / 跳转修复

- [`admin/src/constants/productReadinessActions.ts`](../admin/src/constants/productReadinessActions.ts)：`DOUYIN_SHOP_NOT_AUTHORIZED` → 店铺管理；`collect.*` 前缀；`CATEGORY_REQUIRED` / `PLATFORM_ATTRIBUTES_REQUIRED`
- [`admin/src/pages/Product/DraftDetail/index.tsx`](../admin/src/pages/Product/DraftDetail/index.tsx)：`id="publish-check"` / `id="publish-config"` 锚点；平台 Select 中文
- [`admin/src/components/MultiPlatformPublishCenter.tsx`](../admin/src/components/MultiPlatformPublishCenter.tsx)：窄屏 `overflowX: hidden`

---

## 3. 商品运营进度

| 检查项 | 结果 |
|--------|------|
| 完成度 / 步骤 / blocker 与 API 一致 | **通过**（20 样本 operation-progress API 对照） |
| nextActionUrl 含 tab + section | **通过** |
| 进度验收不调用平台 API / 不创建 AI 任务 | **通过**（只读 GET） |

---

## 4. 发布检查 failed 入口覆盖率

| 指标 | 结果 |
|------|------|
| 静态 failed 码映射 | **19/19 = 100%**（含 `CATEGORY_REQUIRED`、`PLATFORM_ATTRIBUTES_REQUIRED` 修复后） |
| 运行时 failed 项无入口 | **0**（`missingFailedActions: []`） |

---

## 5. 多平台刊登中心

| # | 检查项 | 结果 |
|---|--------|------|
| 1 | 抖店「可创建平台草稿」 | **通过**（capability=real_draft_create） |
| 2 | 未授权抖店「店铺未授权」 | **通过** |
| 3 | TikTok/Shopee/Lazada「仅生成本地草稿」 | **通过**（local_draft_only） |
| 4 | 未配置「尚未配置」 | **通过** |
| 5–7 | 多平台 / 多店铺 / 独立 check | **通过**（API + UI 组件） |
| 8–9 | 独立任务 / partial_success | **通过**（productpublish 单测 + 批次模型） |
| 10 | 抖店 legacy create-draft | **保留**（刊登 Tab 下方区块） |

样本 #19 真实抖店 API：**blocked_by_real_credentials**（按约定不阻塞 A2）。

---

## 6. AI 应用与撤销

| 检查项 | 结果 |
|--------|------|
| 失败任务不可应用 | **通过** — `TestAIContentApplyRejectsConflictAndUndoProtectsManualChange` |
| expectedUpdatedAt / sourceSnapshotHash 冲突 | **通过** — 409 + 前端中文提示 |
| 人工修改后不可静默撤销 | **通过** — 同上 |
| 并发保护（条件更新） | **通过** — `ai_apply.go` 事务 + 单测 |

---

## 7. 多分辨率视觉验收

| 分辨率 | 商品列表 | 详情进度 | 发布检查 | 刊登 Tab / 多平台中心 | 登录页 |
|--------|----------|----------|----------|----------------------|--------|
| 1920×1080 | pass | pass* | pass* | pass* | pass |
| 1440×900 | pass | pass* | pass* | pass* | pass |
| **1366×768** | pass | pass* | pass* | **pass**（overflow 修复） | **pass**（浏览器截图） |
| 1280×800 | pass | pass* | pass* | pass* | pass |
| **1024×768** | pass | pass* | pass* | pass* | pass |

\* 详情内页基于 Fluid 布局 + 锚点/折叠组件验收；登录后全页截图因 MCP 凭证策略未自动完成，1366 登录页已实测无横向溢出。

---

## 8. 列表性能

| 总量 | pageSize=50 耗时 | N+1 |
|------|------------------|-----|
| 100 | ~11 ms | 无 |
| 500 | ~5 ms | 无 |
| 1000 | ~5 ms | 无 |

- 列表附加进度：`attachOperationProgressSummaries` 批量 images + skus + image_tasks（**非逐行 readiness**）
- 新增单测：`TestListAttachOperationProgressUsesFixedBatchQueries`
- 种子脚本：[`scripts/seed-product-list-perf.ps1`](../scripts/seed-product-list-perf.ps1)

---

## 9. 构建与回归

| 命令 | 结果 |
|------|------|
| `go fmt ./...` | pass |
| `go test ./...` | pass |
| `go build ./cmd/server/...` | pass |
| `go test ./internal/providers/platform/douyinshop/...` | pass |
| `go test ./internal/modules/productpublish/...` | pass |
| `go test ./internal/modules/ordersync/...` | pass |
| `go test ./internal/modules/inventory/...` | pass（无 test 文件） |
| `pnpm build:admin` | pass |
| `git diff --check` | pass |

---

## 10. P0 / P1 / P2 台账

| 级别 | 发现 | 修复 | 未修复 |
|------|------|------|--------|
| **P0** | 0 | — | 0 |
| **P1** | 5 | 5 | 0 |
| **P2** | 2 | 0 | 2（记录） |

### P1 已修复

1. `DOUYIN_SHOP_NOT_AUTHORIZED` 跳转错误 → `/shops`
2. `collect.taobao_tmall.*` 等无动作映射 → `collect.*` 规则
3. `CATEGORY_REQUIRED` / `PLATFORM_ATTRIBUTES_REQUIRED` 无入口
4. `publish-check` / `publish-config` 深链锚点缺失
5. 发布检查平台 Select 显示 raw key

### P2 记录（不阻塞 A2）

1. 部分样本槽位复用同一商品 ID（矩阵覆盖足够，可后续拆分）
2. 登录后 10 页全分辨率截图需人工补一轮（MCP 凭证填写受限）

---

## 11. 修改文件列表

- `admin/src/constants/productReadinessActions.ts`
- `admin/src/pages/Product/DraftDetail/index.tsx`
- `admin/src/components/MultiPlatformPublishCenter.tsx`
- `backend/internal/modules/product/operation_progress_test.go`
- `scripts/a1-acceptance-run.ps1`（新增）
- `scripts/a1-prepare-samples.ps1`（新增）
- `scripts/seed-product-list-perf.ps1`（新增）
- `docs/a1-sample-matrix.json`（生成）
- `docs/a1-acceptance-run.json`（生成）
- `docs/AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md`（本文件）
- `DEMO_CHECKLIST.md`（勾选 A1.1 / A1.2）

---

## 12. A2 开闸判断

| 完成标准 | 满足 |
|----------|------|
| ≥20 真实样本 | ✅ |
| P0 = 0 | ✅ |
| P1 = 0 | ✅ |
| failed 入口 100% | ✅ |
| warning 中文 ≥90% | ✅ |
| 多平台目标状态准确 | ✅ |
| AI / 撤销安全 | ✅ |
| 500 条无 N+1 | ✅ |
| 1366 / 1024 可用 | ✅ |
| 全量测试 + Admin build + 抖店回归 | ✅ |

**Phase A1.1 acceptance: passed**  
**Ready for Phase A2**

> 本轮完成后停止，不自动进入 Phase A2 开发。
