# AGENTS.md

本文件是 TradeMind 给 AI 编程工具和协作开发者的通用入口。无论使用 Cursor、Claude Code、Copilot、Continue、Windsurf、Trae 或其他 AI 编辑器，都应先阅读本文件和下方必读文档。

## 项目定位

贸灵 TradeMind 是一个开源 AI 跨境电商运营平台，当前优先服务两条主线：

1. **AI 商品运营工具**
2. **多平台跨境 ERP MVP**

不要主动把项目扩展成重型完整 ERP。多仓、采购、财务、WMS / OMS、复杂 BI 等能力后置。

## 必读文档

开始开发前请阅读：

| 文档 | 用途 |
| --- | --- |
| [README.md](README.md) | 项目首页、能力概览、启动方式 |
| [docs/README.md](docs/README.md) | 文档中心 |
| [docs/ai-coding-rules.md](docs/ai-coding-rules.md) | AI 编程规则与文档同步要求 |
| [docs/branching.md](docs/branching.md) | 分支策略与 PR 规则 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献指南 |
| [docs/PROGRESS.md](docs/PROGRESS.md) | 当前进度、已完成事项、遗留问题 |
| [.cursor/rules/README.md](.cursor/rules/README.md) | Cursor rules 索引，可作为更细的工程规则参考 |

## 技术栈

- 后端：Go + Gin + GORM
- 管理端：React + TypeScript + Ant Design Pro
- 采集服务：Node.js + TypeScript + Playwright
- 数据库：PostgreSQL 默认
- 队列 / 缓存：Redis
- 包管理：pnpm workspace
- 部署：Docker Compose

## 开发规则

- 不直接在 `main` 上开发。
- 日常功能从 `dev` 创建 `feat/*` 或 `fix/*` 分支。
- 功能完成后 PR 到 `dev`，稳定后再从 `dev` PR 到 `main`。
- 后端业务遵循 handler → service → provider / repository / queue 分层。
- AI、存储、图片、平台、采集能力必须优先通过 Provider 抽象扩展。
- 耗时任务必须使用任务状态和队列，不要在 HTTP 请求里长时间同步阻塞。
- 敏感配置必须加密存储、脱敏展示，日志中不得输出完整密钥或 Token。

## 文档同步要求

代码变更必须同步相关文档：

- 新增环境变量：更新 `.env.example`。
- Docker 也需要该变量：更新 `.env.docker.example` 和 `docker-compose.full.yml`。
- 修改启动命令：更新 `README.md`、`README.en.md`、`docs/development.md`。
- 修改 Docker 部署：更新 `docs/docker-deployment.md`。
- 新增 API / Provider / 队列 / 页面 / 数据表：更新对应 `docs/`。
- 较大模块或阶段性变更：更新 `docs/PROGRESS.md`。
- 分支、CI、PR 流程变更：更新 `docs/branching.md`、`CONTRIBUTING.md`、PR 模板。

## 检查命令

按改动范围执行：

```bash
pnpm check:dev
pnpm build:admin
pnpm build:collector
```

修改后端 Go 代码时，在 `backend` 目录执行：

```bash
go fmt ./...
go test ./...
```

如果没有执行某项检查，需要在最终说明或 PR 中写明原因。

## 禁止事项

- 不提交 `.env`、真实密钥、Token、Cookie、平台凭证或生产数据。
- 不把第三方平台逻辑写进核心业务层。
- 不让前端直接调用第三方 AI、平台或存储 API。
- 不默认引入 Kubernetes、Kafka、复杂微服务治理等重型架构。
- 不在 MVP 阶段默认实现 AI 客服自动外发，必须人工确认。

## 给 AI Agent 的工作方式

1. 先读取相关代码、配置和文档。
2. 明确影响范围，再编辑文件。
3. 保持修改小而聚焦。
4. 不回滚用户已有修改，除非用户明确要求。
5. 修改代码后检查文档、配置、CI 是否需要同步。
6. 最终说明改了什么、验证了什么、还有什么风险。
