# TradeMind 文档中心

这里是 TradeMind 的文档入口。README 只保留项目首页重点信息；更细的开发、部署、架构、协作和维护规则统一放在 `docs/` 下维护。

## 快速入口

| 文档 | 说明 | 适合谁 |
| --- | --- | --- |
| [development.md](development.md) | 本地开发、常用命令、端口、环境变量 | 开发者 |
| [docker-deployment.md](docker-deployment.md) | Docker Compose 完整部署、端口、日志、数据卷 | 试用者 / 部署者 |
| [architecture.md](architecture.md) | 总体架构、分层、数据与队列、安全原则 | 开发者 / 架构维护者 |
| [provider.md](provider.md) | AI / Storage / Image / Platform / Collector Provider 扩展机制 | Provider 贡献者 |
| [roadmap.md](roadmap.md) | 版本路线图与阶段目标 | 所有人 |

## 开发与部署

| 文档 | 内容 |
| --- | --- |
| [development.md](development.md) | 本地开发环境、`pnpm dev`、分服务启动、调试与故障排查 |
| [docker-deployment.md](docker-deployment.md) | `docker-compose.full.yml`、生产前安全配置、日志与数据管理 |

## 架构与扩展

| 文档 | 内容 |
| --- | --- |
| [architecture.md](architecture.md) | Go backend、React admin、Node collector、PostgreSQL、Redis 的整体关系 |
| [provider.md](provider.md) | Provider 抽象、扩展建议、安全要求 |
| [roadmap.md](roadmap.md) | AI 商品运营工具、多平台 ERP MVP、完整 ERP 增强的推进顺序 |

## 协作与工程规则

| 文档 | 内容 |
| --- | --- |
| [branching.md](branching.md) | `main` / `dev` / `feat/*` / `fix/*` / `release/*` 分支策略与 PR 规则 |
| [ai-coding-rules.md](ai-coding-rules.md) | AI 编程规则、配置文件与文档同步要求 |
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

## 维护规则

新增或修改功能时，请同步检查：

- 启动命令变化：更新 `README.md`、`README.en.md`、`development.md`。
- Docker 行为变化：更新 `docker-deployment.md`、`.env.docker.example`。
- 环境变量变化：更新 `.env.example`，必要时更新 `.env.docker.example`。
- API / Provider / 队列 / 数据库变化：更新对应架构或 Provider 文档。
- 分支、CI、PR 流程变化：更新 `branching.md`、`CONTRIBUTING.md`、PR 模板。
- 较大模块或阶段性变更：更新 [PROGRESS.md](PROGRESS.md)。

详细规则见 [ai-coding-rules.md](ai-coding-rules.md)。
