# 订单 SKU 匹配与候选 UX（Phase F2）

## 状态

| match_status | 中文 |
| --- | --- |
| matched | 已匹配 |
| manual_bound | 人工绑定 |
| ambiguous | 候选待确认 |
| unmatched | 未匹配 |
| skipped | 已跳过 |

## 候选 API

- `GET /api/v1/order-items/:itemId/sku-candidates`
- `POST /api/v1/orders/:id/sku-candidates/batch`

候选字段：confidence、reason、matchSignals、stock、sourceBreakdown。

## 规则

1. 候选只读，**不自动绑定**
2. 绑定走 `POST /order-items/:itemId/bind-sku` 或异常工作台 bind-sku
3. 低置信度（<40）UI 标注「参考」
4. 已 `manual_bound` 行保留既有自动匹配保护。明细有成功库存流水后，人工绑定和强制重新匹配均不能改变该明细的规格归属。
5. 自动匹配按照订单租户和店铺读取刊登规格，回退到本地编码时只搜索当前租户。
5. 绑定/解绑写 operation log

## 页面入口

- 订单详情 Tab「SKU 匹配」
- `/orders/sku-matches` 全局列表
- 异常工作台「查看候选 / 绑定 SKU」

## 深链

`/orders/:orderId?itemId=:orderItemId` 打开详情并聚焦对应行
