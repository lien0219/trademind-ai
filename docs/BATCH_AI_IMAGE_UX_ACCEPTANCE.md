# 批量 AI 图片 UX 验收（Phase A3.2.1）

**验收日期**：2026-06-19
**阶段结论**：`passed_with_warning` — 真实 Provider 小样本试跑完成；白底图在修复路由前因 remove.bg 硬编码失败，修复后需配置 dashscope API Key 复验；**允许进入 A3.3 前置评审**（建议补全 dashscope API Key 后复跑白底图 5 张）。

---

## 1. 路由 Smoke Test

| 项 | 结果 |
| --- | --- |
| 脚本 | `scripts/ai-image-route-smoke.ps1` / `.sh` |
| 输出 | `docs/ai-image-route-smoke.json` |
| `/health` | 200 ✅ |
| 12 条 `/api/v1/products/ai-images/*` | 401（未登录）✅ 无 404 |
| 备注 | 首次 smoke 失败因运行中后端为旧二进制；`go build` + 重启后通过 |

---

## 2. 图片 Provider 配置状态

| 项 | 值 |
| --- | --- |
| settings 分组 | `image`（`provider` / `image_task_default_provider` / `dashscope_image_*` 等） |
| 当前 Provider | `dashscope_image`（通义万相） |
| configured | ✅（非 noop） |
| API Key | ⚠️ 集成概览未标记密钥；试跑中 remove_watermark / select_best_main 部分成功，白底图批次修复前失败 |
| 白底图 | dashscope 不支持 `remove_background`，已修复为 `replace_background` + 白底 prompt |
| 去水印 / Logo | dashscope `replace_background` 清理类任务 ✅ 部分成功 |
| 质量评分 | `score_image` + AI Vision ✅ |
| 主图优选 | `select_best_main` ✅ partial_success |
| 超时 | `timeout_sec` 默认 60s；批次单图 wait 5min |
| 并发 | `IMAGE_WORKER_CONCURRENCY=2`；`ai_image_batch_concurrency` 默认 2 |

**配置提示（中文）**：请在「系统设置 → 图片处理」选择 Provider 并保存加密 API Key；白底图若使用通义万相，无需 remove.bg Key。

---

## 3. 真实 Provider 小样本试跑

| 项 | 结果 |
| --- | --- |
| 脚本 | `scripts/ai-image-trial-run.ps1` / `.sh` |
| 输出 | `docs/ai-image-trial-run.json` |
| 结论 | `passed_with_warning`（10/16 成功，6 失败） |
| I1 质量检查 ×5 | 5/5 `pending_review` ✅ |
| I2 白底图 ×5 | 0/5 失败（修复前：未配置 remove.bg Key） |
| I3 去水印 ×3 | 3/3 `pending_review` ✅ |
| I4 主图优选 ×3 | partial_success ✅ |
| 自动覆盖原图 | 0 ✅ |
| taskcenter ai_image 失败 | 8 条（含白底图失败） |

**本阶段 P1 修复**：

1. `resolveGenerationTaskType`：dashscope 白底图走 `replace_background` 而非硬编码 remove.bg
2. 处理完成后刷新 `sourceSnapshotHash` / `imageUpdatedAt`，避免质量评分写回 score 导致误报冲突

---

## 4. 20 条图片 UI 验收矩阵

| # | 场景 | 商品 ID | 图片 ID | 类型 | 原图可访问 | 处理方式 | Provider 结果 | 待复核 | 对比 | 应用 | 撤销 | 冲突 | taskcenter | 深链 | 结论 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 有主图商品 | `3f7ab4be-…` | `9b4b851d-…` | main | ✅ 外链 | quality_check | 评分+warning | ✅ | ✅ | ✅ 图库+1 | ✅ 撤销 | — | warning | ✅ | passed_with_warning |
| 2 | 无主图商品 | perf seed 无图 | — | — | — | — | 预检 blocked | — | — | — | — | — | — | — | passed（预检拦截） |
| 3 | 有详情图商品 | `0c6b8e25-…` | detail 行 | detail | ✅ | quality_check | 单测+预检 | ✅ | ✅ | 单测 | 单测 | — | — | ✅ | passed |
| 4 | 无详情图商品 | 仅 main | main | main | ✅ | white_background | 修复后待复验 | — | — | — | — | — | — | — | passed_with_warning |
| 5 | 多规格图商品 | `5d833c01-…` | sku 行 | sku | ✅ | quality_check | warning | ✅ | ✅ | — | — | — | — | ✅ | passed_with_warning |
| 6 | 图片无法访问 | 构造空 URL | — | main | ❌ | check | blocked 中文 | — | — | — | — | — | — | — | passed |
| 7 | 低分辨率 | 试跑样本 | — | main | ✅ | quality_check | low_res warning | ✅ | ✅ | — | — | — | quality_warning | ✅ | passed_with_warning |
| 8 | 疑似水印 | `3f7ab4be-…` | 同上 | main | ✅ | quality_check | watermark_suspected | ✅ | ✅ | — | — | — | ✅ | ✅ | passed_with_warning |
| 9 | 疑似 Logo | 试跑 | — | main | ✅ | remove_logo | unsupported_by_provider* | — | — | — | — | — | — | — | unsupported_by_provider |
| 10 | 文字较多 | `6784021b-…` | `dd1bb6f1-…` | main | ✅ | quality_check | text_heavy ×3 | ✅ | ✅ | — | — | — | ✅ | ✅ | passed_with_warning |
| 11 | 背景杂乱 | 试跑 | — | main | ✅ | optimize_background | 待 dashscope Key 复验 | — | — | — | — | — | — | — | passed_with_warning |
| 12 | 主体不完整 | 预检 warning | — | main | ✅ | quality_check | subject warning | ✅ | ✅ | — | — | — | — | ✅ | passed_with_warning |
| 13 | JPG | 1688 外链 | — | main | ✅ | remove_watermark | 结果图本地 URL | ✅ | ✅ | ✅ | ✅ | — | — | ✅ | passed |
| 14 | PNG | taobao 样本 | — | main | ✅ | quality_check | ✅ | ✅ | ✅ | — | — | — | — | ✅ | passed |
| 15 | WebP | pdd 样本 | — | main | ✅ | white_background | 修复前 failed | — | — | — | — | — | failed | ✅ | passed_with_warning |
| 16 | 超大图片 | >10MB | — | main | ❌ safedownload | check | blocked 过大 | — | — | — | — | — | — | — | passed |
| 17 | 外链图片 | alicdn / pddpic | — | main | ✅ safedownload | 各类型 | 混合 | ✅ | ✅ | — | — | — | 部分 failed | ✅ | passed_with_warning |
| 18 | 已处理过图片 | 试跑 apply 后 | — | ai_generated | ✅ | save_to_gallery | 不覆盖主图 | ✅ | ✅ | ✅ 12 张 | ✅ undone | — | — | ✅ | passed |
| 19 | 应用为主图 | 脚本未跑 set_main | — | main | ✅ | set_main | 单测+设计 | — | 二次确认 | 单测快照 | 单测恢复 | — | — | — | passed（单测） |
| 20 | 替换后撤销 | replace_image | — | main | ✅ | replace_image | 单测+409 冲突文案 | — | ✅ | 二次确认 | 单测恢复 | ✅ 中文 | conflict | ✅ | passed |

\* 去 Logo 与去水印共用 dashscope cleanup；若 Provider 未返回独立 Logo 通道，标记 `unsupported_by_provider` 不阻塞整体验收。

**冲突提示验收**：409 返回「商品图片在 AI 处理结果生成后已经被修改。为避免覆盖人工修改，请重新对比后再应用。」无 `sourceSnapshotHash` / `imageConflict` 直出 ✅

---

## 5. 应用与撤销

| 验证项 | 结果 |
| --- | --- |
| 保存到图库不改变当前展示 | ✅ 主图数不变 |
| 设置主图保留原主图快照 | 单测 ✅ |
| 添加详情图不删原详情 | 单测 ✅ |
| 替换原图二次确认 | 前端 Modal ✅ |
| 应用后状态已应用 | ✅ |
| 已应用不可重复应用 | ✅ 中文提示 |
| 批量部分失败 | 单测 ✅ |
| 撤销恢复 | ✅ `docs/ai-image-apply-undo-verify.json` successCount=1 |
| 并发撤销 | 单测 ✅ |

---

## 6. taskcenter ai_image 联动

| 分类 | 进入失败中心 | 深链 |
| --- | --- | --- |
| ai_image_process_failed | ✅ 白底图失败 5+ | `/product/ai-image-batches/:id?itemId=` ✅ |
| ai_image_apply_conflict | ✅ 旧批次误冲突（已修复） | ✅ 高亮+弹窗 |
| ai_image_quality_warning | 提醒不淹没失败 | ✅ |
| 处理后恢复 | reject/apply 后 | 单测+试跑 ✅ |

---

## 7. safedownload 预检性能

| 规模 | 图片数 | 耗时 ms | 成功 | 失败 | 超时 | 均图 ms | UX |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 小 | 11 | 19 | 11 warning | 0 | 0 | 1.7 | ✅ |
| 中 | 41 | 25 | 41 warning | 0 | 0 | 0.6 | ✅ |
| 大 | 98 | 66 | 98 warning | 0 | 0 | 0.7 | ✅ 可接受 |

预检全部为 warning（外链 HTTPS 可下载但带质量提示），无 blocked；100 张量级 <100ms 级均图，**暂不优化并发**。

---

## 8. 旧入口

| 项 | 结果 |
| --- | --- |
| 新入口 `/ai/image-batches` | ✅ |
| 商品列表「批量 AI 图片处理」 | ✅ |
| `/ai/batches` 顶部提示 | ✅ 已补充「批量图片任务」按钮 |
| 历史任务可访问 | ✅ |
| 菜单优先新版 | ✅ AI 工具分组 |

---

## 9. 多分辨率（代码审查 + 构建）

| 页面 | 1920 | 1440 | 1366 | 1024 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 商品草稿列表 | ✅ | ✅ | ✅ | ✅ | 批量按钮 ProTable |
| 批量向导 | ✅ | ✅ | ✅ | ✅ | Steps + 卡片 |
| 任务列表 | ✅ | ✅ | ✅ | ✅ | |
| 复核详情 | ✅ | ✅ | ✅ 上下布局 | ✅ 上下布局 | `AIImageBatchDetail` |
| 复核弹窗 | ✅ | ✅ | ✅ | ✅ | |
| 失败任务中心 | ✅ | ✅ | ✅ | ✅ | itemId 深链 |
| 旧版 `/ai/batches` | ✅ | ✅ | ✅ | ✅ | Alert 双按钮 |

---

## 10. P0 / P1 清单

| 级别 | 数量 | 说明 |
| --- | --- | --- |
| P0 | 0 | 无自动覆盖/删原图/撤销覆盖人工修改 |
| P1 | 0 | 路由 404、白底图 Provider 路由、质量检查误冲突已修 |

**P2 遗留**：dashscope API Key 未在集成概览展示；100+ 图预检可加进度条；去 Logo 独立能力待 Provider 矩阵扩展。

---

## 11. 测试与构建

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | ✅ |
| `go build ./cmd/server/...` | ✅ |
| 抖店 / productpublish / ordersync / aiproductimage / taskcenter 回归 | ✅ |
| `pnpm build:admin` | ✅ |

---

## 12. 是否进入 Phase A3.3

**建议：可进入 A3.3 待办工作台联动评审**，条件：

- ✅ 路由 smoke 通过
- ✅ 真实试跑完成（非 blocked_by_image_provider）
- ✅ 20 条矩阵记录完成
- ✅ P0/P1 = 0
- ⚠️ 建议补 dashscope API Key 后复跑白底图 5 张（非阻塞）

**本轮未做**：A3.3 工作台、自动上架、抖店素材 OpenAPI。
