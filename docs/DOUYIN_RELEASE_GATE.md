# 抖店发布门禁（Release Gate）

> **当前发布状态**：**Release Candidate**（非 production available）  
> **真实 E2E**：`blocked_by_real_credentials`（无凭证环境不得伪造通过）

---

## 1. 代码与 CI

| # | 门禁项 | 通过标准 | 状态 |
| --- | --- | --- | --- |
| 1.1 | `go test ./...` | backend 全量测试绿 | CI `backend` job |
| 1.2 | Race 测试 | `douyinshop`、`ordersync`、`inventory`、`productpublish` 包 `-race` 无 DATA RACE | CI `backend-race` job（`CGO_ENABLED=1`） |
| 1.3 | `go build ./cmd/server/...` | 服务端可编译 | CI `backend-race` job |
| 1.4 | `pnpm build:admin` | 管理端可构建 | CI `node.yml` |
| 1.5 | 契约测试 | `douyinshop/contract_test.go` + Fixture | 本地/CI |

---

## 2. 文档与 Runbook

| # | 门禁项 | 文档 |
| --- | --- | --- |
| 2.1 | 生产审计 | [`DOUYIN_PRODUCTION_AUDIT.md`](DOUYIN_PRODUCTION_AUDIT.md) §5.3 Phase 10.4 |
| 2.2 | E2E 清单 | [`DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md) |
| 2.3 | 灰度 Runbook | [`DOUYIN_PRODUCTION_RUNBOOK.md`](DOUYIN_PRODUCTION_RUNBOOK.md) § 灰度观察 |
| 2.4 | 回滚 Runbook | [`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md) |
| 2.5 | 回滚演练 | [`DOUYIN_ROLLBACK_DRILL_REPORT.md`](DOUYIN_ROLLBACK_DRILL_REPORT.md)（`environment_simulation_only` 可接受于 RC） |

---

## 3. E2E 脚本（Phase 10.4）

| # | 脚本 | 无凭证预期 | 有凭证预期 |
| --- | --- | --- | --- |
| 3.1 | `scripts/douyin-e2e-preflight` | exit `3`，stderr `blocked_by_real_credentials` | 预检 JSON 归档 |
| 3.2 | `scripts/douyin-e2e-readonly` | exit `3` blocked | 类目/任务中心/看板探针绿 |
| 3.3 | `scripts/douyin-e2e-write` | exit `3` 或 exit `4`（未设 `ALLOW_DOUYIN_WRITE_TEST`） | 写链路脚手架 + 人工清单 |
| 3.4 | `scripts/douyin-e2e-report` | Markdown 报告含 blocked 结论 | 完整 §9 发布结论 |

环境变量：`TRADEMIND_API_BASE`、`TRADEMIND_ADMIN_ACCOUNT`、`TRADEMIND_ADMIN_PASSWORD`；写测试 **`ALLOW_DOUYIN_WRITE_TEST=true`**。

---

## 4. 生产预检（有凭证环境）

| # | 检查 | API / 入口 |
| --- | --- | --- |
| 4.1 | Storage 公网 | `POST /api/v1/storage/test-public-access` |
| 4.2 | 抖店预检 | `POST /api/v1/platform/douyin/production-preflight`（`liveTest: true`） |
| 4.3 | 运行状态 | `GET .../platform/douyin/runtime-status` → `normal` |
| 4.4 | 健康 | `GET /health` → `orderSyncQueue` / `productPublishQueue` / `inventorySyncQueue` 无 degraded |

---

## 5. 可观测性（无 Prometheus）

| # | 能力 | 说明 |
| --- | --- | --- |
| 5.1 | `/health` | 队列与 Worker 心跳 |
| 5.2 | `GET .../platform/douyin/health` | 抖店聚合健康（config/auth/storage/tasks/api） |
| 5.3 | `GET .../platform/douyin/metrics-summary` | 24h 进程内指标（非 Prometheus） |
| 5.4 | `GET .../platform/douyin/release-gate` | RC 门禁 API |
| 5.5 | taskcenter | 失败任务、告警 scan、Webhook（可选） |
| 5.6 | operationlog | `douyin.*` 审计 |
| 5.7 | operationdashboard | 商品运营看板 |
| 5.8 | **不做** | Prometheus / Grafana 专用 `/metrics` |

---

## 6. 灰度与发布结论

| # | 条件 | 要求 |
| --- | --- | --- |
| 6.1 | 真实 E2E 全绿 | [`DOUYIN_E2E_REPORT_TEMPLATE.md`](DOUYIN_E2E_REPORT_TEMPLATE.md) §9；`real_e2e_status` ≠ blocked |
| 6.2 | 灰度观察 | G1–G2 共 48–72h，见 Runbook |
| 6.3 | 幂等验收 | 重复订单 / 扣库存 = 0 |
| 6.4 | 回滚演练 | 生产环境实跑或升级 drill 报告 |
| 6.5 | Tag | 建议 `v0.8.0-douyin-mvp-demo`（全部门禁通过后） |

**当前 RC 结论**：§1–§2、§3（blocked 预期）、§5 文档/CI 就绪；§4、§6 待真实凭证与灰度。

---

## 7. 变更记录

| 日期 | 摘要 |
| --- | --- |
| 2026-06-13 | Phase 10.4 初版 Release Gate |
