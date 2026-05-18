<h1 align="center" id="trademind">TradeMind</h1>

<p align="center">
  <strong>Open-source AI Commerce Operation Platform</strong>
</p>

<p align="center">
  Product Collection · Product Drafts · AI Titles · AI Descriptions · Image Management · Store Authorization · Order Sync · AI Reply Suggestions
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
  <a href="README.md">简体中文</a> | English
</p>

> TradeMind is an open-source AI operation tool for cross-border commerce sellers. It currently supports product collection, product drafts, AI title optimization, AI description generation, image management, AI image tasks, store authorization, order sync, SKU matching, product publishing, inventory sync, and AI customer service reply suggestions.

## Screenshots / Demo

> Demo and screenshots are reserved. PRs are welcome to improve the project showcase.

| Admin Console | Product Drafts | AI Product Optimization |
| --- | --- | --- |
| Coming soon | Coming soon | Coming soon |

## Table of Contents

- [Introduction](#introduction)
- [Why This Project](#why-this-project)
- [Core Features](#core-features)
- [Product Capability Map](#product-capability-map)
- [Quick Start](#quick-start)
- [Docker Deployment](#docker-deployment)
- [Local Development](#local-development)
- [Environment Variables](#environment-variables)
- [Project Structure](#project-structure)
- [Technical Architecture](#technical-architecture)
- [Current Development Priorities](#current-development-priorities)
- [Roadmap](#roadmap)
- [Documentation](#documentation)
- [Partners](#partners)
- [Contributors Board](#contributors-board)
- [Sponsors Board](#sponsors-board)
- [Open Source Usage](#open-source-usage)
- [Contributing](#contributing)
- [Sponsor](#sponsor)
- [License](#license)
- [Acknowledgements](#acknowledgements)

## Introduction

TradeMind is an open-source AI commerce operation platform for cross-border sellers and developer teams that need to handle product onboarding, content optimization, image processing, store operations, and order workflows more efficiently.

The project already provides a runnable product operation workflow: collect product links, create product drafts, manage SKUs and images, generate AI titles and descriptions, run image processing tasks, configure platform stores, sync orders, match SKUs, sync inventory, publish products, and generate AI-assisted customer service replies. TradeMind uses provider abstractions for AI, storage, image processing, collection sources, and commerce platforms, making it suitable for private deployment and secondary development.

```text
Product collection → Product drafts → AI title optimization → AI description generation → Image management
  → AI image processing → Store authorization → Product publishing → Order sync
  → SKU matching → Inventory sync → AI reply suggestions
```

## Why This Project

Cross-border commerce sellers repeatedly collect products, rewrite titles, generate multilingual descriptions, process images, maintain platform stores, sync orders, and reply to buyers. Traditional ERP systems are usually stronger at data entry and workflow management, while TradeMind focuses on embedding AI into product operations and cross-platform collaboration.

TradeMind aims to provide an open-source, deployable, and extensible platform for individual sellers, operation teams, and developers. You can connect your own AI Provider, Storage Provider, Image Provider, Collector Provider, and Platform Provider around your actual business workflow.

## Core Features

| Module | Capability | Status |
| --- | --- | --- |
| Product Collection | 1688 collection, AliExpress beta, custom rule beta, collection tasks and batches | Supported |
| Product Drafts | Products, SKUs, images, inventory thresholds, readiness checks | Supported |
| AI Title Optimization | OpenAI-compatible Provider, prompt templates, task records, apply results | Supported |
| AI Description Generation | Product description generation, prompt templates, AI task tracking | Supported |
| SKU Candidate Recommendation | Candidate SKUs for order items, manual binding, match audit | Supported |
| Image Management | Local / cloud file upload, product image management, storage providers | Supported |
| AI Image Processing | remove.bg, OpenAI Image, ComfyUI Provider, async task queue | Supported |
| Store Authorization | TikTok Shop / Shopee / Lazada / Amazon authorization foundation | In progress |
| Multi-platform Configuration | Platform app config schema, encrypted and masked sensitive settings | Supported |
| Order Sync | Multi-platform order sync framework, task queue, exception workspace | In progress |
| Product Publishing | Multi-platform publishing tasks, readiness checks, publication snapshots | In progress |
| Inventory Sync | Local stock, platform stock mirror, inventory alerts, sync tasks | In progress |
| AI Customer Service | Message sync, AI suggested replies, manual send confirmation | In progress |
| Operation Automation | Failure task center, alerts, batch AI, task retry | Architecture reserved |

## Product Capability Map

```text
AI Product Operation Tool
├── Product collection: 1688 / AliExpress / custom rules
├── Product drafts: titles, descriptions, SKUs, images, inventory thresholds
├── AI text: title optimization, description generation, prompt templates, call records
├── AI image: background removal, background replacement, scene images, async image tasks
└── Readiness checks and batch AI operations

Multi-platform Cross-border ERP MVP
├── Store authorization: TikTok Shop / Shopee / Lazada / Amazon
├── Order sync: platform order import, local orders, SKU matching
├── Inventory sync: stock alerts, platform inventory tasks, failure retry
├── Product publishing: publishing tasks, platform mappings, publication snapshots
└── AI customer service: message sync, suggested replies, manual send confirmation
```

## Quick Start

TradeMind provides two startup paths:

1. **One-command local development**: for development and secondary customization.
2. **Docker deployment**: for quickly running the full project.

### Option 1: Local Development

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

`pnpm dev` starts the root development script and runs:

- PostgreSQL / Redis infrastructure from `docker-compose.yml`
- backend Go service
- admin console
- collector service

Useful commands:

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

> `pnpm dev:reset` resets the default Compose volumes and may clear your local PostgreSQL data.

### Option 2: Full Docker Deployment

The repository includes `docker-compose.full.yml`, `backend/Dockerfile`, `admin/Dockerfile`, `collector/Dockerfile`, and `admin/nginx.conf`.

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell:

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Default URLs:

| Service | URL |
| --- | --- |
| Admin | <http://127.0.0.1:8000> |
| Backend Health | <http://127.0.0.1:8080/health> |
| Collector Health | <http://127.0.0.1:3001/health> |

Stop services:

```bash
docker compose -f docker-compose.full.yml down
```

View logs:

```bash
docker compose -f docker-compose.full.yml logs -f backend
docker compose -f docker-compose.full.yml logs -f admin
docker compose -f docker-compose.full.yml logs -f collector
```

## Docker Deployment

The full Docker Compose setup includes:

- PostgreSQL 16
- Redis 7
- Go Gin backend
- React / Ant Design Pro admin served by nginx
- Node.js / Playwright collector

Default host ports can be overridden in `.env`:

| Variable | Default | Description |
| --- | ---: | --- |
| `ADMIN_PUBLISH_PORT` | `8000` | Admin host port |
| `BACKEND_PUBLISH_PORT` | `8080` | Backend API host port |
| `COLLECTOR_PUBLISH_PORT` | `3001` | Collector host port |
| `POSTGRES_PUBLISH_PORT` | `5432` | PostgreSQL host port |
| `REDIS_PUBLISH_PORT` | `6379` | Redis host port |

Before production or public deployment, update `JWT_SECRET`, `APP_MASTER_KEY`, `ADMIN_BOOTSTRAP_PASSWORD`, database passwords, and all other sensitive values in `.env`.

See [docs/docker-deployment.md](docs/docker-deployment.md) for more details.

## Local Development

Local development requires:

- Node.js
- pnpm `9.15+`
- Go `1.22+`
- Docker / Docker Compose

Start infrastructure:

```bash
pnpm dev:infra
```

Start services separately:

```bash
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
```

Install Collector browser dependency:

```bash
pnpm install:collector:browsers
```

See [docs/development.md](docs/development.md) for more details.

## Environment Variables

The repository provides two environment templates:

| File | Purpose |
| --- | --- |
| `.env.example` | Local development environment template |
| `.env.docker.example` | Full Docker deployment environment template |

Key variables:

| Variable | Default / Example | Description |
| --- | --- | --- |
| `APP_HTTP_ADDR` | `:8080` | backend listen address |
| `DB_DRIVER` | `postgres` | PostgreSQL by default; MySQL is optional compatibility |
| `DB_PORT` | `5432` | PostgreSQL default port |
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis address |
| `COLLECTOR_BASE_URL` | `http://127.0.0.1:3100` | backend to collector URL for local development |
| `COLLECTOR_HTTP_ADDR` | `:3100` | local collector listen address |
| `JWT_SECRET` | `change-me-in-production` | JWT secret; must be changed in production |
| `APP_MASTER_KEY` | empty / example secret | AES-GCM master key for encrypted settings |
| `ADMIN_BOOTSTRAP_EMAIL` | empty / example account | first admin email |
| `ADMIN_BOOTSTRAP_PASSWORD` | empty / example password | first admin password; must be changed in production |

Do not commit sensitive information. AI keys, storage secrets, platform app secrets, and store tokens should be configured in the admin console and stored encrypted by the backend.

## Project Structure

```text
trademind-ai/
├── backend/                 # Go + Gin + GORM main service
├── admin/                   # React + TypeScript + Ant Design Pro admin console
├── collector/               # Node.js + TypeScript + Playwright collector service
├── docs/                    # project documentation
├── scripts/                 # local development orchestration scripts
├── data/uploads/            # local upload directory
├── docker-compose.yml       # local development infrastructure: PostgreSQL + Redis
├── docker-compose.full.yml  # full Docker deployment compose file
├── .env.example             # local development env template
├── .env.docker.example      # Docker deployment env template
├── README.md                # Simplified Chinese README
├── CONTRIBUTING.md          # contribution guide
└── LICENSE                  # Apache-2.0 License
```

## Technical Architecture

```text
React + Ant Design Pro Admin
        ↓
Go Gin API
        ↓
PostgreSQL + Redis
        ↓
Node Playwright Collector
```

Provider architecture:

```text
Go Gin API
├── AI Provider
│   ├── OpenAI-compatible
│   ├── DeepSeek / Qwen / Doubao / Gemini / Claude / Ollama reserved
│   └── prompt templates and call records
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

See [docs/architecture.md](docs/architecture.md) and [docs/provider.md](docs/provider.md) for more details.

## Current Development Priorities

1. **First priority: AI product operation tool**
   - Product collection, product drafts, AI titles, AI descriptions, image management, AI image processing, batch AI operations.
2. **Second priority: multi-platform cross-border ERP MVP**
   - Store authorization, order sync, SKU matching, inventory sync, product publishing, AI reply suggestions.
3. **Later iteration: full ERP enhancement**
   - Multi-warehouse, purchasing, after-sales, finance, WMS / OMS, complex BI, automation rules.

## Roadmap

| Version | Focus | Status |
| --- | --- | --- |
| v0.1.0 | foundation, login, settings, local storage, Docker | completed / improving |
| v0.2.0 | AI text capability, prompt templates, title and description generation | supported |
| v0.3.0 | product drafts, SKUs, image management, applying AI results | supported |
| v0.4.0 | collector service, collection tasks, 1688 / custom rules | supported |
| v0.5.0 | AI image tasks, remove.bg / OpenAI Image / ComfyUI | in progress |
| v0.6.0 | store authorization, platform configuration, order sync, publishing / inventory | in progress |
| v0.7.0 | AI reply suggestions, platform message sync, manual send confirmation | in progress |
| v1.0.0 | stable open-source release, complete docs, deployable and extensible ecosystem | planned |

See [docs/roadmap.md](docs/roadmap.md) for the detailed roadmap.

## Documentation

| Document | Description |
| --- | --- |
| [README.md](README.md) | 简体中文 README |
| [docs/development.md](docs/development.md) | local development guide |
| [docs/docker-deployment.md](docs/docker-deployment.md) | Docker deployment guide |
| [docs/architecture.md](docs/architecture.md) | architecture design |
| [docs/provider.md](docs/provider.md) | Provider extension mechanism |
| [docs/roadmap.md](docs/roadmap.md) | roadmap |
| [docs/sponsor.md](docs/sponsor.md) | sponsor information |
| [CONTRIBUTING.md](CONTRIBUTING.md) | contribution guide |
| [SECURITY.md](SECURITY.md) | security policy |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | community code of conduct |
| [NOTICE](NOTICE) | third-party notices and acknowledgements |

## Partners

| Partner | Area | Status |
| --- | --- | --- |
| Coming soon | AI / Platform / Storage / Collector / Operation Service | Reserved |

## Contributors Board

| Contributor | Contribution Area | Link |
| --- | --- | --- |
| Coming soon | Code / Docs / Provider / Prompt / Docker | - |

## Sponsors Board

| Sponsor | Support Method | Link |
| --- | --- | --- |
| Coming soon | WeChat / Alipay / GitHub Sponsor | - |

## Open Source Usage

This project is licensed under Apache-2.0. You may use, modify, distribute, and commercialize it, but you must comply with the following requirements:

1. Keep the original `LICENSE` file.
2. Mention this project as the source in your derived project's README, documentation, or about page.
3. Clearly include the original project repository URL.
4. Do not remove copyright notices from source files.
5. If you modify the source code, it is recommended to document the major changes.

Original project repository:

<https://github.com/lien0219/trademind-ai>

## Contributing

All kinds of contributions are welcome:

- Report bugs
- Suggest features
- Improve documentation
- Add new AI Providers
- Add new Storage Providers
- Add new commerce Platform Providers
- Improve collector rules
- Improve prompt templates
- Improve Docker deployment

Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a PR. If you are unsure whether a direction fits the current stage, open an Issue first.

## Sponsor

If this project helps you, you can support it by:

- Starring the project
- Forking and contributing
- Opening Issues / PRs
- Sharing it with cross-border commerce sellers or developers
- Sponsoring ongoing maintenance

WeChat Pay and Alipay sponsor QR codes are available in [docs/sponsor.md](docs/sponsor.md).

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Acknowledgements

Thanks to everyone who follows, uses, and contributes to TradeMind. Thanks also to Go, Gin, GORM, PostgreSQL, Redis, React, Ant Design Pro, TypeScript, Playwright, and the open-source AI ecosystem.

If TradeMind is useful to you, please Star, Fork, open Issues, or submit PRs to help build a better open-source AI commerce operation platform.
