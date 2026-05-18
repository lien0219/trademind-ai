<h1 align="center" id="贸灵-trademind">贸灵 TradeMind</h1>

<p align="center">
  <strong>开源 AI 跨境电商运营平台</strong>
</p>

<p align="center">
  商品采集 · 商品草稿 · AI 标题优化 · AI 描述生成 · 图片管理 · 店铺授权 · 订单同步 · AI 客服建议
</p>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-18+-61DAFB?logo=react&logoColor=111">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5+-3178C6?logo=typescript&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white">
  <img alt="pnpm" src="https://img.shields.io/badge/pnpm-9.15+-F69220?logo=pnpm&logoColor=white">
  <img alt="PRs Welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg">
  <img alt="Stars Welcome" src="https://img.shields.io/badge/Stars-welcome-yellow.svg">
</p>

<p align="center">
  简体中文 | <a href="README.en.md">English</a>
</p>

> 贸灵 TradeMind 是一个面向跨境电商卖家的开源 AI 运营工具，当前支持商品采集、商品草稿、AI 标题优化、AI 描述生成、图片管理、AI 图片任务、店铺授权、订单同步、SKU 匹配、商品刊登、库存同步和 AI 客服建议等运营能力。

## 项目截图 / Demo

> Demo 与截图素材预留中，欢迎提交 PR 完善首页展示。

| 管理后台 | 商品草稿 | AI 商品优化 |
| --- | --- | --- |
| Coming soon | Coming soon | Coming soon |

## 目录

- [项目介绍](#项目介绍)
- [为什么做这个项目](#为什么做这个项目)
- [核心功能](#核心功能)
- [产品能力地图](#产品能力地图)
- [快速开始](#快速开始)
- [Docker 部署启动](#docker-部署启动)
- [本地开发启动](#本地开发启动)
- [环境变量说明](#环境变量说明)
- [项目结构](#项目结构)
- [技术架构](#技术架构)
- [当前开发优先级](#当前开发优先级)
- [路线图 Roadmap](#路线图-roadmap)
- [文档导航](#文档导航)
- [合作商展示](#合作商展示)
- [贡献榜](#贡献榜)
- [赞助榜](#赞助榜)
- [开源使用规范](#开源使用规范)
- [贡献指南](#贡献指南)
- [赞助支持](#赞助支持)
- [License](#license)
- [致谢](#致谢)

## 项目介绍

贸灵 TradeMind 是一个开源 AI 跨境电商运营平台，面向需要高效完成商品上新、内容优化、图片处理、店铺运营和订单处理的跨境电商卖家与开发团队。

当前项目已经围绕商品运营链路提供一组可运行的能力：采集商品链接后生成商品草稿，维护 SKU 与商品图片，调用 AI 生成标题和描述，执行图片处理任务，配置多平台店铺，拉取订单，进行 SKU 匹配、库存同步、商品刊登，并在客服场景中生成 AI 建议回复。项目通过 Provider 抽象接入 AI、存储、图片处理、采集源和跨境平台，便于私有化部署和二次开发。

```text
商品采集 → 商品草稿 → AI 标题优化 → AI 描述生成 → 图片管理
  → AI 图片处理 → 店铺授权 → 商品刊登 → 订单同步
  → SKU 匹配 → 库存同步 → AI 客服建议
```

## 为什么做这个项目

跨境卖家的日常运营中存在大量重复工作：采集商品、整理标题、生成多语言描述、处理商品图、维护平台店铺、同步订单、回复买家消息。传统 ERP 更偏数据录入与流程管理，而 TradeMind 更强调把 AI 能力嵌入商品运营和跨平台协作流程。

TradeMind 希望提供一个开源、可部署、可二次开发的基础平台，让个人卖家、运营团队和开发者都能围绕自己的业务流程接入 AI Provider、Storage Provider、Image Provider、Collector Provider 与 Platform Provider。

## 核心功能

| 模块 | 能力 | 当前状态 |
| --- | --- | --- |
| 商品采集 | 1688 可用采集、AliExpress beta、自定义规则 beta、采集任务与批次 | 已支持 |
| 商品草稿 | 商品、SKU、图片、库存阈值、发布前检查 | 已支持 |
| AI 标题优化 | OpenAI-compatible Provider、Prompt 模板、任务记录、应用结果 | 已支持 |
| AI 描述生成 | 商品描述生成、Prompt 模板、AI 任务追踪 | 已支持 |
| SKU 候选推荐 | 订单行 SKU 候选、人工绑定、匹配审计 | 已支持 |
| 图片管理 | 本地 / 云存储文件上传、商品图片管理、对象存储 Provider | 已支持 |
| AI 图片处理 | remove.bg、OpenAI Image、ComfyUI Provider、异步任务队列 | 已支持 |
| 店铺授权 | TikTok Shop / Shopee / Lazada / Amazon 授权基座 | 开发中 |
| 多平台配置 | 平台开放配置 Schema、敏感配置加密与脱敏 | 已支持 |
| 订单同步 | 多平台订单同步框架、任务队列、异常工作台 | 开发中 |
| 商品刊登 | 多平台刊登任务、发布前检查、刊登快照 | 开发中 |
| 库存同步 | 本地库存、平台库存镜像、库存预警、同步任务 | 开发中 |
| AI 客服 | 客服消息同步、AI 建议回复、人工确认外发 | 开发中 |
| 自动化运营 | 失败任务中心、告警、批量 AI、任务重试 | 预留架构 |

## 产品能力地图

```text
AI 商品运营工具
├── 商品采集：1688 / AliExpress / 自定义规则
├── 商品草稿：标题、描述、SKU、图片、库存阈值
├── AI 文本：标题优化、描述生成、Prompt 模板、调用记录
├── AI 图片：去背景、换背景、场景图、异步处理任务
└── 商品发布前检查与批量 AI 操作

多平台跨境 ERP MVP
├── 店铺授权：TikTok Shop / Shopee / Lazada / Amazon
├── 订单同步：平台订单拉取、本地订单、SKU 匹配
├── 库存同步：库存预警、平台库存任务、失败重试
├── 商品刊登：发布任务、平台映射、刊登快照
└── AI 客服：消息同步、建议回复、人工确认发送
```

## 快速开始

TradeMind 提供两种启动方式：

1. **本地开发一键启动**：适合开发者调试和二次开发。
2. **Docker 部署启动**：适合快速试用完整项目。

### 方式一：本地开发一键启动

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

`pnpm dev` 会使用根目录脚本并行启动：

- PostgreSQL / Redis 基础设施（来自 `docker-compose.yml`）
- backend Go 服务
- admin 管理端
- collector 采集服务

常用开发命令：

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm dev:stop
pnpm dev:reset
pnpm build:admin
pnpm build:collector
pnpm collect:test
```

> `pnpm dev:reset` 会重置默认 Compose 数据卷，可能清空本地 PostgreSQL 数据，请谨慎使用。

### 方式二：完整 Docker 部署

仓库已包含 `docker-compose.full.yml`、`backend/Dockerfile`、`admin/Dockerfile`、`collector/Dockerfile` 与 `admin/nginx.conf`。

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

默认访问地址：

| 服务 | 地址 |
| --- | --- |
| Admin | <http://127.0.0.1:8000> |
| Backend Health | <http://127.0.0.1:8080/health> |
| Collector Health | <http://127.0.0.1:3001/health> |

停止服务：

```bash
docker compose -f docker-compose.full.yml down
```

查看日志：

```bash
docker compose -f docker-compose.full.yml logs -f backend
docker compose -f docker-compose.full.yml logs -f admin
docker compose -f docker-compose.full.yml logs -f collector
```

## Docker 部署启动

Docker 完整编排包含：

- PostgreSQL 16
- Redis 7
- Go Gin backend
- React / Ant Design Pro admin（nginx 托管）
- Node.js / Playwright collector

默认端口可通过 `.env` 中的变量覆盖：

| 变量 | 默认值 | 说明 |
| --- | ---: | --- |
| `ADMIN_PUBLISH_PORT` | `8000` | 管理端宿主机端口 |
| `BACKEND_PUBLISH_PORT` | `8080` | 后端 API 宿主机端口 |
| `COLLECTOR_PUBLISH_PORT` | `3001` | Collector 宿主机端口 |
| `POSTGRES_PUBLISH_PORT` | `5432` | PostgreSQL 宿主机端口 |
| `REDIS_PUBLISH_PORT` | `6379` | Redis 宿主机端口 |

生产或公网部署前，请务必修改 `.env` 中的 `JWT_SECRET`、`APP_MASTER_KEY`、`ADMIN_BOOTSTRAP_PASSWORD`、数据库密码等敏感配置。

更多说明见 [docs/docker-deployment.md](docs/docker-deployment.md)。

## 本地开发启动

本地开发需要：

- Node.js
- pnpm `9.15+`
- Go `1.22+`
- Docker / Docker Compose

开发基础设施：

```bash
pnpm dev:infra
```

分服务启动：

```bash
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
```

Collector 浏览器依赖：

```bash
pnpm install:collector:browsers
```

更多说明见 [docs/development.md](docs/development.md)。

## 环境变量说明

本仓库提供两份环境变量模板：

| 文件 | 用途 |
| --- | --- |
| `.env.example` | 本地开发环境变量示例 |
| `.env.docker.example` | Docker 完整部署环境变量示例 |

关键变量：

| 变量 | 默认 / 示例 | 说明 |
| --- | --- | --- |
| `APP_HTTP_ADDR` | `:8080` | backend 监听地址 |
| `DB_DRIVER` | `postgres` | 默认 PostgreSQL，MySQL 仅作为可选兼容 |
| `DB_PORT` | `5432` | PostgreSQL 默认端口 |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis 地址 |
| `COLLECTOR_BASE_URL` | `http://127.0.0.1:3100` | 本地 backend 访问 Collector 的地址 |
| `COLLECTOR_HTTP_ADDR` | `:3100` | 本地 Collector 监听地址 |
| `JWT_SECRET` | `change-me-in-production` | JWT 密钥，生产必须修改 |
| `APP_MASTER_KEY` | 空 / 示例密钥 | AES-GCM 配置加密主密钥，生产必须设置 |
| `ADMIN_BOOTSTRAP_EMAIL` | 空 / 示例账号 | 首个管理员邮箱 |
| `ADMIN_BOOTSTRAP_PASSWORD` | 空 / 示例密码 | 首个管理员密码，生产必须修改 |

敏感信息不要提交到仓库。AI Key、存储 Secret、平台 App Secret、店铺 Token 等应通过后台配置并加密存储。

## 项目结构

```text
trademind-ai/
├── backend/                 # Go + Gin + GORM 主业务服务
├── admin/                   # React + TypeScript + Ant Design Pro 管理后台
├── collector/               # Node.js + TypeScript + Playwright 采集服务
├── docs/                    # 项目文档
├── scripts/                 # 本地开发编排脚本
├── data/uploads/            # 本地上传目录
├── docker-compose.yml       # 本地开发基础设施：PostgreSQL + Redis
├── docker-compose.full.yml  # 完整 Docker 部署编排
├── .env.example             # 本地开发环境变量模板
├── .env.docker.example      # Docker 部署环境变量模板
├── README.en.md             # 英文 README
├── CONTRIBUTING.md          # 贡献指南
└── LICENSE                  # Apache-2.0 License
```

## 技术架构

```text
React + Ant Design Pro Admin
        ↓
Go Gin API
        ↓
PostgreSQL + Redis
        ↓
Node Playwright Collector
```

Provider 扩展架构：

```text
Go Gin API
├── AI Provider
│   ├── OpenAI-compatible
│   ├── DeepSeek / Qwen / Doubao / Gemini / Claude / Ollama 预留
│   └── Prompt 模板与调用记录
├── Storage Provider
│   ├── local
│   ├── S3 / R2 / MinIO
│   ├── Tencent COS
│   └── Aliyun OSS
├── Image Provider
│   ├── remove.bg
│   ├── OpenAI Image
│   └── ComfyUI
├── Platform Provider
│   ├── TikTok Shop
│   ├── Shopee
│   ├── Lazada
│   └── Amazon
└── Collector Provider
    ├── 1688
    ├── AliExpress
    └── custom rules
```

详细设计见 [docs/architecture.md](docs/architecture.md) 与 [docs/provider.md](docs/provider.md)。

## 当前开发优先级

1. **第一优先级：AI 商品运营工具**
   - 商品采集、商品草稿、AI 标题、AI 描述、图片管理、AI 图片处理、批量 AI 操作。
2. **第二优先级：多平台跨境 ERP MVP**
   - 店铺授权、订单同步、SKU 匹配、库存同步、商品刊登、AI 客服建议。
3. **后续迭代：完整 ERP 增强**
   - 多仓、采购、售后、财务、WMS / OMS、复杂 BI、自动化规则引擎。

## 路线图 Roadmap

| 版本 | 重点 | 状态 |
| --- | --- | --- |
| v0.1.0 | 项目地基、登录、配置中心、本地存储、Docker | 已完成 / 持续完善 |
| v0.2.0 | AI 文本能力、Prompt 模板、标题与描述生成 | 已支持 |
| v0.3.0 | 商品草稿、SKU、图片管理、AI 结果应用 | 已支持 |
| v0.4.0 | 采集服务、采集任务、1688 / 自定义规则 | 已支持 |
| v0.5.0 | AI 图片任务、remove.bg / OpenAI Image / ComfyUI | 开发中 |
| v0.6.0 | 店铺授权、平台配置、订单同步、刊登 / 库存 | 开发中 |
| v0.7.0 | AI 客服建议、平台消息同步、人工确认外发 | 开发中 |
| v1.0.0 | 开源稳定版、完整文档、可部署与可扩展生态 | 规划中 |

详细路线图见 [docs/roadmap.md](docs/roadmap.md)。

## 文档导航

| 文档 | 说明 |
| --- | --- |
| [README.en.md](README.en.md) | English README |
| [docs/development.md](docs/development.md) | 本地开发说明 |
| [docs/docker-deployment.md](docs/docker-deployment.md) | Docker 部署说明 |
| [docs/architecture.md](docs/architecture.md) | 架构设计 |
| [docs/provider.md](docs/provider.md) | Provider 扩展机制 |
| [docs/roadmap.md](docs/roadmap.md) | 路线图 |
| [docs/sponsor.md](docs/sponsor.md) | 赞助支持 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献指南 |
| [SECURITY.md](SECURITY.md) | 安全策略 |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | 社区行为准则 |
| [NOTICE](NOTICE) | 第三方声明与致谢 |

## 合作商展示

| 合作商 | 方向 | 状态 |
| --- | --- | --- |
| Coming soon | AI / 平台 / 存储 / 采集 / 运营服务 | 预留 |

## 贡献榜

| 贡献者 | 贡献方向 | 链接 |
| --- | --- | --- |
| Coming soon | Code / Docs / Provider / Prompt / Docker | - |

## 赞助榜

| 赞助者 | 支持方式 | 链接 |
| --- | --- | --- |
| Coming soon | WeChat / Alipay / GitHub Sponsor | - |

## 开源使用规范

本项目基于 Apache-2.0 协议开源。你可以自由学习、使用、修改、二次开发和商业化使用，但必须遵守以下要求：

1. 保留原始 `LICENSE` 文件。
2. 在二次开发项目的 README、文档或关于页面中注明本项目来源。
3. 明确标注原项目地址。
4. 不得移除代码文件中的版权声明。
5. 如果你修改了源码，建议在文档中说明主要修改内容。

原项目地址：

<https://github.com/lien0219/trademind-ai>

## 贡献指南

欢迎任何形式的贡献：

- 提交 Bug
- 提交功能建议
- 改进文档
- 接入新的 AI Provider
- 接入新的 Storage Provider
- 接入新的跨境平台 Provider
- 优化采集规则
- 优化 Prompt 模板
- 完善 Docker 部署

请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。如果你不确定某个方向是否适合当前阶段，可以先提交 Issue 讨论。

## 赞助支持

如果这个项目对你有帮助，欢迎通过以下方式支持：

- Star 本项目
- Fork 并参与贡献
- 提交 Issue / PR
- 分享给更多跨境电商卖家或开发者
- 赞助项目持续维护

微信 / 支付宝赞助二维码见 [docs/sponsor.md](docs/sponsor.md)。

## License

本项目采用 [Apache License 2.0](LICENSE) 开源协议。

## 致谢

感谢所有关注、使用和贡献 TradeMind 的朋友。也感谢 Go、Gin、GORM、PostgreSQL、Redis、React、Ant Design Pro、TypeScript、Playwright 及开源 AI 生态提供的基础能力。

如果你觉得 TradeMind 有价值，欢迎 Star、Fork、提交 Issue 或 PR，一起把它建设成更好用的开源 AI 跨境电商运营平台。
