# AI 编程规则与文档同步要求

本文用于约束使用 AI Agent、Cursor、Copilot 或人工协作开发 TradeMind 时的基本工程规则。核心原则是：**代码、配置、文档、示例和 CI 必须保持同步**。

## 基本原则

- 先理解现有模块边界，再修改代码。
- 优先沿用仓库已有架构和 Provider 抽象，不绕过业务分层。
- 不把 API Key、Token、Secret、Cookie、密码写入代码、日志、示例或截图。
- 不默认引入重型架构，MVP 阶段优先保持可运行、可部署、可维护。
- 修改代码时同步考虑测试、构建、部署、文档和示例配置。
- 对任何跨模块改动，先查 [module-map.md](module-map.md)，再决定需要同步哪些文件。

## 必须同步更新文档的场景

以下任一变更发生时，必须同步更新相关文档：

| 变更类型 | 必须检查 / 更新 |
| --- | --- |
| 新增或修改启动命令 | `README.md`、`README.en.md`、`docs/development.md`、`package.json` 脚本说明 |
| 新增或修改 Docker 部署 | `README.md`、`README.en.md`、`docs/docker-deployment.md`、`.env.docker.example` |
| 新增或修改环境变量 | `.env.example`、`.env.docker.example`、`docs/env.md`、`docs/development.md`、`docs/docker-deployment.md` |
| 新增 API 或改变 API 契约 | `docs/api.md`、前端 `services` / `types`、README 中的能力描述 |
| 新增 Provider | `docs/provider.md`、`docs/provider-template.md`、README 功能表、设置页面说明、示例配置 |
| 新增后台页面或路由 | README 能力描述、相关 `docs/`、菜单 / 路由说明 |
| 新增异步任务或队列 | `.env.example`、健康检查说明、任务中心 / Worker 相关文档 |
| 新增数据库表或关键字段 | `docs/PROGRESS.md`、架构 / 模块文档、必要时补迁移说明 |
| 修改分支、CI、PR 流程 | `docs/branching.md`、`CONTRIBUTING.md`、PR 模板 |
| 修改安全、密钥、授权逻辑 | `SECURITY.md`、`.env.example`、相关设置文档 |

## 配置文件同步规则

涉及配置时必须遵守：

1. 新增环境变量时，同时更新 `.env.example`。
2. Docker 部署也需要该变量时，同时更新 `.env.docker.example` 和 `docker-compose.full.yml`。
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

1. 先读取相关代码、配置和文档，不凭空假设脚本、端口、路径或变量。
2. 只修改与任务相关的文件，不顺手重构无关模块。
3. 业务能力走既有分层：handler → service → provider / repository / queue。
4. 涉及 AI、存储、图片、平台、采集能力时，优先通过 Provider 接口扩展。
5. 涉及耗时任务时，使用任务状态和队列，不在请求中长时间同步阻塞。
6. 涉及密钥时，走加密、脱敏和日志保护。
7. 完成后按 [task-checklist.md](task-checklist.md) 执行与改动匹配的检查，并说明未执行的原因。

## 提交前检查清单

- [ ] 代码与现有架构一致。
- [ ] 没有提交 `.env`、密钥、Token、Cookie 或真实平台凭证。
- [ ] 新增 / 修改配置已同步 `.env.example`、`.env.docker.example` 和相关文档。
- [ ] 新增 / 修改命令已同步 README 和开发文档。
- [ ] 新增 / 修改 Docker 行为已同步 Docker 文档。
- [ ] 新增 / 修改 API、Provider、任务或页面已同步相关 docs。
- [ ] 后端 API / DTO 变化已同步前端 services / types 与 `docs/api.md`。
- [ ] 已按 `docs/module-map.md` 检查关联文件。
- [ ] 涉及后端 Go 代码时已执行 `go fmt ./...`。
- [ ] 涉及 admin 时已执行或说明 `pnpm build:admin`。
- [ ] 涉及 collector 时已执行或说明 `pnpm build:collector`。
- [ ] 较大模块或阶段性变更已更新 `docs/PROGRESS.md`。
