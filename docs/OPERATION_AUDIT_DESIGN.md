# 操作审计设计（Phase F5）

## 表

`operation_logs` 扩展字段：`adminRole`、`shopId`、`platform`

## 必记敏感操作

用户角色/店铺权限变更、AI 应用、刊登草稿、SKU 绑定、库存/客服/任务重试、系统配置与 Storage 测试、抖店授权等。

## 禁止记录

完整密钥、Token、完整 Prompt、买家明文敏感信息。

## 查看权限

- admin：全部
- operator/readonly：按授权店铺过滤（`shop_id` 有值记录）

## API

`GET /api/v1/operation-logs` — 需 `operationlog.view`
