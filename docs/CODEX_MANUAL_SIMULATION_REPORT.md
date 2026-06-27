# TradeMind Phase R1.3-Codex 模拟人工测试与体验问题扫描

## 1. 测试环境

| 项 | 结果 |
| --- | --- |
| 日期 | 2026-06-27 |
| 分支 | `dev` |
| 项目状态 | `MVP Demo Ready`，非 Production Ready |
| 检查方式 | 代码路由走查 + 核心页面实现检查 + 既有 R1/R1.2 验收报告复核 |
| 未执行项 | 未进入抖店真实 E2E；未打 `v0.1.0-demo` tag；未做生产灰度 |
| 浏览器截图 | `manual_required` |

## 2. 模拟角色

| 角色 | 覆盖结果 |
| --- | --- |
| 新手运营 | 检查工作台入口、中文说明、旧入口提示、复核页主按钮与错误恢复说明 |
| 熟练运营 | 检查批量文案、批量图片、批量刊登、部分成功、重试失败项与批次详情 |
| 管理员 | 检查失败任务中心、Provider / Storage / 抖店凭证阻断提示、技术详情折叠和敏感信息暴露 |

## 3. 走查链路

主链路 24 步按 `/ai/operation-workbench` 起步做静态模拟。工作台统计卡、待办列表、详情抽屉、AI 文案复核、AI 图片复核、商品详情、发布检查、批量刊登、刊登批次详情、失败任务中心均有明确路由和页面实现。

实测限制：本轮未启动浏览器逐步点击，涉及弹窗视觉、浏览器后退、刷新保留状态和多分辨率截图的项目标记为 `manual_required`。

## 4. 页面覆盖清单

| 页面 | 结果 | 备注 |
| --- | --- | --- |
| `/ai/operation-workbench` | passed | 统计、筛选、待办、详情抽屉、操作入口清楚 |
| `/product/drafts` | passed_with_warning | 新版批量入口清楚；旧版批量 AI 已补充旧版提示 |
| `/product/ai-text-batch` | passed | 四步批量文案向导入口明确 |
| `/product/ai-text-batches/:id` | passed | 支持 `?itemId=` 定位并打开复核 |
| `/ai/text-batches` | passed | 列表可进入新版复核 |
| `/product/ai-image-batch` | passed | 批量图片向导入口明确 |
| `/product/ai-image-batches/:id` | passed | 支持 `?itemId=` 定位并打开对比 |
| `/ai/image-batches` | passed | 列表可进入新版复核 |
| `/product/publish-batch` | passed | local draft only 与检查创建流程存在 |
| `/product/publish-batches/:id` | passed | 部分成功提示、重试失败项、失败任务中心入口存在 |
| `/ops/task-center/failures` | passed | 失败详情、重试、标记、深链跳转可用 |
| `/ai/batches` | passed_with_warning | 旧版页保留；已支持 `?id=` 打开详情并显示新版入口提示 |
| `/products/:id` | not_applicable | 实际路由为 `/product/drafts/:id` |
| `/product/drafts/:id` | passed | 商品详情、发布检查、AI 应用/撤销入口存在 |

## 5. 主链路结果

| 步骤范围 | 结果 | 说明 |
| --- | --- | --- |
| 1-4 登录与工作台 | passed_with_warning | 登录需人工环境；工作台入口、5 张统计卡、待办列表代码存在 |
| 5-10 AI 文案复核与应用 | passed | 复核页、对比弹窗、应用、冲突保护、工作台刷新入口存在 |
| 11-16 AI 图片复核与应用 | passed | 原图/结果图对比、应用到图库、撤销入口存在 |
| 17-18 发布检查 | passed | 商品详情 readiness / publish check 深链存在 |
| 19-22 批量刊登 | passed | local draft only、创建草稿、批次详情、部分成功和重试入口存在 |
| 23-24 失败任务中心跳转 | passed | 失败中心识别复核工作台和刊登批次详情链接 |

## 6. 异常流程结果

| 异常 | 结果 | 说明 |
| --- | --- | --- |
| AI 文案生成失败 | passed | 批次详情支持失败项重试 |
| AI 文案应用冲突 | passed | 应用前检查商品内容变化，冲突不覆盖 |
| AI 文案撤销冲突 | passed | 批量撤销提示人工修改会失败 |
| AI 图片处理失败 | passed | 批次详情支持失败项重试 |
| AI 图片应用冲突 | passed | 批量应用返回成功 / 冲突 / 失败统计 |
| AI 图片撤销冲突 | passed | 批量撤销提示人工修改会失败 |
| 批量刊登 partial_success | passed | 显示“部分子任务失败”并提供重试失败项 |
| local_draft_only 平台 | passed | 显示“仅生成本地草稿”，不调用外部平台 API |
| 抖店凭证未配置 | passed | 仍为 Release Candidate，真实 E2E 阻断不误报为发布成功 |
| Storage public access 未配置 | passed_with_warning | 已有设置页与预检；真实公网需人工 |
| 图片 Provider Key 缺失 | passed_with_warning | 配置类问题可定位到设置页，需真实环境复核 |
| 发布检查 failed | passed | 商品详情与工作台待办可跳转处理 |
| 店铺未授权 | passed | 中文提示与店铺管理入口存在 |
| 直接访问旧版 `/ai/batches` | passed_with_warning | 已保留旧版提示并修复 `?id=` 定位 |

## 7. 状态一致性结果

| 模块 | 结果 | 说明 |
| --- | --- | --- |
| AI 文案 | passed_with_warning | 列表、详情、工作台、失败中心使用同一批次/子项状态；数量需浏览器联调再确认 |
| AI 图片 | passed_with_warning | 列表、详情、工作台、失败中心深链一致；图库变化需真实点击复核 |
| 批量刊登 | passed | 批次状态、子任务状态、失败中心、重试失败项链路一致 |
| 发布检查 | passed_with_warning | 商品详情与工作台入口一致；滚动定位需人工复核 |

## 8. 文案问题

| 等级 | 问题 | 处理 |
| --- | --- | --- |
| P1 | 商品运营看板默认快捷入口“批量 AI 优化”指向旧版 `/ai/batches`，新手可能误入旧流程 | 已修复：改为“批量文案任务”并跳转 `/ai/text-batches` |
| P2 | 商品草稿页旧版“批量 AI”按钮与新版批量入口并列，容易误解 | 已修复：标为“旧版批量 AI”，说明建议优先使用新版 |
| P2 | 旧版批量 AI 应用策略提示直出 `ai_title / ai_description` | 已修复：改为“AI 优化标题 / AI 优化描述” |

## 9. 跳转问题

| 等级 | 问题 | 处理 |
| --- | --- | --- |
| P1 | `/ai/batches?id=:id` 不会自动定位目标批次详情 | 已修复：旧版页读取 `id` 查询参数并打开批次详情 |
| P1 | 商品运营看板快捷入口仍指向旧版 AI 批次 | 已修复：改指向新版 `/ai/text-batches` |

## 10. 分辨率问题

| 分辨率 | 结果 |
| --- | --- |
| 1920x1080 | manual_required |
| 1440x900 | manual_required |
| 1366x768 | manual_required |
| 1280x800 | manual_required |
| 1024x768 | manual_required |

说明：本轮没有自动截图工具链输出，因此不伪造通过。已有 R1.1/R1.2 报告覆盖过 1366 / 1024，但 R1.3 修复后仍建议人工或 Playwright 复验。

## 11. P0 问题列表

无。

## 12. P1 问题列表

| 编号 | 问题 | 状态 |
| --- | --- | --- |
| R13-P1-001 | 商品运营看板快捷入口误导到旧版 AI 批次页 | fixed |
| R13-P1-002 | 旧版 AI 批次 `?id=` 深链不能定位目标批次 | fixed |
| R13-P1-003 | `TechnicalDetails` 已被调用方传入 `style`，组件 props 未声明，存在 Admin build 类型风险 | fixed |

## 13. P2 问题列表

| 编号 | 问题 | 状态 |
| --- | --- | --- |
| R13-P2-001 | 商品草稿页旧版批量 AI 入口文案不够明确 | fixed |
| R13-P2-002 | 旧版批量 AI 应用策略提示出现内部字段名 | fixed |
| R13-P2-003 | 多分辨率真实截图未在本轮自动完成 | manual_required |

## 14. 已修复项

- 运营看板快捷入口改为新版“批量文案任务”。
- 旧版 `/ai/batches?id=:id` 支持自动打开批次详情。
- 商品草稿页旧版批量 AI 入口和说明改为明确的旧版兼容文案。
- `TechnicalDetails` 增加 `style` props，保留默认折叠。

## 15. 未修复项

- 多分辨率浏览器截图和刷新 / 后退真实交互未在本轮自动执行。
- 抖店真实 E2E、Storage 公网访问、真实预发 HTTPS 仍需人工环境。

## 16. 仍需人工确认项

- 真实浏览器按 24 步主链路点击一遍。
- 1920 / 1440 / 1366 / 1280 / 1024 分辨率截图检查。
- 真实 Provider / Storage / 抖店凭证环境下的阻断提示复核。

## 17. 最终结论

`codex_simulation_passed_with_warning`

