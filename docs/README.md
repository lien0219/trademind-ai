# TradeMind 文档中心

TradeMind 是一个聚焦 `AI 商品运营工具` 与 `多平台跨境 ERP MVP` 的开源平台。仓库首页负责展示项目定位与产品预览，`docs/` 负责承载开发、部署、架构、协作与维护细节。

如果你是第一次进入这个项目，建议先按角色找到入口，而不是从头顺着文件名阅读。

## 按角色开始

| 你现在想做什么 | 建议先看 |
| --- | --- |
| 我想快速了解项目能做什么 | [../README.md](../README.md) · [roadmap.md](roadmap.md) · [PROGRESS.md](PROGRESS.md) |
| 我想本地跑起来或用 Docker 试用 | [development.md](development.md) · [docker-deployment.md](docker-deployment.md) · [env.md](env.md) |
| 我想接 API、改功能、扩 Provider | [architecture.md](architecture.md) · [api.md](api.md) · [provider.md](provider.md) |
| 我想参与协作或用 AI 工具开发 | [../AGENTS.md](../AGENTS.md) · [ai-workflow.md](ai-workflow.md) · [module-map.md](module-map.md) |

## 核心入口

| 文档 | 说明 | 适合谁 |
| --- | --- | --- |
| [development.md](development.md) | 本地开发、常用命令、端口、环境变量 | 开发者 |
| [docker-deployment.md](docker-deployment.md) | Docker Compose 完整部署、端口、日志、数据卷 | 试用者 / 部署者 |
| [ai-workflow.md](ai-workflow.md) | 跨 AI 工具通用工作流、提示词优化、上下文预算、token 节约和经验沉淀 | 开发者 / AI Agent |
| [ui-copywriting.md](ui-copywriting.md) | 管理端/API 用户可见文案中文化、术语表与 `pnpm check:ui-copy` | 开发者 / AI Agent |
| [env.md](env.md) | 环境变量清单、敏感配置、安全规则与同步要求 | 开发者 / 部署者 / AI Agent |
| [api.md](api.md) | API 公共约定、主要路由与前后端契约同步要求 | 前后端开发者 |
| [architecture.md](architecture.md) | 总体架构、分层、数据与队列、安全原则 | 开发者 / 架构维护者 |
| [provider.md](provider.md) | AI / Storage / Image / Platform / Collector Provider 扩展机制 | Provider 贡献者 |
| [collector-1688-pitfalls.md](collector-1688-pitfalls.md) | 1688 采集已知 bug、防复发约束与回归命令 | Collector / AI Agent |
| [custom-collect-rules.md](custom-collect-rules.md) | 自定义链接采集规则 JSON、API 与错误码 | collect / admin / Collector |
| [github-repo-presentation.md](github-repo-presentation.md) | GitHub 仓库首页、About、Topics、Social Preview 配置清单 | 维护者 / 开源协作者 |
| [open-source-presentation-checklist.md](open-source-presentation-checklist.md) | 开源展示发布前自检：README、About、Topics、头图与文档入口一致性 | 维护者 / 开源协作者 |
| [AI_PRODUCT_OPERATION_UX_AUDIT.md](AI_PRODUCT_OPERATION_UX_AUDIT.md) / [AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md](AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md) | AI 商品运营体验审计、A1.1 稳定性补强与验收状态 | 产品 / 前后端 / AI Agent |
| [module-map.md](module-map.md) | 模块关联索引，说明改 A 时要检查哪些 B / C / D | 开发者 / AI Agent |
| [roadmap.md](roadmap.md) | 版本路线图与阶段目标 | 所有人 |

## 文档分层

| 层级 | 作用 |
| --- | --- |
| `README.md` / `README.en.md` | 对外首页：项目定位、能力概览、界面预览、快速开始。 |
| `docs/README.md` | 文档导航首页：帮助不同角色找到正确入口。 |
| `docs/*.md` | 详细规则与实现说明：开发、部署、架构、契约、协作。 |
| `.cursor/rules/` 与 `AGENTS.md` | AI 协作规则入口，约束工程实践与文档同步。 |

## 开发与部署

| 文档 | 内容 |
| --- | --- |
| [development.md](development.md) | 本地开发环境、`pnpm dev`、分服务启动、调试与故障排查 |
| [docker-deployment.md](docker-deployment.md) | `docker-compose.full.yml`、生产前安全配置、日志与数据管理 |
| [env.md](env.md) | `.env.example`、`.env.docker.example`、Docker 端口、队列变量和敏感配置说明 |

## 架构与扩展

| 文档 | 内容 |
| --- | --- |
| [architecture.md](architecture.md) | Go backend、React admin、Node collector、PostgreSQL、Redis 的整体关系 |
| [api.md](api.md) | `/api/v1` API 契约、统一返回、鉴权与前后端同步要求 |
| [provider.md](provider.md) | Provider 抽象、扩展建议、安全要求 |
| [provider-template.md](provider-template.md) | 新增 Provider 时的接口、配置、错误处理、安全与文档模板 |
| [collector-1688-pitfalls.md](collector-1688-pitfalls.md) | 1688 采集已知问题、禁止做法与回归检查 |
| [roadmap.md](roadmap.md) | AI 商品运营工具、多平台 ERP MVP、完整 ERP 增强的推进顺序 |

## 协作与工程规则

| 文档 | 内容 |
| --- | --- |
| [branching.md](branching.md) | `main` / `dev` / `feat/*` / `fix/*` / `release/*` 分支策略与 PR 规则 |
| [ai-workflow.md](ai-workflow.md) | Codex、Cursor 等 AI 工具的通用执行流程、提示词优化、上下文控制和自我成长机制 |
| [ai-coding-rules.md](ai-coding-rules.md) | AI 编程规则、配置文件与文档同步要求 |
| [module-map.md](module-map.md) | 模块关联索引，避免代码、配置、文档、CI 漏同步 |
| [task-checklist.md](task-checklist.md) | 按任务类型收尾自查：Go、Admin、Collector、环境变量、API、Provider、Docker、CI |
| [cursor-rules-usage.md](cursor-rules-usage.md) | Cursor rules 使用说明 |
| [../AGENTS.md](../AGENTS.md) | 通用 AI Agent 协作入口 |
| [../.cursor/rules/README.md](../.cursor/rules/README.md) | Cursor rules 索引 |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | 贡献指南、提交建议、PR 要求 |

## 社区与治理

| 文档 | 内容 |
| --- | --- |
| [sponsor.md](sponsor.md) | 微信 / 支付宝赞助入口与赞助榜 |
| [../SECURITY.md](../SECURITY.md) | 安全漏洞披露与部署安全建议 |
| [../CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) | 社区行为准则 |
| [../NOTICE](../NOTICE) | 第三方声明、商标和致谢 |
| [../LICENSE](../LICENSE) | Apache-2.0 开源协议 |
| [../CHANGELOG.md](../CHANGELOG.md) | 版本与重要变更记录 |

## 仓库关键文件说明

| 路径 | 作用 |
| --- | --- |
| `.github/CODEOWNERS` | 定义关键目录负责人，PR 改动匹配路径时请求维护者 review。 |
| `.github/dependabot.yml` | 自动检查 GitHub Actions、pnpm、Go modules、Docker 依赖更新。 |
| `.github/labeler.yml` | 按改动路径为 PR 自动打 `area:*`、`needs:*` 标签。 |
| `.github/workflows/` | Go / Node / Docker 配置检查 / PR Labeler 等 GitHub Actions。 |
| `.github/ISSUE_TEMPLATE/` | Bug、Feature、Documentation Issue 模板与入口配置。 |
| `.github/PULL_REQUEST_TEMPLATE.md` | PR 描述、测试方式、目标分支与关联内容同步清单。 |
| `.cursor/rules/` | Cursor / AI Agent 项目级持久规则。 |
| `AGENTS.md` | 通用 AI 编程工具协作入口，适用于 Cursor 以外的 Agent。 |
| `CHANGELOG.md` | 版本与重要变更记录。 |
| `.env.example` | 本地开发环境变量模板。 |
| `.env.docker.example` | Docker 完整部署环境变量模板。 |
| `docker-compose.yml` | 本地开发基础设施：PostgreSQL + Redis。 |
| `docker-compose.full.yml` | 完整 Docker 部署编排：PostgreSQL + Redis + backend + admin + collector。 |

## 维护规则

新增或修改功能时，请同步检查：

- 启动命令变化：更新 `README.md`、`README.en.md`、`development.md`。
- Docker 行为变化：更新 `docker-deployment.md`、`.env.docker.example`。
- 环境变量变化：更新 `.env.example`、必要时更新 `.env.docker.example`，并同步 [env.md](env.md)。
- API / Provider / 队列 / 数据库变化：更新 [api.md](api.md)、[provider.md](provider.md)、[module-map.md](module-map.md) 或对应架构文档。
- 分支、CI、PR 流程变化：更新 `branching.md`、`CONTRIBUTING.md`、PR 模板。
- 较大模块或阶段性变更：更新 [PROGRESS.md](PROGRESS.md)。

详细规则见 [ai-coding-rules.md](ai-coding-rules.md)。
