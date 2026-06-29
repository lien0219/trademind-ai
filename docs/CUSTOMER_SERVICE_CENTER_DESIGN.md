# 客服中心设计（Phase F4）

## 入口

- 客服中心首页：`/customer/hub`（KPI + 快捷入口）
- 会话列表：`/customer/conversations`
- 会话详情：`/customer/conversations/:id`
- 消息同步任务：`/customer/message-sync-tasks`
- API 别名：`/api/v1/customer/*` 与 `/api/v1/customer-service/*`

## 列表字段

平台、店铺、买家（脱敏）、状态、最近消息、关联订单/商品、AI 建议状态、发送状态、更新时间。

## 权限

- `admin` / `operator`：可生成建议、发送、重试
- `readonly`：仅查看（后端 `CanWriteCustomer` + 前端 `canWrite`）

## 原则

不自动发送；不展示平台 raw；技术详情默认折叠。
