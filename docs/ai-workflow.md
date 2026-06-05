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

## 标准执行流程

1. **对齐目标**：确认任务是否属于当前两条主线，避免默认扩展完整 ERP。
2. **扫描上下文**：检查分支、未提交改动、模块映射和相关文件。
3. **形成计划**：列出影响范围、编辑文件和验证命令。
4. **实施改动**：保持小而聚焦，优先复用已有分层和 Provider 抽象。
5. **同步文档**：按 `docs/module-map.md` 更新相关文档和入口。
6. **执行验证**：按 `docs/task-checklist.md` 做最小必要检查。
7. **沉淀经验**：把可复用结论写到正确位置。
8. **最终说明**：说明改了什么、验证了什么、未验证原因和剩余风险。

## 自我成长机制

AI 不应把“成长”理解成在本地偷偷保存私有记忆；TradeMind 的成长应通过仓库可审计文档完成。

| 触发场景 | 写回位置 |
| --- | --- |
| 某类 Bug 第二次出现 | 对应 pitfalls 文档或模块文档 |
| 新增跨工具长期规则 | `AGENTS.md`、`docs/ai-coding-rules.md`，必要时同步 `.cursor/rules/` |
| Cursor 专属执行约束 | `.cursor/rules/*.mdc` 与 `.cursor/rules/README.md` |
| 阶段事实、已完成能力、遗留问题 | `docs/PROGRESS.md` |
| API / Provider / 队列 / 配置契约变化 | `docs/api.md`、`docs/provider.md`、`docs/env.md` |
| Prompt、AI 调用链路或质量门槛变化 | Prompt 模板、AI Provider 文档、相关任务文档 |
| PR 流程、检查命令或分支策略变化 | `docs/branching.md`、`CONTRIBUTING.md`、PR 模板 |

经验回写必须满足：

- 不记录真实密钥、Cookie、Token、客户数据、生产数据或私密对话。
- 不把一次性的个人偏好升级为全局规则。
- 新规则必须短、可执行、能降低后续误判或 token 消耗。
- 如果规则只适用于某个目录或技术栈，优先写成领域规则，避免污染所有任务。

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
- 修改后按 docs/task-checklist.md 验证
期望输出：
- 改动摘要
- 验证结果
- 未验证原因和剩余风险
```

## 完成标准

一次 AI 协作任务完成前至少确认：

- 已读当前任务相关的代码、配置和文档。
- 未覆盖用户已有修改。
- 已按 `docs/module-map.md` 检查关联内容。
- 已按 `docs/task-checklist.md` 执行或说明验证。
- 新增长期经验已写到合适文档，而不是只留在聊天里。
