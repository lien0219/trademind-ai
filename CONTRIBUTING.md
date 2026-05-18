# Contributing to TradeMind

感谢你愿意参与贸灵 TradeMind。任何形式的贡献都欢迎：Bug 反馈、功能建议、文档改进、Provider 接入、采集规则优化、Prompt 模板优化、Docker 部署完善等。

## 参与方式

1. Fork 本仓库。
2. 从 `dev` 创建功能分支。
3. 在本地完成修改并补充必要测试或文档。
4. 提交 Pull Request 到 `dev`，并按模板填写变更说明。

```bash
git switch dev
git pull --ff-only origin dev
git switch -c feat/your-feature-name
```

分支策略与 PR 合并规则见 [docs/branching.md](docs/branching.md)。

## 本地启动

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

常用命令：

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm dev:stop
```

更多说明见 [docs/development.md](docs/development.md)。

## Commit Message 建议

建议使用清晰、简短的提交信息：

```text
feat: add storage provider docs
fix: handle collector timeout error
docs: improve docker deployment guide
chore: update issue templates
```

常见类型：

- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档
- `refactor`: 重构
- `test`: 测试
- `chore`: 工程维护

## Pull Request 要求

提交 PR 前请确认：

- 变更范围清晰，避免混入无关修改。
- 目标分支符合 [docs/branching.md](docs/branching.md)：`feat/*` 与普通 `fix/*` 先合并到 `dev`，`release/*` 再合并到 `main`。
- 已阅读并遵守 [docs/ai-coding-rules.md](docs/ai-coding-rules.md) 与 [docs/module-map.md](docs/module-map.md)，代码、配置、示例和文档需要同步更新。
- 已按 [docs/task-checklist.md](docs/task-checklist.md) 完成对应范围的收尾检查。
- 涉及后端 Go 代码时已在 `backend` 目录执行 `go fmt ./...`。
- 涉及前端或 Collector 时已执行相关构建或说明未执行原因。
- 涉及接口、部署、环境变量、配置文件或 Provider 机制时同步更新文档。
- 不提交 `.env`、密钥、Token、Cookie、真实平台凭证。

## Issue 模板说明

提交 Issue 时请优先选择：

- Bug Report：报告问题或异常行为。
- Feature Request：提出功能建议。
- Documentation：反馈文档问题。

请尽量提供复现步骤、环境信息、截图、日志和期望行为，这会显著提升处理效率。

## 代码风格

- 后端：Go + Gin + GORM，业务模块通过 service 编排，第三方能力必须走 Provider。
- 前端：admin 使用 React + TypeScript + Ant Design Pro，表格优先 ProTable，表单优先 ProForm。
- 采集服务：Node.js + TypeScript + Playwright，采集服务不直接操作主业务数据库。
- 数据库：默认 PostgreSQL；MySQL 仅作为可选兼容路径。
- 安全：日志与文档中不要输出完整 API Key、Token、Secret、密码或 Cookie。

## 文档贡献

文档贡献同样重要。你可以改进：

- README 首页表达
- 本地开发说明
- Docker 部署说明
- Provider 扩展文档
- 路线图与模块设计说明
- 示例配置与故障排查

新增核心模块、公共契约或部署方式时，请同步更新 `README.md`、`docs/` 下相关文档，并在必要时更新 `docs/PROGRESS.md`、`CHANGELOG.md`。

配置文件变更也必须同步文档：新增或修改环境变量时更新 `.env.example` 与 [docs/env.md](docs/env.md)；Docker 部署也需要时更新 `.env.docker.example` 与 `docker-compose.full.yml`；命令、端口、路径或服务名变化时同步 README 与对应 docs。

如果不确定关联范围，先查 [docs/module-map.md](docs/module-map.md)。

## Open Source Usage

TradeMind is licensed under Apache-2.0. Derived works must keep the original license and clearly mention the original project repository:

<https://github.com/lien0219/trademind-ai>
