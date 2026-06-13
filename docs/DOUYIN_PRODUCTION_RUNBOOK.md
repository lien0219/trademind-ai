# 抖店生产 Runbook（Phase 10.3–10.4）

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

## 灰度观察阶段（Phase 10.4）

> 有真实凭证并通过 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md) 预检后执行。全程保持 **Release Candidate**，未全绿前不得标记 production available。

### Phase G0 — 发布前（0–2h）

1. 运行 `scripts/douyin-e2e-preflight`（预期有凭证时 `blockedByRealCredentials=false`）
2. 确认 `runtime-status=normal`，开关与 Runbook § 上线前检查一致
3. 备份 PostgreSQL；确认迁移无重复订单阻断
4. 记录 Git SHA 与配置快照（不含 Secret）

### Phase G1 — 只读观察（2–24h）

1. 运行 `scripts/douyin-e2e-readonly`；归档 JSON 至 `DOUYIN_E2E_REPORT_DIR`
2. 每 4h 检查：`GET /health` 队列深度、`task-center/summary` 抖店失败数
3. 操作日志抽查：`douyin.auth.*`、无 Token 明文
4. **禁止**开启写链路脚本，除非已进入 G2 且 `ALLOW_DOUYIN_WRITE_TEST=true`

### Phase G2 — 写链路小流量（24–72h）

1. 单测试店铺、单 SKU、小时间窗订单同步
2. 运行 `scripts/douyin-e2e-write`（显式 `ALLOW_DOUYIN_WRITE_TEST=true`）
3. 验证幂等：重复订单 Upsert = 0、重复扣库存 = 0（见 E2E 报告模板 §6）
4. 失败任务 → 告警 scan → 人工处理；P0 错误立即 `paused` 或 `emergency_disabled`

### Phase G3 — 收口

1. 填写 [`DOUYIN_E2E_REPORT_TEMPLATE.md`](DOUYIN_E2E_REPORT_TEMPLATE.md) §9 发布结论
2. 完成回滚演练记录（可为 `environment_simulation_only`）
3. 仍 **不** 默认引入 Prometheus；继续用 taskcenter + `/health` + 看板

## 可观测性（无 Prometheus）

| 检查 | 频率 | 入口 |
| --- | --- | --- |
| 进程与队列 | 5min（自动化） / 人工按需 | `GET /health` |
| 抖店失败任务 | 每小时 | 运维 → 失败任务中心；`GET /task-center/failures?keyword=DOUYIN` |
| 告警 | 每 15min（若开启 scan worker） | `POST /task-center/alerts/scan` |
| 运营 KPI | 每日 | 工作台 → 商品运营看板 |
| 预检 | 每次发布前 | `POST .../production-preflight` |

## 相关文档

- [`DOUYIN_PRODUCTION_AUDIT.md`](DOUYIN_PRODUCTION_AUDIT.md)
- [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)
- [`DOUYIN_DUPLICATE_DATA_REPAIR.md`](DOUYIN_DUPLICATE_DATA_REPAIR.md)
- [`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md)
- [`DOUYIN_ROLLBACK_DRILL_REPORT.md`](DOUYIN_ROLLBACK_DRILL_REPORT.md)
