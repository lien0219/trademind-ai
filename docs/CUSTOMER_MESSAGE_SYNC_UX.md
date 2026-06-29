# 客服消息同步 UX（Phase F4）

## 页面

`/customer/message-sync-tasks`

## 状态

与订单同步一致：`pending` / `running` / `success` / `partial_success` / `failed` / `cancelled` / `blocked`

## partial_success

展示成功/失败会话数、失败游标、错误信息、是否可重试；不显示为完全成功或完全失败。

## 未授权

店铺未授权时任务创建/执行被阻断，文案为「待授权」，不记为系统 bug。

## 失败任务

同步失败进入失败任务中心 `customer_message_sync`；详情深链 `/customer/message-sync-tasks?id=`
