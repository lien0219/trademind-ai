# Demo 数据种子指南（Phase F7）

> **用途**：在本地或预发环境导入全链路演示数据，支撑 16 步 MVP 主链路走查。  
> **状态**：MVP Demo Ready · 非 Production Ready · 抖店 Release Candidate · Tag pending

## 前置条件

1. PostgreSQL + Redis 已启动（`docker compose up -d` 或等价）
2. 后端 API 可访问（默认 `http://127.0.0.1:8080`）
3. 根目录 `.env` 含 `ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PASSWORD`
4. （可选）AI Provider 已配置 — 客服 AI 建议样本为 best-effort

## 一键种子

```powershell
# 仓库根目录
.\scripts\seed-demo-data.ps1 -ApiBase http://127.0.0.1:8080 -OutFile docs/demo-dataset.json
.\scripts\seed-demo-permissions.ps1 -ApiBase http://127.0.0.1:8080
```

Linux / macOS：

```bash
./scripts/seed-demo-data.sh
./scripts/seed-demo-permissions.ps1   # 需 PowerShell
```

## 脚本行为

### seed-demo-data

1. 登录 bootstrap 管理员
2. 调用 `a1-prepare-samples.ps1` 补齐 20 商品 slot
3. 创建 **F2 订单**、**F3 库存**、**F4 客服** 样本
4. 探测 **F6/F7 Dashboard** KPI API
5. 汇总 AI / 刊登 / 失败任务 / 工作台待办
6. 写入：
   - `docs/demo-dataset.json`
   - `docs/demo-dataset.orders.json`
   - `docs/demo-dataset.inventory.json`
   - `docs/demo-dataset.customer.json`
   - `docs/demo-dataset.dashboard.json`
   - `docs/demo-dataset.full-project.json`

### seed-demo-permissions

创建 Demo 账号并写入 `docs/demo-dataset.permissions.json`：

| 账号 | 角色 | 用途 |
| --- | --- | --- |
| `demo_admin@trademind.local` | admin | 全权限演示 |
| `demo_operator@trademind.local` | operator | 店铺隔离演示 |
| `demo_readonly@trademind.local` | readonly | 只读阻断演示 |

默认密码见脚本输出或 `demo-dataset.permissions.json`（开发环境）。

## 验证

```powershell
# 读 validation 段
Get-Content docs/demo-dataset.json | ConvertFrom-Json | Select-Object -ExpandProperty validation
```

期望 `passed: true`（至少 20 slot、7 task samples、订单/库存/客服各 ≥3）。

专项 smoke（需 API 在线）：

```powershell
.\scripts\demo-dashboard-smoke.ps1
.\scripts\demo-order-inventory-customer-smoke.ps1
.\scripts\demo-rbac-smoke.ps1
```

## 注意事项

- **不写入真实平台数据**；抖店步骤预期 `blocked_by_real_credentials` 或 `local_draft_only`
- 重复运行会追加/更新样本，演示前可清空 dev 库或接受增量
- 商品标题含 `R1 demo` / `F3 demo` 等前缀便于检索

## 相关文档

- [`DEMO_DATASET.md`](DEMO_DATASET.md) — slot 与样本明细
- [`FULL_PROJECT_DEMO_DATASET.md`](FULL_PROJECT_DEMO_DATASET.md) — 全项目数据集索引
- [`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md) — 自动化回归
- [`../DEMO_CHECKLIST.md`](../DEMO_CHECKLIST.md) — 验收勾选
