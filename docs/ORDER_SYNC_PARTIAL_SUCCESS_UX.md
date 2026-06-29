# 订单同步 partial_success UX（Phase F2）

## 语义

- **partial_success**：部分页已成功写入本地订单，部分页拉取失败
- 不得展示为「完全成功」或「完全失败」

## 任务 output 字段

```json
{
  "totalPages": 10,
  "successPages": 8,
  "failedPages": 2,
  "totalFetched": 120,
  "createdOrders": 5,
  "updatedOrders": 115,
  "matchedItems": 100,
  "unmatchedItems": 20,
  "pageErrors": [{ "page": 3, "error": "..." }]
}
```

## 重试策略

- `POST /api/v1/order-sync/tasks/:id/retry` 对 `partial_success` 自动设置 `input.retryPagesOnly`
- 仅重拉 `pageErrors` 中的页码，不重复成功页

## UI（`/orders/sync-tasks`）

- 详情 Drawer 展示分页统计与失败页表格
- 「重试失败页」按钮
- 链到失败任务中心与异常工作台

## 失败任务中心

- `partial_success` 纳入失败列表（`normalizedStatus=partial_success`）
- `Retryable=true`
