# AI 回复建议设计（Phase F4）

## 流程

选择会话 → 生成建议 → 人工编辑 → 二次确认 → 发送（或仅采纳为内部记录）。

## API

- `POST /api/v1/customer/conversations/:id/ai/generate-reply`
- `GET /api/v1/customer/conversations/:id/ai-suggestions`
- `PUT /api/v1/customer/reply-suggestions/:id`
- `POST /api/v1/customer/ai-suggestions/:id/apply`
- `POST /api/v1/customer/ai-suggestions/:id/reject`

## 状态

`generated` / `edited` / `accepted` / `rejected` / `discarded` / `generate_failed` / `send_failed`

## 上下文

Prompt 使用订单、商品、库存摘要；前端仅展示 `contextSummary`，不含完整 Prompt 与平台 raw。

## 失败

生成/发送失败写入 `customer_failure_events`，聚合至失败任务中心 `customer_failure`。
