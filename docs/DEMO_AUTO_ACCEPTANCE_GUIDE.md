# Demo 自动化验收指南（Phase F7-Auto）

> **用途**：本地 / CI 可运行的自动化回归，**不替代** Phase F9 最终人工总体验收。  
> **状态**：MVP Demo Ready · 非 Production Ready · Tag pending · 无真实抖店 E2E

## 总控脚本

```powershell
.\scripts\demo-auto-acceptance.ps1 -ApiBase http://127.0.0.1:8080
```

```bash
./scripts/demo-auto-acceptance.sh
```

参数：

| 参数 | 说明 |
| --- | --- |
| `-ApiBase` | 后端地址，默认 `http://127.0.0.1:8080` |
| `-SkipApiTests` | 跳过需 API 的步骤（仅静态检查） |
| `-SkipBuild` | 跳过 go test / build / admin build |

## Phase F7-Auto 步骤概览

| 类别 | 步骤 |
| --- | --- |
| 构建 | `go test ./...`、`go build`、`pnpm build:admin` |
| 静态 | `git diff --check`、`check-ui-copy --strict`、`demo-empty-state-scan`、`demo-sensitive-confirm-scan`、`security-release-check`、`check-doc-links` |
| API（需 backend） | `demo-route-smoke`、`seed-demo-data`、`seed-demo-permissions`、`demo-dashboard-smoke`、`demo-rbac-smoke`、`demo-order-inventory-customer-smoke` |
| AI / perf | AI 文案/图片 route smoke + trial run、publish-batch-perf、ai-operation-workbench-perf |

后端不可达时 API 步骤标记 `skipped`，不阻断静态项。

## 输出报告

| 文件 | 内容 |
| --- | --- |
| `docs/DEMO_AUTO_ACCEPTANCE_REPORT.md` | 人类可读总报告 |
| `docs/demo-auto-acceptance.json` | 机器可读汇总 |
| `docs/*.auto.json` / `*.auto.md` | 各子步骤分项报告 |
| `docs/global-status-copywriting-scan.json` | F7 全局状态文案扫描 |

## 通过标准（F7）

- 总控结论 `passed` 或 AI 试跑 `passed_with_warning`（已知图片 Provider 限制）
- `seed-demo-data` validation `passed`
- `check-ui-copy --strict` + `global-status-copywriting-scan.json` → `passed: true`

## 明确不在 F7-Auto 范围（留 F9）

- 最终人工完整走查与多分辨率截图
- 真实预发 HTTPS / Nginx / Storage 公网
- 抖店真实凭证 E2E
- 生产灰度与 **`v0.1.0-demo` tag**
- Production Ready 判定

## 相关

- [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md) — 种子前置
- [`../DEMO_CHECKLIST.md`](../DEMO_CHECKLIST.md) — 人工勾选清单
- [`FULL_PROJECT_DEVELOPMENT_PLAN.md`](FULL_PROJECT_DEVELOPMENT_PLAN.md) — F8/F9 边界
