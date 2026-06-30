# TradeMind Phase F8 — P0 / P1 清零审计

> **日期**：2026-06-30  
> **阶段**：Phase F8（功能冻结前收口）  
> **目标**：P0 = 0，P1 = 0（代码与 Demo 层面）；环境依赖单独标记。

## 摘要

| 指标 | F1 基线 | F7 后 | F8 后 |
| --- | --- | --- | --- |
| P0 open（代码） | 6 | 0（2 项 reclass） | **0** |
| P1 open（代码） | 14 | 1 | **0** |
| P2 保留 | 12 | 12 | 12（见 POST_FREEZE_BACKLOG） |
| P3 保留 | 8 | 8 | 8 |

**结论**：主链路 Demo 可走查；抖店真实 E2E / 预发 / tag **留 F9**；**Function Freeze Ready**。

---

## P0 清零明细

| ID | 问题描述 | 来源 | 影响 | 阻塞 F9 | 处理方式 | 处理结果 | 剩余风险 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| P0-01 | 无 RBAC | F1 | 越权 | 是 | F5 交付 RBAC | **已关闭** | 无 |
| P0-02 | 无真实抖店凭证无法验证 product.addV2 | F1 | 误导真实发布 | 否 | `environment_required` → F9 | **降级为 F9 环境项** | 需真实凭证 E2E |
| P0-03 | 本地 Storage 无公网 URL | F1 | 抖店图片 E2E 断点 | 否 | `environment_required` → F9 | **降级为 F9 环境项** | 需公网 Storage |
| P0-04 | 客服发送失败未进失败任务中心 | F1 | 发送失败无感知 | 是 | F4 失败事件 | **已关闭** | 无 |
| P0-05 | 无总 Dashboard 异常入口 | F1 | 主链路后半段难发现 | 是 | F6 Dashboard | **已关闭** | 无 |
| P0-06 | 无独立订单详情页 | F1 | 订单跳转链不完整 | 是 | F2 订单中心 | **已关闭** | 无 |

---

## P1 清零明细

| ID | 问题描述 | 来源 | 影响 | 阻塞 F9 | 处理方式 | 处理结果 | 剩余风险 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| P1-01 | 无用户管理 UI/API | F1 | 多角色试用 | 是 | F5 | **已关闭** | 无 |
| P1-02 | 无配置状态中心 | F1 | 配置健康不可见 | 是 | F5 | **已关闭** | 无 |
| P1-03 | 非抖店 local_draft_only 标注不清 | F1 | 状态误解 | 是 | F6 文案/RC | **已关闭** | 无 |
| P1-04 | partial_success 页级错误 UI 不足 | F1 | 订单同步状态不清 | 是 | F2 | **已关闭** | 无 |
| P1-05 | 异常工作台 ↔ 失败任务深链 | F1 | 跳转不完整 | 是 | F2 | **已关闭** | 无 |
| P1-06 | ambiguous SKU 批量 UX 弱 | F1 | 异常处理慢 | 是 | F2 | **已关闭** | 无 |
| P1-07 | inventory_sync_enabled 默认 off 无引导 | F1 | 新手不知开启 | 是 | F3 横幅 | **已关闭** | 无 |
| P1-08 | SKU 未绑定提示分散 | F1 | 库存阻断不清 | 是 | F3 统一文案 | **已关闭** | 无 |
| P1-09 | 会话缺订单/商品上下文 | F1 | 客服效率 | 是 | F4 侧栏 | **已关闭** | 无 |
| P1-10 | 待回复数未进 Dashboard | F1 | KPI 缺失 | 是 | F4+F6 | **已关闭** | 无 |
| P1-11 | 旧版 /ai/batches 入口混淆 | F1 | 入口混乱 | 是 | F6 隐藏/标识 | **已关闭** | 无 |
| P1-12 | docs/api.md legacy 路径 | F1 | 文档与实现不一致 | 是 | F8 同步 `/shops`、客服 AI 路径 | **已关闭** | 无 |
| P1-13 | Demo 订单/库存/客服样本不足 | F1 | Demo 不完整 | 是 | F7 seed | **已关闭** | 无 |
| P1-14 | 抖店 RC 标识不统一 | F1 | 状态误解 | 是 | F6 RC 标识 | **已关闭** | 无 |

---

## F7 → F8 剩余项处理

| 项 | 处理方式 | F8 结果 |
| --- | --- | --- |
| Demo worker 依赖 edge-case 样本 | 方案 A：`POST /api/v1/dev/demo-seed/full-project-edge-cases` | **已实现**（dev/demo only） |
| 商品刊登配置 sensitiveConfirm | `confirmPlatformPublishConfigSave` | **已接入** DraftDetail |
| 采集目标店铺提示 | Collect Hub/Tasks + 空状态 + 成功提示 | **已落地** |
| demo:auto-acceptance 未跑 | F8 复跑或记录 backend 不可用 | **见测试报告** |

---

## 环境依赖（不计入 P0/P1）

| ID | 描述 | 标记 | F9 动作 |
| --- | --- | --- | --- |
| ENV-01 | 抖店真实凭证 E2E | `manual_required` | F9 人工 + 脚本 |
| ENV-02 | Storage 公网 URL | `environment_required` | 预发 Storage 配置 |
| ENV-03 | 运行中 backend 做 API smoke | `environment_required` | F9 环境初始化 |
| ENV-04 | 1366/1024 截图验收 | `manual_required` | F9 人工 |
| ENV-05 | 浏览器后退/刷新状态 | `manual_required` | F9 人工 |

---

## P2 / P3

见 [`POST_FREEZE_BACKLOG.md`](POST_FREEZE_BACKLOG.md)。**不阻塞 F9**。

---

## 相关文档

- [`FUNCTION_FREEZE_RULES.md`](FUNCTION_FREEZE_RULES.md)
- [`F9_FINAL_ACCEPTANCE_PRECHECK.md`](F9_FINAL_ACCEPTANCE_PRECHECK.md)
- [`FULL_PROJECT_MVP_GAP_AUDIT.md`](FULL_PROJECT_MVP_GAP_AUDIT.md)
- [`PROGRESS.md`](PROGRESS.md)
