# Demo Release 中文文案审计（Phase R1）

> 审计范围：面向用户的 Admin 页面与常见 API 错误展示  
> 工具：`node scripts/check-ui-copy.mjs --strict`  
> **结论**：无 P1 级内部码直出；技术详情默认折叠

## 审计方法

1. 全局搜索内部字段：`pending_review`、`partial_success`、`sourceSnapshotHash`、`expectedUpdatedAt`、`sourceType`、`local_draft_only`、`operationType`、`qualityWarnings` 等
2. 运行 `check-ui-copy.mjs --strict`（2026-06-27：**通过**）
3. 抽查复核弹窗、批次详情、工作台、失败任务中心

## 字段处理策略

| 内部码 | 用户可见处理 | 位置 |
| --- | --- | --- |
| `pending_review` | 「待复核」 | `aiProductText.ts` / `aiProductImage.ts` StatusTag |
| `partial_success` | 「部分成功」 | `copywriting.ts` / `publishLabels.ts` / 批次 Alert |
| `local_draft_only` | 「仅生成本地草稿」 | `publishLabels.ts` / 多平台刊登中心 |
| `sourceSnapshotHash` | 折叠于「技术详情」 | `ReviewItemModal` TechnicalDetails |
| `expectedUpdatedAt` | 仅 API 请求体，UI 不展示 | `DraftDetail` / services |
| `operationType` | 中文 `operationLabel` 主展示 | 复核页标题 |
| `qualityWarnings` | 中文 warning 列表 | 复核弹窗 Alert |
| `blocked_by_real_credentials` | 抖店 E2E 脚本 exit 3 说明，UI 用中文阻断原因 | 刊登/预检 |
| `sourceType` / `sourceId` | 工作台技术详情 JSON，主列表用中文 title | 工作台抽屉 |

## 本轮修复

| 文件 | 问题 | 修复 |
| --- | --- | --- |
| `Collect/Batches/index.tsx` | Alert 直出 `partial_success` | 改为「部分成功」 |

## 仍允许（技术详情折叠）

- 复核弹窗 `TechnicalDetails` 内 JSON 键名（`处理类型`、`内容快照`、`aiTaskId`）
- 开发/运维向日志与 smoke JSON 报告

## warning / failed 语义

- **warning**：使用「部分成功」「建议处理」，不写「系统失败」
- **failed**：使用「失败」「需处理」，与普通建议区分

## 结论

- ✅ 用户主路径无严重英文内部码直出
- ✅ `check-ui-copy --strict` 通过
- ⚠️ P2：部分旧版 `/ai/batches` 仍可见「旧版」提示（有意保留）

**Release 状态**：`MVP Demo Ready`

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1 文案审计 + Collect 批次修复 |
