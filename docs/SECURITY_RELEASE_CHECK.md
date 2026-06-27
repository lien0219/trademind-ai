# Demo Release 权限与安全检查（Phase R1）

> **Release 状态**：`MVP Demo Ready`（非 Production Ready）  
> 检查方式：代码审查 + 既有单测 + 路由 smoke + 试跑脚本

## 权限与隔离

| # | 检查项 | 结果 | 依据 |
| --- | --- | --- | --- |
| 1 | 用户不能查看无权限商品 | ✅ | 商品 API JWT + 租户预留；MVP 单管理员 |
| 2 | 不能应用无权限 AI 文案 | ✅ | `aiproducttext` apply 校验商品归属与 batch CreatedBy |
| 3 | 不能应用无权限 AI 图片 | ✅ | `aiproductimage` 同批次权限模式 |
| 4 | 不能查看无权限刊登批次 | ✅ | `productpublish` 批次 `CreatedBy` 校验（A2.1） |
| 5 | 不能访问其他店铺任务 | ✅ | 店铺/同步任务按 shopId 隔离 |
| 6 | taskcenter 不泄露他人失败任务 | ✅ | 失败列表按管理员上下文；MVP 单租户 |
| 7 | AI Prompt 不完整写入日志 | ✅ | 日志规范 + AI 任务脱敏 |
| 8 | API Key 不出现在响应/日志 | ✅ | settings 加密 + 脱敏展示 |
| 9 | 图片 URL Token 不出现在日志 | ✅ | safefields / 抖店 sanitized logs |
| 10 | safedownload SSRF 防护 | ✅ | `go test ./internal/pkg/safedownload/...` |
| 11 | local_draft_only 不调外部平台 API | ✅ | publish orchestration + perf 脚本 `externalApiCalled=false` |
| 12 | 工作台 refresh 不调外部平台 API | ✅ | `aiopsworkbench` 只读聚合设计 |

## 自动化验证

```bash
go test ./internal/pkg/safedownload/...
go test ./internal/modules/aiopsworkbench/...
go test ./internal/modules/aiproducttext/...
go test ./internal/modules/aiproductimage/...
go test ./internal/modules/productpublish/...
go test ./internal/modules/taskcenter/...
.\scripts\demo-route-smoke.ps1
```

## 已知边界（Demo）

- MVP 默认单管理员，多租户 RBAC 为预留
- 抖店真实凭证 E2E 未在本阶段执行；OAuth state 与 token 加密已在 Phase 10.x 加固

## 结论

**通过** Demo Release 权限与安全检查；生产 Ready 仍需抖店真实 E2E + 灰度观察（见 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)）。

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1 安全检查清单 |
