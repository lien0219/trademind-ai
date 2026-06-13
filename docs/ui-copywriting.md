# 管理端与 API 用户可见文案规范

TradeMind 面向跨境卖家与运营人员，**用户可见文案默认使用中文**。代码标识符、路由、JSON 字段键、日志与「技术详情」折叠区可保留英文；主界面、告警、错误提示、表单 Label/Help、预检项 Title/Message 不得直接裸露英文技术词。

## 必读入口（AI 与开发者）

| 资源 | 用途 |
| --- | --- |
| 本文 | 术语表、禁止项、检查方式 |
| `admin/src/constants/copywriting.ts` | 页面标题、商品/平台/任务/库存统一术语 |
| `admin/src/constants/userFriendly.ts` | 通用标签与 `platformLabel()` 等映射 |
| `admin/src/constants/errorMessages.ts` | 错误码 → 用户提示 |
| `docs/ai-workflow.md` § Admin 文案与 UI 规范 | AI 工作流中的摘要约束 |
| `.cursor/rules/14-ui-copywriting.mdc` | Cursor 领域规则 |
| `scripts/check-ui-copy.mjs` | 本地/CI 混排英文扫描 |

## 核心原则

1. **面向用户，不面向开发者**：主界面不直接展示 Provider、Worker、runtime、Storage、Payload、Stale、Endpoint 等内部词。
2. **统一术语**：同一概念全项目一个中文叫法（见下表）；新增文案优先从 `copywriting.ts` / `userFriendly.ts` 引用。
3. **必要英文**：平台品牌（TikTok Shop、Amazon SP-API）、协议名（HTTPS）、云厂商产品名（S3、OSS）可保留；配置项若与开放平台文档一致，可用「中文（英文）」如「应用 Key（App Key）」——**副标题与说明句中优先纯中文**。
4. **技术信息下沉**：错误码、Request ID、原始 JSON、字段名放入 `TechnicalDetails` / `TaskJsonBlock`，默认收起。
5. **后端同步**：会进入 API `message`、`label`、`title`、`suggestedAction` 的 Go 字符串与前端同等要求。

## 标准术语表

| 避免（主界面裸英文） | 统一中文 | 备注 |
| --- | --- | --- |
| Worker | 后台任务进程 | 菜单已为「后台任务监控」 |
| runtime | 运行时 | |
| Provider（泛称） | 接入方式 / 图片处理服务 / AI 服务商 | 按场景选 `userFriendly.ts` 常量 |
| Storage | 存储 | |
| API（泛指接口） | 接口 | 「API 密钥」→「接口密钥」 |
| API Key | 接口密钥 | 设置页 |
| Token / Access Token / Refresh Token | 访问令牌 / 刷新令牌 | |
| OAuth | 店铺授权 | 流程说明可用「授权（OAuth）」 |
| App Key / App Secret | 应用 Key / 应用密钥 | 抖店等平台文档常用可保留 Key |
| Stale | 停滞 | 任务长时间无进展 |
| SKU（对用户） | 规格 / 商品规格 | 代码字段名不变 |
| Mock | 模拟 | 模拟店铺 |
| Webhook | 回调通知 | |
| SMTP | 邮件服务器 | 可写「邮件（SMTP）」 |
| Endpoint | 接口地址 | |
| Bucket | 存储桶 | |
| E2E | 端到端联调 | |
| Release Candidate | 发布候选 | |
| JSON（表单标签） | 配置（JSON） / JSON 格式 | 编辑区可提示格式 |
| need_check | 需要检查 | 状态枚举用 `commonStatusLabel` |
| public_base | 公网访问地址 | 帮助文字中说明 |

## 禁止出现在主界面的模式

- 裸内部枚举：`blocked_by_*`、`runtimeBlocked24h`、`emergency_disabled`（应用 `commonStatusLabel` 或专用映射）
- 裸 Phase / RC / taskcenter 等开发阶段用语
- 裸数据库表名：`product_publications`（改为「商品刊登映射」）
- 裸函数/实现名：`page.evaluate`（改为「页面解析脚本」；详细函数名仅技术详情）

## 实现约定

### 前端

- 页面 `title` / `subTitle` / `description` / `Alert` / `message.*` / 表格列 `title` / `Form.Item label` 走中文。
- 状态 Tag 使用 `commonStatusLabel()`、`platformLabel()`、`failureCategoryLabel()` 等，禁止直接渲染英文枚举。
- 新增常量放入 `copywriting.ts` 或 `userFriendly.ts`，页面引用而非硬编码。

### 后端

- `response.Fail` 的 `message`、`TestConnectionResult.Message`、健康检查 `Label`、预检 `Title`/`Message`/`Suggestion`、告警 `Title`/`SuggestedAction` 使用中文。
- 英文仅保留在：错误码常量、JSON 键、日志、`*_test.go` 内部断言（若断言用户可见文案则同步中文）。

## 检查命令

```bash
pnpm check:ui-copy
pnpm check:ui-copy --strict   # CI 使用；有命中则 exit 1
```

扫描 `admin/src` 用户可见路径与 `backend/internal/modules` 下常见混排模式。**CI（`node.yml` admin job）在构建前执行 `pnpm check:ui-copy --strict`**，有命中则阻断合并。

## AI 工作流约束（摘要）

生成或修改**用户可见文案**时 Agent 必须：

1. 先查 `copywriting.ts`、`userFriendly.ts` 是否已有术语。
2. 不得在主界面新增裸英文技术词；若 API 返回英文，在前端或后端增加中文映射。
3. 完成 Admin/后端用户文案改动后运行 `pnpm check:ui-copy`。
4. 新增高频术语时更新本文术语表与 `userFriendly.ts`。

详见 `docs/ai-workflow.md` 与 `.cursor/rules/14-ui-copywriting.mdc`。
