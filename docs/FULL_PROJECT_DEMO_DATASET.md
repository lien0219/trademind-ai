# 全项目 Demo 数据集（Phase F7）

> **状态**：F7 完成 · MVP Demo Ready · 非 Production Ready · 抖店 Release Candidate · Tag pending

## 脚本

| 脚本 | 说明 |
| --- | --- |
| `scripts/seed-demo-data.ps1` / `.sh` | 商品 / 订单 / 库存 / 客服 / Dashboard 主链路 |
| `scripts/seed-demo-permissions.ps1` | demo_admin / demo_operator / demo_readonly + 店铺授权 |

## F7 全链路样本

- **商品**：20 slot（A1.1 + R1 场景）+ AI / 刊登 / 失败任务
- **订单**（F2）：正常 / 未匹配 SKU / partial_success 探测
- **库存**（F3）：正常 / 低库存 / 零库存 / 扣减成功与阻断
- **客服**（F4）：待回复 / AI 建议待确认 / 发送失败（best-effort）
- **Dashboard**（F6/F7）：10 KPI 聚合（采集失败、订单异常、库存预警、客服待回复、配置风险等）
- **权限**：平台未授权 / 店铺隔离 / readonly 写阻断

## 输出

- `docs/demo-dataset.json` — 商品 / AI / 刊登
- `docs/demo-dataset.orders.json` — 订单
- `docs/demo-dataset.inventory.json` — 库存
- `docs/demo-dataset.customer-service.json` — 客服
- `docs/demo-dataset.dashboard.json` — Dashboard KPI 探测
- `docs/demo-dataset.permissions.json` — 权限
- `docs/demo-dataset.full-project.json` — F7 全链路索引（运行种子后生成）

## 账号

见 `docs/demo-dataset.permissions.json`：`demo_admin@trademind.local` / `demo_operator@trademind.local` / `demo_readonly@trademind.local`。

## 相关

- [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md) — 种子步骤与前置条件
- [`DEMO_DATASET.md`](DEMO_DATASET.md) — 商品 slot 明细
