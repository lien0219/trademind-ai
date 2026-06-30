# AI 工作流优化指南

本文用于让 Codex、Cursor、Claude Code、Copilot、Continue、Windsurf、Trae 等 AI 编程工具在 TradeMind 中更智能、快速、准确、高效并节约 token。核心目标是：**少读无关内容，先形成最小上下文包，再小步实现、验证和沉淀经验**。

## 适用范围

- 日常 vibe coding、Bug 修复、功能开发、重构、文档改进和 PR 准备。
- 适用于所有 AI Agent 和 AI 编辑器；工具特有规则只放在对应配置中，长期通用规则放在 `AGENTS.md` 与本文。
- 不替代 [ai-coding-rules.md](ai-coding-rules.md)、[module-map.md](module-map.md) 和 [task-checklist.md](task-checklist.md)，而是说明如何更省上下文地使用它们。

## 工作原则

1. **先定位任务类型，再读取文件**：不要一开始全量读取仓库。
2. **先读入口，再读局部**：`AGENTS.md`、本文、`docs/module-map.md`、相关模块文档、相关代码。
3. **先事实后方案**：用搜索和现有代码确认脚本、路由、字段、配置、队列名和 Provider 接口。
4. **小步改动，小步验证**：优先完成一个可验证闭环，再扩展关联内容。
5. **保留人类决策权**：涉及产品范围、外部平台、密钥、生产数据、破坏性操作或高费用 AI 调用时必须谨慎确认。
6. **把经验写回仓库**：重复出现的问题、架构决策和工具约定必须沉淀到对应文档，而不是只留在某次对话里。

## 最小上下文包

AI 开始任务时优先整理一个不超过 10 条的上下文包：

| 内容 | 说明 |
| --- | --- |
| 任务目标 | 用户真正要的结果，而不是工具的第一反应 |
| 当前分支与改动 | `git status --short --branch`，避免覆盖用户修改 |
| 改动类型 | 后端、Admin、Collector、Provider、API、配置、Docker、CI、文档 |
| 关联入口 | 从 `docs/module-map.md` 找到必须检查的文件 |
| 已有实现 | 用 `rg` 找 handler/service/provider/page/type/test，而不是猜 |
| 约束 | MVP 范围、Provider 抽象、安全、队列、人工确认等 |
| 计划 | 2-5 个可执行步骤 |
| 验证方式 | 对应 `docs/task-checklist.md` 的最小检查 |
| 风险 | 未确认、未验证、需人工判断的点 |
| 经验沉淀 | 本次是否需要更新规则、pitfalls、PROGRESS 或模块文档 |

上下文包只保存“会影响下一步决策的信息”。读过但不再需要的日志、候选文件、失败假设和临时输出不应继续带入后续对话。

## 上下文工程预算

每次任务按“预算 → 检索 → 压缩 → 执行”的顺序管理上下文，避免越做越重。

| 阶段 | Token 目标 | 允许进入上下文 | 不应进入上下文 |
| --- | --- | --- | --- |
| 任务启动 | ≤ 1k | 用户目标、分支状态、改动类型、约束 | 全量 README、全量目录树 |
| 定位文件 | ≤ 2k | `rg` 结果、相关文件路径、关键符号 | 无关搜索结果、大段重复代码 |
| 读实现 | ≤ 6k | 相关函数、类型、配置片段、错误摘要 | 整个模块、完整构建日志 |
| 修改验证 | ≤ 4k | diff 摘要、失败测试关键行、验证命令 | 成功日志全文、无关 warning |
| 交付沉淀 | ≤ 2k | 改动摘要、验证结果、剩余风险、可复用经验 | 过程流水账 |

执行规则：

- 先用 `rg --files`、`rg -n`、`git diff --stat` 找入口，再局部读取。
- 同一事实只保留一次；如果已写入计划或文档，不在对话中反复展开。
- 对长文件按标题、函数名或行号读取；只在结构未知时读取全文。
- 对测试和构建输出只记录首个根因、相关文件行号、命令和最终状态。
- 当上下文开始变大时，先产出 5-8 条“当前事实摘要”，再继续下一步。

## 任务分流

| 任务类型 | 优先读取 | 常见同步 |
| --- | --- | --- |
| 后端 API / DTO | `docs/api.md`、对应 handler/service/model、前端 services/types | `docs/api.md`、Admin 页面、测试 |
| Provider | `docs/provider.md`、`docs/provider-template.md`、`backend/internal/providers` | 设置页、连接测试、脱敏展示、Provider 文档 |
| Admin 页面 | `admin/config/routes.ts`、页面、services、types、UI rules | README 能力描述、相关 docs |
| Collector | `collector/`、`docs/collector-1688-pitfalls.md`、采集 API | 后端 DTO、草稿映射、`docs/api.md` |
| 环境变量 | `.env.example`、`.env.docker.example`、`docs/env.md`、config 代码 | Docker、开发和部署文档 |
| Docker / CI | workflow、compose、Dockerfile、`docs/docker-deployment.md` | README、CONTRIBUTING、PR 模板 |
| 文档 / 规则 | `docs/README.md`、`AGENTS.md`、`.cursor/rules/README.md` | README / README.en 导航、相关 rule |

## Token 节约策略

- 优先用 `rg --files`、`rg -n`、`git diff --stat` 和局部 `Get-Content` / `sed` 读取，不粘贴大文件。
- 先读目录和符号，再读实现；先读最近相关文件，再扩大范围。
- 对大文档只读取相关章节；需要长期使用的结论写进计划或文档，不反复重读。
- 不把构建日志、测试日志和 API 响应原样塞进上下文，只保留错误摘要、文件行号和关键命令。
- 规则文件保持短而强，详细解释放到 `docs/`；Cursor `.mdc` 只保留高频约束和链接。
- 一个任务只维护一个当前计划；完成一步更新一步，避免长篇重复复述。

## 自动提示词优化

AI Agent 应把用户原始需求先压缩成“执行提示词”，再开始搜索和修改。执行提示词不是写给用户看的长说明，而是给当前任务使用的短指令。

### 标准改写模板

```text
目标：用一句话描述可验收结果。
任务类型：后端 / Admin / Collector / Provider / API / 配置 / Docker / CI / 文档。
范围边界：必须做什么；明确不做什么。
必读入口：AGENTS.md、docs/ai-workflow.md、docs/module-map.md、任务相关文件。
事实确认：需要用 rg / git status / 局部读取确认的字段、路由、命令或配置。
实现策略：2-5 步，优先复用现有分层、Provider、组件和文档结构。
验证：按 docs/task-checklist.md 选择最小检查命令。
沉淀：判断是否需要更新 docs / pitfalls / rules / PROGRESS。
```

### 改写规则

- 模糊需求先补齐验收结果：例如“优化 AI 工作流”应落到“更新文档与规则，使 AI 能自动做提示词改写、上下文预算和经验回写”。
- 大需求先拆 MVP 闭环：优先完成能验证的最小结果，不默认扩展完整 ERP。
- 如果用户指定路径、命令或平台，先验证它存在；不存在时记录假设并搜索邻近实现。
- 对高风险动作生成确认点：生产数据、密钥、付费 AI 调用、外部平台写操作、破坏性命令。
- 最终提示词保持短，不复制长文档；长规则通过链接引用。

### 需求澄清优先级

只有在无法安全推断时才追问。优先自己从仓库确认以下信息：

1. 路由、脚本、环境变量、Provider 名称、任务类型。
2. 现有页面、服务、类型、测试和文档入口。
3. 是否已有相同功能、相同 bug 或相同规则。

必须追问的情况：

- 目标会改变产品边界或引入重型能力。
- 同一需求存在互斥实现路径且都会影响用户数据。
- 需要真实密钥、账号、生产数据或外部平台写权限。

## 标准执行流程

1. **对齐目标**：确认任务是否属于当前两条主线，避免默认扩展完整 ERP。
2. **扫描上下文**：检查分支、未提交改动、模块映射和相关文件。
3. **形成计划**：列出影响范围、编辑文件和验证命令。
4. **实施改动**：保持小而聚焦，优先复用已有分层和 Provider 抽象。
5. **同步文档**：按 `docs/module-map.md` 更新相关文档和入口。
6. **执行验证**：按 `docs/task-checklist.md` 做最小必要检查。
7. **沉淀经验**：把可复用结论写到正确位置。
8. **最终说明**：说明改了什么、验证了什么、未验证原因和剩余风险。

## 质量门槛

为减少返工，AI Agent 在编辑前、验证前和交付前各做一次轻量自检。

| 时机 | 自检问题 |
| --- | --- |
| 编辑前 | 这个改动是否在用户目标内？是否查过 `docs/module-map.md`？是否会覆盖用户已有修改？ |
| 验证前 | 是否同步了 API / 类型 / 配置 / 文档入口？是否需要格式化或构建？ |
| 交付前 | 是否说明验证结果？未验证是否有原因？是否有可复用经验需要写回？ |

如果某个检查失败，先补齐再继续；如果因为环境限制无法补齐，在最终说明中明确写出。

## 自我成长机制

AI 不应把“成长”理解成在本地偷偷保存私有记忆；TradeMind 的成长应通过仓库可审计文档完成。自我成长遵循“观察 → 归纳 → 写回 → 下次复用”的闭环。

| 触发场景 | 写回位置 |
| --- | --- |
| 某类 Bug 第二次出现 | 对应 pitfalls 文档或模块文档 |
| 新增跨工具长期规则 | `AGENTS.md`、`docs/ai-coding-rules.md`，必要时同步 `.cursor/rules/` |
| Cursor 专属执行约束 | `.cursor/rules/*.mdc` 与 `.cursor/rules/README.md` |
| 阶段事实、已完成能力、遗留问题 | `docs/PROGRESS.md` |
| API / Provider / 队列 / 配置契约变化 | `docs/api.md`、`docs/provider.md`、`docs/env.md` |
| Prompt、AI 调用链路或质量门槛变化 | Prompt 模板、AI Provider 文档、相关任务文档 |
| PR 流程、检查命令或分支策略变化 | `docs/branching.md`、`CONTRIBUTING.md`、PR 模板 |

### 经验回写判定

满足任一条件时应写回，而不是只在聊天中说明：

- 同类问题已经出现第二次。
- 本次为了省 token 或提准确率形成了可复用步骤。
- 某个模块有新的质量门槛、回归命令或禁止做法。
- Prompt 模板、Provider 契约、AI 任务输入输出或 token 记录方式改变。
- 文档或规则之间出现冲突，需要建立优先级。

经验回写必须满足：

- 不记录真实密钥、Cookie、Token、客户数据、生产数据或私密对话。
- 不把一次性的个人偏好升级为全局规则。
- 新规则必须短、可执行、能降低后续误判或 token 消耗。
- 如果规则只适用于某个目录或技术栈，优先写成领域规则，避免污染所有任务。

### 自我进化输出格式

沉淀经验时优先使用以下格式，便于后续 AI 检索：

```text
触发：什么情况会用到这条经验。
规则：一句可执行约束。
验证：用什么命令、文件或页面确认。
位置：规则适用的目录、模块或任务类型。
```

## 多工具协作约定

- **Codex / Claude Code / 其他 Agent**：先读 `AGENTS.md`、本文和任务相关文档，再执行改动。
- **Cursor**：主要依赖 `.cursor/rules/*.mdc`，需要细节时跳转到 `docs/`；不要把本文完整复制到每条规则。
- **Copilot / Continue / Windsurf / Trae**：把 `AGENTS.md` 和本文作为项目说明入口，按任务类型读取最小相关文档。
- **人工开发者**：可把“最小上下文包”作为 Issue、PR 或交接说明模板。

## 常用提示模板

面向任意 AI 工具发起任务时，建议包含：

```text
目标：
影响范围：
必须遵守：
- 先读 AGENTS.md、docs/ai-workflow.md、docs/module-map.md
- 保持 MVP 范围，不引入重型 ERP 能力
- 先把需求改写成短执行提示词，再按最小上下文包读取文件
- 修改后按 docs/task-checklist.md 验证
期望输出：
- 改动摘要
- 验证结果
- 未验证原因和剩余风险
```

## Admin 文案与 UI 规范

管理端改动（页面、组件、样式）时，除 `docs/module-map.md` 外还应遵守以下约定：

| 资源 | 用途 |
| --- | --- |
| `docs/ui-copywriting.md` | 用户可见文案术语表、禁止项、`pnpm check:ui-copy` |
| `admin/src/constants/copywriting.ts` | 页面标题、说明、商品/平台/任务/库存统一术语 |
| `admin/src/constants/layoutTokens.ts` | 页面内边距、卡片间距、表单栅格间距 |
| `admin/src/constants/errorMessages.ts` | 错误码 → 用户可见提示（含操作建议） |
| `admin/src/constants/status.ts` | 状态文案与 Tag 颜色 |
| `admin/src/constants/userFriendly.ts` | 通用标签（规格、存储、运行时、接入方式等） |
| `admin/src/components/ui/` | PageContainer、SectionCard、FormGrid、EmptyState、TechnicalDetails、TaskJsonBlock 等 |

### 文案原则（摘要）

1. **面向用户，不面向开发者**：主界面不裸写 Provider、Worker、runtime、Storage、Stale、Endpoint 等；完整术语表见 **`docs/ui-copywriting.md`**。
2. **帮助文字**只回答：有什么用、填什么、填错会怎样。
3. **技术信息**（错误码、Request ID、原始 JSON）放入「技术详情」折叠区，默认收起；任务详情抽屉内 JSON 使用 `TechnicalDetails` + `TaskJsonBlock`。
4. **空状态**须包含：标题、原因、建议操作（可选按钮）。
5. **按钮**用「动词 + 对象」（如「保存设置」「测试连接」），避免裸「确定」「提交」。
6. **改动用户可见文案后**运行 `pnpm check:ui-copy --strict`（CI 同命令）；新增高频词同步 `userFriendly.ts` 与 `docs/ui-copywriting.md`。

### 布局原则（摘要）

- 页面容器：`padding-inline 24px`，`max-width 1440px`（看板 1680px）。
- 间距仅用：4 / 8 / 12 / 16 / 20 / 24 / 32 / 40。
- 配置表单桌面端两列，`<1100px` 单列；开关字段独立一行（名称 + 开关 + 说明）。
- 标题与说明分行；表格文本左对齐，金额/数量右对齐。

改动 Admin 页面时：先查是否已有 `PAGE_COPY` / 公共组件可复用，避免每页手写样式与重复术语。**页面容器统一使用 `TmPageContainer`**（勿直接使用 `PageContainer`），看板类页面可设置 `contentMaxWidth={layoutTokens.dashboardMaxWidth}`。

### 已落地的典型模式

| 场景 | 做法 |
| --- | --- |
| 任务详情抽屉 | 业务字段在 `Descriptions`；`input` / `output` / 原始 JSON 用 `TechnicalDetails` + `TaskJsonBlock` |
| 发布 / 库存 / 刊登说明 | 用户可读说明在外；API 参数名、预设键名折叠在 `TechnicalDetails` |
| 店铺授权表单 | OAuth 主流程外露；密钥覆盖、Token、卖家编号等收进 `TechnicalDetails` |
| 采集规则 / Prompt JSON | 编辑区整体包在 `TechnicalDetails`，并提示「一般无需修改」 |
| 状态 Tag | 优先 `constants/status.ts` 或 `commonStatusLabel()`，避免直接渲染英文枚举 |
| 平台 / 任务类型 | `platformLabel()`、`aiTaskTypeLabel()`、`taskTypeLabel()` 等 |
| 错误展示 | 主界面用 `formatUserErrorMessage()`；原始 `errorCode` 仅在技术详情区 |

### 常用辅助函数（`copywriting.ts` / `status.ts`）

- `commonStatusLabel` / `readinessLevelLabel` / `publishModeLabel`
- `collectTaskEventLabel` / `collectTaskStatusTransition`
- `AI_FIELD_COPY`（如 AI 优化标题 / 描述）

新增页面或抽屉时，先对照上表与 `PublishTasks`、`DraftDetail` 刊登 Tab 的实现，再写 UI。

## 完成标准

一次 AI 协作任务完成前至少确认：

- 已读当前任务相关的代码、配置和文档。
- 未覆盖用户已有修改。
- 已按 `docs/module-map.md` 检查关联内容。
- 已按 `docs/task-checklist.md` 执行或说明验证。
- 新增长期经验已写到合适文档，而不是只留在聊天里。

## Phase F1 全项目功能规划（2026-06-29）

**策略调整**：当前不进入最终人工验收 / 真实预发 / 抖店 E2E / 生产灰度；先按规划补齐全项目功能（F2–F8），再 Phase F9 统一总体验收。

开始 F2+ 功能开发前，Agent 应先读：

| 文档 | 用途 |
| --- | --- |
| [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md) | 34 模块完成度、页面/API/表、缺口 |
| [FULL_PROJECT_MVP_MAIN_FLOW.md](FULL_PROJECT_MVP_MAIN_FLOW.md) | 16 步主链路：入口、兜底、跳转 |
| [FULL_PROJECT_DEVELOPMENT_PLAN.md](FULL_PROJECT_DEVELOPMENT_PLAN.md) | F2–F9 阶段目标与边界 |
| [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md) | P0–P3 缺口优先级 |

**F 阶段任务分流**：

| 阶段 | 优先模块 | 禁止 |
| --- | --- | --- |
| F2 | 订单、异常工作台、SKU 匹配 | 售后/退款/财务 |
| F3 | 库存预警、扣减、平台同步 | 多仓 WMS / 自动补货 |
| F4 | 客服、AI 回复建议、人工发送 | 自动发送 |
| F5 | 配置状态中心、RBAC | 多租户 SSO |
| F6 | 总 Dashboard、全局体验 | 复杂 BI |
| F7 | Demo 数据全链路样本 | 真实平台数据 |
| F8 | 只修 P0/P1 | 新功能 |
| F9 | 人工走查、预发、抖店 E2E、灰度 | —（F8 后启动） |

**Phase F7（2026-06-30）**：全项目 Demo 数据升级完成。Demo 走查前先跑 [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md)；自动化回归用 [`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md)（Phase F7-Auto，**非**最终人工验收）。

缺口分级沿用 [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md)：**P0** 主链路断点 → **P1** 影响试用 → **P2** 体验 → **P3** 后续增强。
