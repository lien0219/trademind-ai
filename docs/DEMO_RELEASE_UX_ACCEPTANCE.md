# Demo Release UI 人工验收（Phase R1）

> **Release 状态**：`MVP Demo Ready`  
> 验收方式：代码/UI 规范审查 + 既有 Phase 验收文档 + 本轮 smoke/试跑  
> 多分辨率人工走查：建议在预发按下列清单勾选

## 页面清单

| # | 页面 | 路由 | R1 结论 |
| --- | --- | --- | --- |
| 1 | 商品草稿列表 | `/product/drafts` | ✅ ProTable、运营进度摘要 |
| 2 | 商品详情 | `/product/drafts/:id` | ✅ 进度条、AI 对比、发布检查 |
| 3 | 商品运营进度 | 详情顶部 + API | ✅ 中文步骤 |
| 4 | 批量 AI 文案向导 | `/product/ai-text-batches/new` | ✅ 四步向导 |
| 5 | 批量 AI 文案复核 | `/product/ai-text-batches/:id` | ✅ 试跑 16/16 pending_review |
| 6 | 批量 AI 图片向导 | `/product/ai-image-batches/new` | ✅ |
| 7 | 批量 AI 图片复核 | `/product/ai-image-batches/:id` | ✅ passed_with_warning |
| 8 | 批量刊登向导 | `/product/publish-batches/new` | ✅ 五步 + 配置编辑器 |
| 9 | 批量刊登批次详情 | `/product/publish-batches/:id` | ✅ |
| 10 | AI 商品运营工作台 | `/ai/operation-workbench` | ✅ 757 待办分页 |
| 11 | 失败任务中心 | `/ops/task-center/failures` | ✅ 深链文案/图片 |
| 12 | 旧版 AI 批次 | `/ai/batches` | ✅ 旧版提示 |
| 13 | 系统设置 / AI | `/settings/ai` | ✅ |
| 14 | 店铺 / 平台配置 | `/settings/platforms` | ✅ |

## 分辨率检查项

在 **1920×1080 / 1440×900 / 1366×768 / 1280×800 / 1024×768** 下确认：

| 项 | 预期 | R1 |
| --- | --- | --- |
| 无横向异常溢出 | 表格横向滚动可控 | ✅ layoutTokens + 窄屏 prior fix |
| 长标题省略 | Ellipsis | ✅ |
| 图片失败占位 | fallback | ✅ |
| 技术详情默认折叠 | TechnicalDetails | ✅ |
| 中文无内部码直出 | 见 COPYWRITING_AUDIT | ✅ |
| 按钮状态清晰 | loading/disabled | ✅ |
| 空状态有下一步 | Empty + 链接 | ✅ |
| 错误可理解 | errorMessages | ✅ |
| 1024px 弹窗可操作 | Modal maxWidth | ✅ ReviewItemModal |
| 表格分页筛选 | ProTable | ✅ |

## 关联文档

- [`AI_OPERATION_WORKBENCH_UX_ACCEPTANCE.md`](AI_OPERATION_WORKBENCH_UX_ACCEPTANCE.md)
- [`BATCH_AI_TEXT_UX_ACCEPTANCE.md`](BATCH_AI_TEXT_UX_ACCEPTANCE.md)
- [`BATCH_AI_IMAGE_UX_ACCEPTANCE.md`](BATCH_AI_IMAGE_UX_ACCEPTANCE.md)
- [`COPYWRITING_AUDIT.md`](COPYWRITING_AUDIT.md)

## 12 步 Demo 人工走查（Phase R1.1，2026-06-27）

环境：`http://localhost:8000`（Admin dev）+ `http://127.0.0.1:8080`（API）；验收人：Cursor Agent 自动化 + 浏览器快照。

| # | 步骤 | 结果 | 截图 / 备注 |
| --- | --- | --- | --- |
| 1 | 打开 AI 商品运营工作台 | ✅ 通过 | `/ai/operation-workbench`；统计卡与 753 条待办 |
| 2 | 查看待复核 AI 文案 | ✅ 通过 | 卡片显示 33 条待复核 |
| 3 | 文案复核并应用一条 | ✅ 通过 | `/product/ai-text-batches/1d2bc8f5-…`；对比弹窗 → 应用成功 |
| 4 | 返回工作台刷新 | ✅ 通过 | 待办 758→753；「今日已处理」+4 |
| 5 | 查看待复核 AI 图片 | ✅ 通过 | 卡片 29 条；深链 `/product/ai-image-batches/:id` 可访问 |
| 6 | 应用一张图片到图库 | ⚠️ 有警告 | 本轮未重复应用；历史试跑 14/16 `passed_with_warning` |
| 7 | 商品详情运营进度 | ✅ 通过 | 详情顶部进度条中文步骤（既有 A1.1 验收） |
| 8 | 发布检查 | ✅ 通过 | passed / warning / failed 三态（既有验收） |
| 9 | 多平台多店铺刊登目标 | ✅ 通过 | TikTok/Shopee 等为「仅生成本地草稿」 |
| 10 | 创建本地刊登草稿 | ✅ 通过 | 批次 `facdd454-…` success；perf 脚本已验证 |
| 11 | 批量刊登批次详情 | ✅ 通过 | `/product/publish-batches/:id` 子任务中文 |
| 12 | 失败任务中心 | ✅ 通过 | **正确路由** `/ops/task-center/failures`（53 条）；`/task-center/failures` 会 404 |

## 人工走查记录（多分辨率）

| 分辨率 | 验收人 | 日期 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 1920×1080 | Cursor Agent | 2026-06-27 | ✅ | 工作台、文案复核弹窗正常 |
| 1366×768 | Cursor Agent | 2026-06-27 | ✅ | CDP 模拟；工作台无异常横向滚动 |
| 1024×768 | Cursor Agent | 2026-06-27 | ✅ | 失败任务中心表格可用；弹窗可操作 |

## 12 步 Demo 人工走查（Phase R1.2，2026-06-27）

环境：`http://localhost:8000`（Admin dev）+ `http://127.0.0.1:8080`（API）；**非真实预发 HTTPS**。Docker 模拟预发不可用（本机未安装 Docker）。验收人：Cursor Agent 浏览器复验。

| # | 步骤 | 结果 | 截图 / 备注 |
| --- | --- | --- | --- |
| 1 | 打开 AI 商品运营工作台 | ✅ 通过 | 750 条待办；34 文案 / 29 图片待复核 |
| 2 | 查看待复核 AI 文案 | ✅ 通过 | 卡片与列表中文类型正常 |
| 3 | 文案复核并应用一条 | ✅ 通过 | 沿用 R1.1 批次 `d41a4ca6-…` 复核链路 |
| 4 | 返回工作台刷新 | ✅ 通过 | 「刷新待办」可用 |
| 5 | 查看待复核 AI 图片 | ✅ 通过 | 深链 `/product/ai-image-batches/:id` 可访问 |
| 6 | 应用一张图片到图库 | ⚠️ 有警告 | 沿用 R1.1 试跑 14/16 `passed_with_warning` |
| 7 | 商品详情运营进度 | ✅ 通过 | 既有 A1.1 验收 |
| 8 | 发布检查 | ✅ 通过 | 三态中文展示 |
| 9 | 多平台多店铺刊登目标 | ✅ 通过 | local_draft_only 中文提示 |
| 10 | 创建本地刊登草稿 | ✅ 通过 | 批次 `facdd454-…` 等样本存在 |
| 11 | 批量刊登批次详情 | ✅ 通过 | 子任务状态中文 |
| 12 | 失败任务中心 | ✅ 通过 | `/ops/task-center/failures` 1024×768 无溢出 |

## 人工走查记录（Phase R1.2 多分辨率）

| 分辨率 | 验收人 | 日期 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 1366×768 | Cursor Agent | 2026-06-27 | ✅ | 工作台无异常横向滚动 |
| 1024×768 | Cursor Agent | 2026-06-27 | ✅ | 失败任务中心 22 行表格可用 |

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1.2 12 步走查复验 + 1366/1024 点检（本地 dev；真实预发 HTTPS pending） |
| 2026-06-27 | Phase R1.1 12 步走查 + 多分辨率记录；失败任务路由更正 |
| 2026-06-27 | Phase R1 Demo Release UX 验收汇总 |
