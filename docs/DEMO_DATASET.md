# Demo 数据集说明（Phase F7）

> **Release 状态**：`MVP Demo Ready`（非 Production Ready）
> 生成脚本：`scripts/seed-demo-data.ps1` / `scripts/seed-demo-data.sh`
> 权限种子：`scripts/seed-demo-permissions.ps1`
> 机器可读输出：`docs/demo-dataset.json`、`docs/demo-dataset.full-project.json`

## 生成方式

```powershell
# 需本地 API 与管理员账号（读取 .env 中 ADMIN_BOOTSTRAP_*）
.\scripts\seed-demo-data.ps1 -ApiBase http://127.0.0.1:8080
.\scripts\seed-demo-permissions.ps1 -ApiBase http://127.0.0.1:8080
```

详见 [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md)。

脚本会：

1. 调用 `a1-prepare-samples.ps1` 补齐 A1.1 样本矩阵（20 slot）
2. 创建 R1 扩展 20 类商品场景（见下表）
3. 汇总已有 AI 文案/图片批次、失败任务、工作台待办
4. 可选创建本地刊登批次与 AI 文案种子批次

## 商品样本（20 类）

| # | tag | 说明 |
| --- | --- | --- |
| 1 | title_complete | 标题完整 |
| 2 | title_pending_optimize | 标题待优化 |
| 3 | description_empty | 描述为空 |
| 4 | description_pending | 描述待优化 |
| 5 | main_images_complete | 主图完整 |
| 6 | main_images_missing | 主图缺失 |
| 7 | detail_images_low | 详情图不足 |
| 8 | multi_sku | 多规格 |
| 9 | stock_unknown | 库存未知 |
| 10 | price_anomaly | 价格异常 |
| 11 | attributes_missing | 参数缺失 |
| 12 | publish_check_passed | 发布检查通过候选 |
| 13 | publish_check_warning | 发布检查 warning |
| 14 | publish_check_failed | 发布检查 failed |
| 15 | ai_text_pending_review | AI 文案待复核 |
| 16 | ai_image_pending_review | AI 图片待复核 |
| 17 | ai_conflict | AI 冲突 |
| 18 | local_publish_draft | 本地刊登草稿 |
| 19 | douyin_blocked_credentials | 抖店 blocked_by_real_credentials |
| 20 | multi_platform_targets | 多平台多店铺目标 |

完整 `productId` 见 `docs/demo-dataset.json` → `productSlots`。

## 任务样本

| 类型 | 期望状态 | 来源 |
| --- | --- | --- |
| AI 文案批次 | success / partial_success | 已有批次 + 可选种子 |
| AI 图片批次 | success / partial_success | 已有批次 |
| 批量刊登批次 | success / partial_success | 已有 + local_draft_only 种子 |
| 失败任务中心 | failure | 已有失败任务 |
| 商品运营工作台 | todos | 聚合待办 |

## 订单样本（Phase F2 / F7）

| tag | 说明 |
| --- | --- |
| normal_matched_order | 已匹配 SKU，可演示扣减 |
| unmatched_sku_order | 未匹配 SKU，异常工作台 |
| sync_partial_success | 订单同步 partial_success（需 shop / mock） |

明细见 `docs/demo-dataset.orders.json`。

## 库存样本（Phase F3 / F7）

| tag | 说明 |
| --- | --- |
| normal_stock_sku | 正常库存 |
| low_stock_sku | 低于预警线 |
| zero_stock_sku | 零库存 |
| deduct_success_order | 扣减成功路径 |
| deduct_blocked_unmatched_order | SKU 未匹配阻断扣减 |

明细见 `docs/demo-dataset.inventory.json`。

## 客服样本（Phase F4 / F7）

| tag | 说明 |
| --- | --- |
| pending_reply | 待回复会话 |
| ai_suggestion_generated | AI 建议已生成待确认 |
| send_failed | 发送失败（best-effort） |

明细见 `docs/demo-dataset.customer-service.json`。需配置 AI Provider 时 AI 建议步骤可能 skipped。

## Dashboard KPI 样本（Phase F6 / F7）

运行 seed 后探测 `GET /dashboard/overview|todos|health`，覆盖 10 KPI：采集失败、商品草稿、AI 待复核、发布检查、刊登异常、订单异常、库存预警、客服待回复、失败任务、配置风险。

明细见 `docs/demo-dataset.dashboard.json`。

## 注意事项

- 抖店真实 create-draft 仍为 **Release Candidate**，无凭证时样本 #19 预期 `blocked_by_real_credentials`
- 批量刊登 perf / 种子依赖 `local_draft_only` 平台店铺；脚本 `publish-batch-perf.ps1` 可自动创建 TikTok / Shopee / Lazada / Amazon demo 店铺
- 商品标题在种子脚本中使用 ASCII 前缀 `R1 demo`，便于检索；演示前可在后台改为中文展示名

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-30 | Phase F7：订单 / 库存 / 客服 / Dashboard 样本 + `demo-dataset.full-project.json` |
| 2026-06-27 | Phase R1.1 复跑：`seed-demo-data.ps1` + `demo-route-smoke.ps1`；12 步走查记录 |
| 2026-06-27 | Phase R1 初版：seed 脚本 + demo-dataset.json |
