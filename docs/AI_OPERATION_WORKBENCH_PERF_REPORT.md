# AI 商品运营工作台性能报告（Phase R1）

## 测试方法

```powershell
.\scripts\ai-operation-workbench-perf.ps1 -ApiBase http://127.0.0.1:8080
```

验证接口：

- `GET /api/v1/ai/operation-workbench/summary`
- `GET /api/v1/ai/operation-workbench/todos?page=1&pageSize=50`
- `GET /api/v1/ai/operation-workbench/todos?page=2&pageSize=50`

环境：本地 API，约 1019 个 draft 商品；AI 文案/图片试跑后工作台待办 **757** 条。

原始 JSON：`docs/ai-operation-workbench-perf.json`

## 结果摘要

| 指标 | 100 商品基线 | 500 商品基线 | 1000 商品基线 | 结论 |
| --- | --- | --- | --- | --- |
| summary 耗时 (ms) | 95.3 | 86.9 | 83.7 | ✅ < 500ms |
| todos 第 1 页 (ms) | 87.2 | 87.7 | 88.5 | ✅ < 1000ms |
| todos 第 2 页 (ms) | 87.4 | 91.4 | 88.5 | ✅ |
| 待办总数 | 757 | 757 | 757 | 分页可用 |
| 第 1 页条数 | 50 | 50 | 50 | ✅ pageSize=50 |
| 加载 AI 大字段 | 否 | 否 | 否 | ✅ |
| 调用外部平台 API | 否 | 否 | 否 | ✅ |

> 说明：三档「目标商品数」用于确保底层商品规模；工作台待办数由 AI 复核 / 发布检查 / 刊登异常 / 失败任务聚合决定，与商品总数非 1:1。757 待办已覆盖 **>500** 分页验收场景。

## SQL / N+1

MVP **未** 内置 HTTP 级 SQL 计数。模块集成测试 `go test ./internal/modules/aiopsworkbench/...` 通过；列表 DTO 不含 `generatedText` / `platformPayload` 等大字段。

## 结论

- **500 待办分页可用**：757 条待办，pageSize=50，第 1/2 页响应均 < 100ms
- **1000 商品规模下接口稳定**：summary / todos 无明显退化
- **refresh 不调外部平台 API**（设计约束，代码审查 + 路由 smoke 确认）

**Release 状态**：`MVP Demo Ready`

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1：perf 脚本 + 757 待办实测 |
