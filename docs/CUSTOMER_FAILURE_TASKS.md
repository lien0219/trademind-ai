# 客服域失败任务（Phase F4）

## 任务类型

| 类型 | 来源 |
| --- | --- |
| `customer_message_sync` | `customer_message_sync_tasks` |
| `customer_failure` | `customer_failure_events` |

## 失败分类（customer_failure）

- `customer_reply_generate_failed`
- `customer_reply_send_failed`
- `customer_reply_permission_denied`
- `customer_platform_not_authorized`
- `customer_conversation_not_found`
- `customer_message_sync_failed` / `customer_message_sync_partial_success`（同步任务表）

## 深链

- 会话：`/customer/conversations/:id`
- 带建议：`/customer/conversations/:id?suggestionId=:id`
- 同步任务：`/customer/message-sync-tasks?id=:taskId`
- 平台授权：`/settings/platforms?platform=:platform`

## 操作

查看会话、重新生成建议、重新发送、查看同步任务、查看平台授权。
