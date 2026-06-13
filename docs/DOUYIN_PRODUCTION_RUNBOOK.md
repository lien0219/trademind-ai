# 抖店生产 Runbook（Phase 10.3）

> **状态**：Release Candidate  
> **真实 E2E**：`blocked_by_real_credentials`

## 上线前检查顺序

1. 备份 PostgreSQL（`orders` / `order_items`）
2. 启动应用：`migrateDouyinPhase102Indexes` **自动**检测重复订单；有重复则启动失败并输出 `sample_ids`（见 [`DOUYIN_DUPLICATE_DATA_REPAIR.md`](DOUYIN_DUPLICATE_DATA_REPAIR.md)）
3. 设置 → 存储 → **测试公网访问**
4. 设置 → 平台开放配置 → 抖店 → **生产预检**
5. 确认 `real_api_enabled`、订单/库存/商品草稿开关符合预期
6. 确认抖店运行状态为 **正常运行**（非暂停/紧急停用）

## 运行状态操作

| 状态 | 含义 | 操作入口 |
| --- | --- | --- |
| `normal` | 正常执行任务 | 设置 → 平台开放配置 → 抖店运行状态 → 恢复运行 |
| `paused` | 暂停新任务与写操作 | 暂停任务（需填写原因） |
| `emergency_disabled` | 紧急停用所有抖店写接口 | 紧急停用（需二次确认+原因） |

API：`GET/POST /api/v1/platform/douyin/runtime-status/*`

## Stale / 结果未知任务

| 用户可见文案 | 内部 recoveryStatus | 处理 |
| --- | --- | --- |
| 任务执行时间过长 | `stale` | 检查 Worker/租约；人工重试 |
| 平台处理结果暂时无法确认 | `result_unknown` | 商品：先 `product.detail` 回查，禁止盲目 `product.addV2` |
| 需要检查后才能继续 | `recovery_required` | 人工确认后重试 |

## 重试策略（Phase 10.3）

- 所有抖店 OpenAPI 经 `Client.Do` → `ExecuteWithRetry`（最多 3 次，指数退避+抖动）
- 权限/参数/配置错误 **不重试**
- Token 刷新按 `shopId` 单飞
- 业务 Worker **不嵌套** HTTP 重试

## 故障恢复

- 订单同步：从 checkpoint 继续，仅重试失败页
- 库存同步：使用任务创建时 `targetStock`；旧版本任务不覆盖新版本
- 图片上传：已成功图片跳过；`force=true` 才强制重传

## 相关文档

- [`DOUYIN_PRODUCTION_AUDIT.md`](DOUYIN_PRODUCTION_AUDIT.md)
- [`DOUYIN_DUPLICATE_DATA_REPAIR.md`](DOUYIN_DUPLICATE_DATA_REPAIR.md)
- [`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md)
