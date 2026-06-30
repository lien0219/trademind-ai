# 全项目 Demo 数据集（Phase F6）

## 脚本

| 脚本 | 说明 |
| --- | --- |
| `scripts/seed-demo-data.ps1` / `.sh` | 商品 / 订单 / 库存 / 客服主链路 |
| `scripts/seed-demo-permissions.ps1` | demo_admin / demo_operator / demo_readonly + 店铺授权 |

## F6 增强样本

- Dashboard 聚合（采集失败、订单异常、库存预警、客服待回复、配置风险）
- 订单 `partial_success` 同步
- 库存同步失败
- 客服 AI 已生成 + 发送失败
- 平台未授权 / 店铺隔离 / readonly 写阻断

## 输出

- `docs/demo-dataset.json` — 商品 / AI / 刊登
- `docs/demo-dataset.orders.json` — 订单
- `docs/demo-dataset.inventory.json` — 库存
- `docs/demo-dataset.customer.json` — 客服
- `docs/demo-dataset.permissions.json` — 权限
- `docs/demo-dataset.full-project.json` — F6 全链路索引（运行种子后生成）

## 账号

见 `docs/demo-dataset.permissions.json`：`demo_admin` / `demo_operator` / `demo_readonly`。
