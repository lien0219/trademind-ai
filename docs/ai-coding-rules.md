# AI 编程规则与文档同步要求

本文用于约束使用 AI Agent、Cursor、Copilot 或人工协作开发 TradeMind 时的基本工程规则。核心原则是：**代码、配置、文档、示例和 CI 必须保持同步**。更完整的跨工具执行流程、最小上下文包、token 节约和经验沉淀机制见 [ai-workflow.md](ai-workflow.md)。

## 基本原则

- 先理解现有模块边界，再修改代码。
- 优先沿用仓库已有架构和 Provider 抽象，不绕过业务分层。
- 不把 API Key、Token、Secret、Cookie、密码写入代码、日志、示例或截图。
- 不默认引入重型架构，MVP 阶段优先保持可运行、可部署、可维护。
- 修改代码时同步考虑测试、构建、部署、文档和示例配置。
- 对任何跨模块改动，先查 [module-map.md](module-map.md)，再决定需要同步哪些文件。

## 三方体验与功能保护

每次修改（包括功能、修复、重构、配置和文档）必须同时改善用户体验、开发体验和 AI Agent 的使用体验，三者都要变好，并且不能破坏现有功能。不得以某一方的收益抵消另一方的退步。

| 维度 | 必须明确的改善 |
| --- | --- |
| 用户体验 | 任务更易完成、信息更易理解，或正确性、可靠性与反馈更好。 |
| 开发体验 | 代码或规则更易理解、修改、调试和验证，减少重复实现与维护成本。 |
| AI Agent 使用体验 | 入口与边界更清楚，命名、契约和文档更一致，减少猜测、无关上下文与误操作。 |

编辑前明确三方各自的改善目标；交付时分别说明具体改善、验证依据和未验证项。只写“无影响”或笼统的“更友好”不能作为改善依据，也不得为满足本规则扩大无关重构。

不得以体验优化为由破坏既有功能与兼容性，或擅自改变 API 契约、权限、readonly、状态机和业务语义。按影响范围使用现有回归保护；本地检查、交由 CI 的测试与人工验收结果分别说明，不得将未执行的验证写成通过。

界面精简、布局重设计与首次交付视觉质量遵循 [Admin UI 主规范](../.agents/skills/frontend-design/SKILL.md) 的文案规范及第 3.1、5 节；自检应覆盖整体构图、宽屏空间利用与短表单的内容轨道，避免大片无意义单侧空白。删除冗余文案不能删掉必要业务信息、操作入口或可访问性支持。

## 生产维护测试规则

- GitHub Actions 是自动化回归的唯一持续执行入口；必须保留工作流依赖的核心测试、fixture、Mock 和配置。
- 本地默认执行相关静态检查、配置验证和必要构建，完整自动化回归交由 CI；未本地执行的测试不得声称已通过。
- 产品、页面与业务流程由维护者人工验收并记录结果。
- 本地测试数据库可不存在；数据库和 Redis 集成测试只能使用显式隔离资源或 CI service container。
- 不创建阶段/批次 gate、长期运行证据、一次性报告或 `artifacts/`；本地 Playwright/测试产物完成诊断后清理。

## 必须同步更新文档的场景

以下任一变更发生时，必须同步更新相关文档：

| 变更类型 | 必须检查 / 更新 |
| --- | --- |
| 新增或修改启动命令 | `README.md`、`README.en.md`、`docs/development.md`、`package.json` 脚本说明 |
| 新增或修改 Docker 部署 | `README.md`、`README.en.md`、`docs/docker-deployment.md`、`.env.example` |
| 新增或修改环境变量 | `.env.example`、`docs/env.md`、`docs/development.md`、`docs/docker-deployment.md` |
| 新增 API 或改变 API 契约 | `docs/api.md`、前端 `services` / `types`、README 中的能力描述 |
| 新增 Provider | `docs/provider.md`、`docs/provider-template.md`、README 功能表、设置页面说明、示例配置 |
| 新增后台页面或路由 | README 能力描述、相关 `docs/`、菜单 / 路由说明 |
| 新增异步任务或队列 | `.env.example`、健康检查说明、任务中心 / Worker 相关文档 |
| 新增数据库表或关键字段 | `docs/PROGRESS.md`、架构 / 模块文档、必要时补迁移说明和 CI 回归 |
| 修改分支、CI、PR 流程 | `docs/branching.md`、`CONTRIBUTING.md`、PR 模板 |
| 修改安全、密钥、授权逻辑 | `SECURITY.md`、`.env.example`、相关设置文档 |

## 配置文件同步规则

涉及配置时必须遵守：

1. 新增环境变量时，同时更新 `.env.example`。
2. Docker 部署也需要该变量时，同时更新唯一模板 `.env.example` 和 `docker-compose.full.yml`。
3. 修改默认端口、默认路径、默认服务名时，同时更新 README、开发文档和 Docker 文档。
4. 新增敏感配置时，必须说明是否加密存储、是否脱敏展示、是否禁止写入日志。
5. 删除或重命名配置时，必须检查脚本、CI、Docker、文档和后台设置页。

详细环境变量说明维护在 [env.md](env.md)。

## 关联内容检查规则

AI Agent 处理任务时必须先判断改动类型，并按 [module-map.md](module-map.md) 检查关联内容：

1. 后端 DTO / API 变化：同步前端 `services`、`types`、页面字段和 [api.md](api.md)。
2. 环境变量变化：同步 env 模板、Docker Compose、开发 / 部署文档和 [env.md](env.md)。
3. Provider 变化：同步 settings、连接测试、脱敏展示、[provider.md](provider.md) 和 [provider-template.md](provider-template.md)。
4. CI / 分支变化：同步 workflow、[branching.md](branching.md)、`CONTRIBUTING.md` 和 PR 模板。
5. 开源治理变化：同步 README、文档中心、`CHANGELOG.md` 和 `.github/` 配置。

## AI Agent 工作流程

AI Agent 修改代码时应遵循：

1. 先把用户需求改写为短执行提示词：目标、任务类型、范围边界、必读入口、事实确认、实现策略、验证和沉淀。
2. 再按 [ai-workflow.md](ai-workflow.md) 形成最小上下文包：任务目标、改动类型、关联入口、已有实现、验证方式和风险。
3. 按上下文预算读取文件：先搜索和局部读取，再扩大范围；不把大文件、大日志和无关上下文直接塞给模型。
4. 先读取相关代码、配置和文档，不凭空假设脚本、端口、路径或变量。
5. 只修改与任务相关的文件，不顺手重构无关模块。
6. 业务能力走既有分层：handler → service → provider / repository / queue。
7. 涉及 AI、存储、图片、平台、采集能力时，优先通过 Provider 接口扩展。
8. 涉及耗时任务时，使用任务状态和队列，不在请求中长时间同步阻塞。
9. 涉及密钥时，走加密、脱敏和日志保护。
10. 完成后按 [task-checklist.md](task-checklist.md) 执行与改动匹配的本地检查，列出交由 CI 的回归和人工验收结果。
11. 对重复问题、架构决策、工具约定、Prompt 模板或质量门槛，按 [ai-workflow.md](ai-workflow.md) 写回对应文档，让后续 AI 工具可复用。

## 提交前检查清单

- [ ] 代码与现有架构一致。
- [ ] 已将用户需求压缩成短执行提示词，并按上下文预算读取相关文件。
- [ ] 已按 `docs/ai-workflow.md` 控制上下文范围，并沉淀必要经验。
- [ ] 没有提交 `.env`、密钥、Token、Cookie 或真实平台凭证。
- [ ] 新增 / 修改配置已同步唯一模板 `.env.example` 和相关文档。
- [ ] 新增 / 修改命令已同步 README 和开发文档。
- [ ] 新增 / 修改 Docker 行为已同步 Docker 文档。
- [ ] 新增 / 修改 API、Provider、任务或页面已同步相关 docs。
- [ ] 后端 API / DTO 变化已同步前端 services / types 与 `docs/api.md`。
- [ ] 已按 `docs/module-map.md` 检查关联文件。
- [ ] 涉及后端 Go 代码时已执行 `go fmt ./...`。
- [ ] 涉及 admin 时已执行或说明 `pnpm build:admin`。
- [ ] 涉及用户可见文案时已执行或说明 `pnpm check:ui-copy --strict`。
- [ ] 涉及 collector 时已执行或说明 `pnpm build:collector`。
- [ ] 较大模块或维护策略变更已更新 `docs/PROGRESS.md`。
