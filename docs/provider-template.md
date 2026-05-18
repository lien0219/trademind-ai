# Provider 开发模板

新增 AI、Storage、Image、Platform、Collector Provider 时，先按本模板确认边界，再实现代码。Provider 的目标是隔离第三方差异，避免业务模块直接依赖具体 SDK 或平台细节。

## 基本信息

| 项目 | 内容 |
| --- | --- |
| Provider 类型 | AI / Storage / Image / Platform / Collector |
| Provider 名称 | 例如 `openai-compatible`、`local`、`tiktok` |
| 业务入口 | 对应 handler / service / worker |
| 配置来源 | settings / 环境变量 / Docker |
| 是否异步 | 是 / 否 |
| 是否涉及敏感字段 | 是 / 否 |

## 实现步骤

1. 定义或复用 Provider 接口。
2. 定义统一请求、响应和错误结构。
3. 实现具体 Provider，不在业务 handler 中散写第三方 SDK。
4. 为所有外部请求设置超时。
5. 敏感配置走加密存储和脱敏展示。
6. 接入连接测试或最小可用验证。
7. 接入任务状态、重试和错误归因，耗时能力必须异步。
8. 同步文档、环境模板、设置页面和测试说明。

## 配置项模板

| 字段 | 示例 | 是否敏感 | 存储位置 | 说明 |
| --- | --- | --- | --- | --- |
| `provider` | `openai-compatible` | 否 | settings | Provider 类型。 |
| `base_url` | `https://api.example.com/v1` | 否 | settings | 服务地址。 |
| `api_key` | `sk-***` | 是 | settings 加密 | API Key，不写入日志。 |
| `timeout_sec` | `60` | 否 | settings | 调用超时。 |

## 错误处理

- 第三方错误要转换为可读业务错误。
- 记录 traceId、provider、任务 ID、错误类型，但不得记录完整密钥或 Token。
- 可重试错误和不可重试错误要区分。
- 异步任务失败时写入任务状态和失败原因，便于任务中心展示。

## 前端联动

新增 Provider 时，通常需要检查：

- 设置页表单字段、校验、脱敏展示。
- 连接测试按钮。
- Provider 枚举和状态文案。
- 任务列表或结果页的错误提示。

## 文档联动

必须同步：

- `docs/provider.md`
- `docs/module-map.md`
- `docs/env.md`（如新增环境变量）
- `docs/api.md`（如新增 API）
- `docs/PROGRESS.md`（较大 Provider 或可交付能力）
- README / README.en 的功能状态（如用户可见能力变化）

## 安全检查

- [ ] 密钥不写代码、不写日志、不写截图。
- [ ] 敏感字段存库加密。
- [ ] 前端展示脱敏。
- [ ] 外部请求有超时。
- [ ] Webhook / OAuth 回调校验来源或签名。
- [ ] 错误信息不泄露平台凭证。
