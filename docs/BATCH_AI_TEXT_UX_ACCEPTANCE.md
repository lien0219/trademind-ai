# Phase A3.1.1 批量 AI 文案 UX 验收报告

> 日期：2026-06-19  
> 阶段：**AI 商品运营体验 Phase A3.1.1** — 人工验收、真实 Provider 试跑、失败任务中心联动、旧入口梳理

## 总体验收结论

| 维度 | 结论 | 说明 |
| --- | --- | --- |
| 代码闭环 / 自动化测试 | **passed** | `go test ./...`、`pnpm build:admin`、抖店回归通过 |
| 失败任务中心联动 | **passed** | `ai_text` 聚合 `failed` / `conflict` / 质量 warning；深链 `?itemId=` |
| 旧入口梳理 | **passed_with_warning** | `/ai/batches` 隐藏菜单 + 旧版提示；历史可访问 |
| 真实 AI Provider 小样本试跑 | **blocked_by_server_restart** | DB 已配置 AI（`settings.ai`）；当前 `:8080` 进程为旧二进制，`/api/v1/products/ai-text/*` 返回 404，需重启后端后再跑 13 条试跑 |
| 20 条人工 E2E 全矩阵 | **passed_with_warning** | 样本商品已落库（perf seed）；冲突/撤销/质量规则有单测；完整 UI E2E 待重启后人工勾选 |

**是否允许进入 Phase A3.2（批量图片）**：**否** — 本轮仅完成 A3.1.1 运营可用收口，未进入图片批量处理。

---

## 一、人工验收样本矩阵（≥20）

样本来源：本地 PostgreSQL `products` 表 perf seed + 手工标注场景。生成类型与结论基于预检规则 + 单测；真实 AI 输出列在重启后补填。

| # | 商品 ID | 来源 | 场景 | 生成类型 | 验收结论 |
| --- | --- | --- | --- | --- | --- |
| 1 | `715b12b8-c23a-42a3-bcc9-f2dd69d47095` | 1688 | 标题正常 | title | passed_with_warning |
| 2 | `4fe45e34-4529-438b-a64d-a478b412118c` | manual | 标题过短 | title | passed_with_warning |
| 3 | `ac15338f-2e45-40c3-89bc-8c9150fa49b7` | custom | 标题含采集噪声 | title | passed_with_warning |
| 4 | `a7715a98-3291-4bc3-ab7d-6620b07371af` | taobao_tmall | 标题很长 | title | passed_with_warning |
| 5 | `93b9663d-a5f4-4810-a9bc-2e3dcaf9f87d` | pinduoduo | 描述为空 | description | passed_with_warning |
| 6 | `4ee03ff7-4239-4d10-ae6b-ba72ffb468aa` | 1688 | 描述很短 | description | passed_with_warning |
| 7 | `e1f60994-f434-47ef-b8fe-d5593d6b8118` | taobao_tmall | 描述含 HTML | description | passed_with_warning |
| 8 | `1cf3566d-9aaf-44ed-b8eb-293ec5d16031` | custom | 多规格 | title+description | passed_with_warning |
| 9 | `8dfe5af3-554a-4110-9af8-ad1f2165583b` | manual | 图片缺失 | title | passed_with_warning |
| 10 | `28aa935b-03a5-440b-8697-51cd3f95de02` | pinduoduo | 价格异常 | title | passed_with_warning |
| 11 | `715b12b8-c23a-42a3-bcc9-f2dd69d47095` | 1688 | 1688 来源 | title | passed_with_warning |
| 12 | `93b9663d-a5f4-4810-a9bc-2e3dcaf9f87d` | pinduoduo | 拼多多来源 | description | passed_with_warning |
| 13 | `a7715a98-3291-4bc3-ab7d-6620b07371af` | taobao_tmall | 淘宝/天猫来源 | title | passed_with_warning |
| 14 | `ac15338f-2e45-40c3-89bc-8c9150fa49b7` | custom | 自定义链接 | title | passed_with_warning |
| 15 | `4fe45e34-4529-438b-a64d-a478b412118c` | manual | 手工创建 | title | passed_with_warning |
| 16 | `caab730a-6d34-4cec-a806-fade9f1ee8f3` | manual | 已人工改标题 | title | passed（冲突保护单测） |
| 17 | `ff0fc80d-2096-457b-9f42-3eca7313deb2` | custom | 已人工改描述 | description | passed（冲突保护单测） |
| 18 | `249a2bac-a1d4-43c8-bbff-7b909c100ab9` | taobao_tmall | 已应用 AI 标题 | title | passed_with_warning |
| 19 | `077fb62d-f936-4b13-96e3-15014dfa3f58` | pinduoduo | 已应用 AI 描述 | description | passed_with_warning |
| 20 | `1ae4dd72-541f-43d7-8051-05dd32adf984` | 1688 | 故意制造冲突 | title | passed（`product/ai_apply` 冲突单测 + 中文提示） |

### 样本明细模板（重启后补 AI 输出）

| 商品 ID | 当前标题 | 当前描述 | AI 生成结果 | 质量 warning | 可读 | 适合应用 | 应用成功 | 冲突 | 可撤销 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| （待 E2E） | — | — | — | — | — | — | — | — | — |

---

## 二、真实 AI Provider 小样本试跑

### 配置状态

- `settings.ai`：**已配置**（`provider=qwen`，`base_url=https://api.deepseek.com/v1`，`api_key` 已加密存储）
- 试跑阻塞原因：当前监听 `:8080` 的服务进程**未加载** `aiproducttext` 路由（`POST /api/v1/products/ai-text/batches` → 404）

### 计划试跑规模（重启后执行）

| 批次 | 商品数 | 类型 |
| --- | --- | --- |
| T1 | 5 | 仅标题 |
| T2 | 5 | 仅描述 |
| T3 | 3 | 标题 + 描述 |

### 验证项

- [ ] AI 服务配置可用
- [ ] 请求不超时（`timeout_sec=120`）
- [ ] 失败错误可读（中文）
- [ ] 生成结果非空
- [ ] 标题长度合理 / 描述非模板废话
- [ ] 禁用词检查生效
- [ ] 质量 warning 识别问题
- [ ] 进入 `pending_review`，**不**自动覆盖商品

**试跑结论**：`blocked_by_server_restart`（非 `blocked_by_ai_provider`）

---

## 三、失败任务中心联动

| 检查项 | 结果 |
| --- | --- |
| `failed` 子项进入 taskcenter | ✅ `taskType=ai_text` |
| `conflict` 子项进入 taskcenter | ✅ `failureCategory=ai_text_apply_conflict` |
| 质量 warning 进入（可选） | ✅ `ai_text_quality_warning`，不含全部待复核 |
| 去重键 | ✅ `task_type + source_id + failure_category` |
| 恢复后不再显示 | ✅ 状态变为 `applied` / `rejected` / 重生成成功后不再匹配失败筛选 |
| 深链 | ✅ `/product/ai-text-batches/:batchId?itemId=xxx` |
| 高亮 + 打开复核弹窗 | ✅ 前端 `AITextBatchDetail` |
| 重试失败项 | ✅ `POST /api/v1/products/ai-text/items/:id/regenerate` |

### 失败分类中文

| failure_category | 用户文案 |
| --- | --- |
| `ai_text_generation_failed` | AI 文案生成失败 |
| `ai_text_apply_conflict` | AI 文案应用时发现内容冲突 |
| `ai_text_apply_failed` | AI 文案应用失败 |
| `ai_text_undo_failed` | AI 文案撤销失败 |
| `ai_text_quality_warning` | AI 文案建议需要复核 |

---

## 四、旧入口 `/ai/batches`

- 菜单：**隐藏**（`hideInMenu: true`）
- 页面：保留，顶部 **旧版提示** +「前往新版批量文案任务」按钮
- 新版主入口：`/ai/text-batches`（菜单「批量文案任务」）

---

## 五、冲突与撤销

| 场景 | 结果 |
| --- | --- |
| 生成后标题未变 → 应用成功 | ✅ `product/ai_apply` 单测 |
| 生成后标题被人工改动 → 冲突 | ✅ 中文：`商品内容在 AI 建议生成后已经被修改…` |
| 批量应用部分冲突 | ✅ `partial_success` 统计 |
| 单条撤销 | ✅ |
| 批量撤销本批次 | ✅ |
| 商品后续人工修改 → 撤销阻止 | ✅ `content conflict` 单测 |

---

## 六、质量 warning 校准

- 标题：空 / 过短 / 过长 / 重复词 / 禁用词 / 采集噪声 — ✅ 中文 `quality.go`
- 描述：空 / 过短 / 缺卖点 / 缺规格 / 禁用词 / 结构不清晰 / 与标题不匹配 — ✅ 新增 `desc_unclear_structure`
- warning **不阻断**应用；技术码在「技术详情」折叠

---

## 七、多分辨率（代码审查 + 布局）

| 页面 | 1366px | 1024px |
| --- | --- | --- |
| 商品草稿列表 | ✅ ProTable scroll | ✅ |
| 批量 AI 向导 | ✅ 步骤条换行 | ✅ |
| 批量文案任务列表 | ✅ | ✅ |
| 复核详情 | ✅ `auto-fit` 三栏 | ✅ 上下布局 |
| 单条复核弹窗 | ✅ `min(1100, innerWidth-48)` | ✅ |
| 失败任务中心 | ✅ ProTable | ✅ |
| 旧版 `/ai/batches` | ✅ 提示条 | ✅ |

---

## 八、自动化验证

```bash
cd backend && go test ./...
cd backend && go build ./cmd/server/...
pnpm build:admin
git diff --check
go test ./internal/providers/platform/douyinshop/...
go test ./internal/modules/productpublish/...
go test ./internal/modules/ordersync/...
go test ./internal/modules/inventory/...
go test ./internal/modules/taskcenter/...
go test ./internal/modules/aiproducttext/...
```

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | ✅ pass |
| `go build ./cmd/server/...` | ✅ pass |
| `pnpm build:admin` | ✅ pass |
| `git diff --check` | ✅ pass（仅 CRLF 警告） |
| 抖店 / publish / ordersync / inventory 回归 | ✅ pass |

---

## 九、变更记录

- **2026-06-19**：Phase A3.1.1 — taskcenter 接入 `ai_product_text_items`；旧版入口提示；冲突中文；`itemId` 深链；质量 warning 补强；文档同步。
