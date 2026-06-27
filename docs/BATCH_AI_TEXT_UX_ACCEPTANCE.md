# Phase A3.1.2 批量 AI 文案 UX 验收报告

> 日期：2026-06-19  
> 阶段：**AI 商品运营体验 Phase A3.1.2** — 路由部署验收、真实 Provider 试跑、20 条 UI 矩阵、P0/P1 修复

## 总体验收结论

| 维度 | 结论 | 说明 |
| --- | --- | --- |
| 404 / 旧进程 | **passed** | 停止旧 `go run` 进程，以 `tmp/server.exe`（含 `aiproducttext` 路由）重启 `:8080` |
| 路由 smoke test | **passed** | `scripts/ai-text-route-smoke.ps1` / `.sh`；13 条路由均非 404（未登录 401） |
| 真实 AI Provider 试跑 | **passed** | Qwen Provider；16 子项（5 标题 + 5 描述 + 6 标题+描述）全部 `pending_review`；0 失败 |
| 20 条人工 E2E 矩阵 | **passed_with_warning** | 真实 AI 输出已回填；10 条含质量 warning（描述结构/规格类，不阻断） |
| 失败任务中心联动 | **passed** | `ai_text` 聚合 quality warning；应用/放弃后计数下降 |
| 冲突 / 撤销 | **passed** | 单测 + 中文提示；试跑未自动覆盖商品 |
| 旧入口梳理 | **passed** | `/ai/batches` 隐藏菜单 + 旧版提示 + 跳转新版 |
| P0 | **0** | 无自动覆盖、无冲突失效、无错误深链 |
| P1 | **0** | 修复异步 context 取消导致批次卡 `pending`；`retry-failed` 支持 orphaned pending |

**是否允许进入 Phase A3.2（批量图片）**：**否** — A3.1.2 文案试跑与验收已完成，图片批量仍属下一阶段。

---

## 一、404 / 旧进程处理

| 项 | 结果 |
| --- | --- |
| 现象 | A3.1.1 阶段 `:8080` 为旧 `go-build` 二进制或未含 `aiproducttext` 的进程，`/api/v1/products/ai-text/*` 曾返回 404 |
| 处理 | 停止 PID 旧进程 → `go build -o tmp/server.exe ./cmd/server/` → 启动新二进制（`TRADEMIND_REPO_ROOT` 指向仓库根） |
| 启动命令 | `Start-Process tmp/server.exe -WorkingDirectory backend`（本地验收）；日常开发仍可用 `pnpm dev:backend` |
| 验证 | 未登录 GET/POST 均返回 **401**（非 404）；health `200` |

---

## 二、路由 smoke test

脚本：`scripts/ai-text-route-smoke.ps1`、`scripts/ai-text-route-smoke.sh`  
结果文件：`docs/ai-text-route-smoke.json`

| 检查 | 结果 |
| --- | --- |
| `/health` | 200 |
| 12 条 `/api/v1/products/ai-text/*` | 401（路由已注册） |
| `failed404Count` | 0 |
| health timestamp | 2026-06-19T13:11:04Z |

---

## 三、真实 AI Provider 小样本试跑

脚本：`scripts/ai-text-trial-run.ps1`  
结果文件：`docs/ai-text-trial-run.json`

### 配置

- Provider：**qwen**（`settings.ai` 已加密配置）
- 连通性：`POST /api/v1/settings/test-ai` → ok，latency ~2s

### 试跑批次

| 批次 | batchNo | 商品数 | 类型 | 子项 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T1 | AT202606190005 | 5 | 仅标题 | 5 | success |
| T2 | AT202606190006 | 5 | 仅描述 | 5 | success |
| T3 | AT202606190007 | 3 | 标题+描述 | 6 | success |

**合计**：13 商品 × 16 子项（operation 维度）；**16/16** 进入 `pending_review`，生成文本非空，**0** `autoOverwrite`。

### 验证项

- [x] AI 服务配置可用
- [x] 请求不 404、不超时（单批 ~10–65s）
- [x] 失败错误可读（本轮 0 失败）
- [x] 生成结果非空
- [x] 标题长度合理（21–37 字）
- [x] 描述非空模板（115–141 字，含卖点语义）
- [x] 质量 warning 识别（10/16 有 warning）
- [x] 进入 `pending_review`，**不**自动覆盖商品
- [x] taskcenter 展示 `ai_text_quality_warning`（试跑后 11 条，含 warning 子项）
- [x] 应用 1 条 / 放弃 1 条后 taskcenter 计数下降（11→10）

**试跑结论**：**passed**

### P1 修复（试跑暴露）

1. **异步 generation 使用 HTTP request context**：`CreateBatch` 返回后 context 取消，子项永久 `pending`。修复：`detachedGinContext(c)` 使用 `context.Background()`。
2. **`retry-failed` 仅重试 failed**：服务重启后 pending 孤儿无法恢复。修复：同时重试 `pending` / `running` / `failed`。

---

## 四、人工验收样本矩阵（20 条）

样本来源：perf seed + A3.1.2 真实试跑 batch `35a1845a` / `4711e67e` / `fa54794c`。

| # | 商品 ID | 来源 | 生成类型 | batch / itemId | AI 可用 | warning | 待复核 | 三栏 | 编辑应用 | 冲突 | 撤销 | taskcenter | 深链 | 结论 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `715b12b8-…7095` | 1688 | title | T1 / `efb0d129-…` | ✅ | 采集噪声 | ✅ | ✅ | ✅ | 单测 | 单测 | warning 展示 | ✅ | passed_with_warning |
| 2 | `4fe45e34-…118c` | manual | title | T1 / `58a86a88-…` | ✅ | 无 | ✅→applied | ✅ | ✅ 已应用 | 单测 | 单测 | 恢复 | ✅ | passed |
| 3 | `ac15338f-…49b7` | custom | title | T1 / `91b2730f-…` | ✅ | 无 | ✅ | ✅ | ✅ | 单测 | 单测 | — | ✅ | passed |
| 4 | `a7715a98-…71af` | taobao_tmall | title | T1 / `dbb08d45-…` | ✅ | 无 | ✅ | ✅ | ✅ | 单测 | 单测 | — | ✅ | passed |
| 5 | `8dfe5af3-…583b` | manual | title | T1 / `8d3bc31b-…` | ✅ | 无 | ✅ | ✅ | ✅ | 单测 | 单测 | — | ✅ | passed |
| 6 | `93b9663d-…f87d` | pinduoduo | description | T2 / `92e7dbd7-…` | ✅ | 结构/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 7 | `4ee03ff7-…68aa` | 1688 | description | T2 / `4e050abd-…` | ✅ | 结构/卖点/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 8 | `e1f60994-…8118` | taobao_tmall | description | T2 / `bdc05cbe-…` | ✅ | 结构/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 9 | `28aa935b-…de02` | pinduoduo | description | T2 / `46b4ffaa-…` | ✅ | 结构/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 10 | `077fb62d-…3f58` | pinduoduo | description | T2 / `c3c90fca-…` | ✅ | 结构/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 11 | `1cf3566d-…6031` | custom | title+desc | T3 / `68eaa4bb-…` + `b015f810-…` | ✅ | desc 结构 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 12 | `249a2bac-…ab9` | taobao_tmall | title+desc | T3 / `288892e6-…` + `6f4d47bb-…` | ✅ | desc 结构/规格 | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 13 | `1ae4dd72-…f984` | 1688 | title+desc | T3 / `238ccc04-…` + `f247112c-…` | ✅ | title 噪声 + desc | ✅ | ✅ | ✅ | 单测 | 单测 | warning | ✅ | passed_with_warning |
| 14 | `715b12b8-…7095` | 1688 | title | 预检场景 | — | 规则 | — | ✅ | — | — | — | — | ✅ | passed_with_warning |
| 15 | `caab730a-…e8f3` | manual | title | 冲突样本 | — | — | — | ✅ | 冲突单测 | ✅ | ✅ | conflict | ✅ | passed |
| 16 | `ff0fc80d-…deb2` | custom | description | 冲突样本 | — | — | — | ✅ | 冲突单测 | ✅ | ✅ | conflict | ✅ | passed |
| 17 | `249a2bac-…ab9` | taobao_tmall | title | 已应用样本 | ✅ | — | — | ✅ | — | — | — | — | ✅ | passed_with_warning |
| 18 | `077fb62d-…3f58` | pinduoduo | description | 已应用样本 | ✅ | — | — | ✅ | — | — | — | — | ✅ | passed_with_warning |
| 19 | `efb0d129-…` item | — | title | 放弃样本 | ✅ | 噪声 | rejected | ✅ | — | — | — | 恢复 -1 | ✅ | passed |
| 20 | `1ae4dd72-…f984` | 1688 | title | 故意冲突 | — | — | — | ✅ | 冲突单测 | ✅ | 中文提示 | conflict | ✅ | passed |

深链格式：`/product/ai-text-batches/:batchId?itemId=:itemId`

---

## 五、失败任务中心联动

| 检查项 | 结果 |
| --- | --- |
| `failed` / `conflict` 进入 taskcenter | ✅ |
| 质量 `ai_text_quality_warning` | ✅ 试跑后 11 条 |
| 去重键 | ✅ |
| 应用成功后恢复 | ✅ 试跑 apply 后仍剩 warning 项 |
| 放弃建议后恢复 | ✅ reject 后 11→10 |
| 深链 + 高亮 + 复核弹窗 | ✅ A3.1.1 已实现 |

---

## 六、旧入口 `/ai/batches`

- 菜单：**隐藏**（`hideInMenu: true`）
- 主入口：`/ai/text-batches`（菜单「批量文案任务」）
- 旧页提示：「这是旧版批量 AI 任务入口…」+ 按钮「前往新版批量文案任务」

---

## 七、多分辨率（代码审查 + 试跑 UI 抽检）

| 页面 | 1920 | 1440 | 1366 | 1280 | 1024 |
| --- | --- | --- | --- | --- | --- |
| 商品草稿列表 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 批量 AI 向导 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 批量文案任务列表 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 复核详情三栏 | ✅ | ✅ | ✅ 上下布局 | ✅ | ✅ |
| 单条复核弹窗 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 失败任务中心 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 旧版 `/ai/batches` | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## 八、自动化验证

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | ✅ pass |
| `go build ./cmd/server/...` | ✅ pass |
| `pnpm build:admin` | ✅ pass |
| `git diff --check` | ✅ pass（CRLF 警告） |
| 抖店 / publish / ordersync / inventory / aiproducttext / taskcenter 回归 | ✅ pass |
| `scripts/ai-text-route-smoke.ps1` | ✅ pass |
| `scripts/ai-text-trial-run.ps1` | ✅ passed (16/16) |

---

## 九、变更记录

- **2026-06-19**：Phase A3.1.2 — 路由 smoke 脚本；真实 Qwen 试跑 16 子项；修复 async context + retry orphaned pending；验收文档同步。
- **2026-06-19**：Phase A3.1.1 — taskcenter 接入；旧版入口；冲突中文；itemId 深链。
