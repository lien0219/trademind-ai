# 贸灵 TradeMind

> Open-source AI commerce operation platform.  
> 开源 AI 跨境电商运营平台。

贸灵 TradeMind 是一个面向跨境电商卖家的开源 AI 运营工具，帮助卖家完成商品采集、AI 标题优化、AI 描述生成、商品图片处理、店铺授权和智能客服等运营流程。

项目早期不追求一次性做成完整 ERP，而是从最容易产生价值的链路切入：

```text
商品采集 → 商品草稿 → AI 优化 → 图片处理 → 店铺授权 → 一键刊登 → AI 客服 → 自动化运营
```

---

## 项目定位

TradeMind 的目标是成为一个 **AI First 的跨境电商运营平台**。

它不是传统意义上只做表格管理的 ERP，而是希望把 AI 能力嵌入跨境卖家的日常运营流程中，让商品上架、标题优化、描述生成、图片处理和客服回复变得更高效。

### 当前阶段

当前阶段定位为：

```text
AI 跨境上货助手 + 商品采集工具 + AI 商品优化平台
```

### 长期方向

后续逐步扩展为：

```text
AI 跨境电商 ERP
AI Seller Operation Platform
AI Commerce Automation Platform
```

---

## 核心能力

### 已规划能力

- 商品采集
- 商品草稿管理
- AI 标题优化
- AI 描述生成
- 商品图片管理
- AI 图片处理扩展
- 本地存储 / 云存储可配置
- 店铺授权 Provider 架构
- AI 客服 Tool Calling 预留
- 批量刊登能力预留
- 订单同步能力预留
- 自动化规则引擎预留

### MVP 核心闭环

第一个可用版本重点完成：

```text
用户登录
  ↓
系统配置 AI Provider / 存储方式
  ↓
输入 1688 商品链接
  ↓
采集商品信息
  ↓
保存为商品草稿
  ↓
AI 优化标题
  ↓
AI 生成描述
  ↓
图片上传 / 图片管理
  ↓
形成可编辑商品草稿
```

---

## 技术栈

### 前端后台

- React
- TypeScript
- Ant Design Pro
- Ant Design
- ProTable
- ProForm

### 后端主服务

- Go
- Gin
- GORM
- MySQL / PostgreSQL（开发默认 **PostgreSQL**；`DB_DRIVER=mysql` 仍支持）
- Redis

### 采集服务

- Node.js
- TypeScript
- Playwright

### AI 能力

AI 能力采用可插拔 Provider 设计，优先支持 OpenAI-Compatible API，后续扩展：

- OpenAI
- DeepSeek
- 通义千问
- 豆包
- Gemini
- Claude
- Ollama
- 其他 OpenAI-Compatible 服务

### 存储能力

存储能力采用 Provider 设计：

- Local Storage
- S3
- 腾讯云 COS
- 阿里云 OSS
- Cloudflare R2
- MinIO

---

## 本地一键启动（小白推荐）

前置：**Node.js**、**pnpm**、**Go**、**Docker Desktop**（或已运行的 Docker 引擎）。仓库默认数据库为 **PostgreSQL**，缓存为 **Redis**，由根目录 `docker-compose.yml` 提供。

### 1. 安装依赖

```bash
pnpm install
```

首次使用 Playwright 采集前，建议在仓库根目录执行一次：

```bash
pnpm install:collector:browsers
```

### 2. 启动全部服务

```bash
pnpm dev
```

会自动：

- 若根目录 **没有** `.env`：从 **`.env.example`** 复制一份（**不会覆盖**已有 `.env`）
- 执行 **`docker compose up -d`** 拉起 **PostgreSQL** 与 **Redis**（**不会**执行 `docker compose down`，也不会删卷）
- 并行启动 **Go backend**、**admin** 管理端、**collector** 采集服务

启动成功后，控制台会给出类似链接（具体端口以 `.env` 与终端为准）：

- **Backend**：默认常为 `http://127.0.0.1:8080`（`APP_HTTP_ADDR`）
- **Admin**：Umi 默认常为 `http://127.0.0.1:8000`（以终端输出的 **Local:** 为准）
- **Collector**：默认常为 `http://127.0.0.1:3100`（`COLLECTOR_HTTP_ADDR`）

按 **Ctrl+C** 可结束本次启动的三个子进程；**不会**自动停止 Docker 容器。

### 3. 打开后台

在浏览器打开管理端地址（一般为 **`http://127.0.0.1:8000`**），登录与引导见界面说明。

### 4. 健康检查

- **Backend**：`http://127.0.0.1:8080/health`（或你的 `APP_HTTP_ADDR` 对应主机端口）
- **Collector**：`http://127.0.0.1:3100/health`（或你的 Collector 监听端口）

### 5. 环境与诊断命令

```bash
pnpm check:dev    # 检查 Node / pnpm / Go / Docker / .env 等（不打印密钥或完整 .env）
pnpm dev:infra    # 仅启动 PostgreSQL + Redis
pnpm dev:stop     # 停止 compose 服务（不删 volume）
```

### 6. 常见问题

- **Docker 没启动 / `docker ps` 报错**  
  先启动 **Docker Desktop**（Windows/macOS）或系统 Docker 服务（Linux），再执行 `pnpm dev`。

- **Go 未安装**  
  从 [go.dev/dl](https://go.dev/dl/) 安装，终端可运行 `go version`。

- **pnpm 未安装**  
  可先装 Node.js，再执行：`npm install -g pnpm@9`。

- **端口被占用**  
  修改根目录 `.env` 中 **`APP_HTTP_ADDR`**、**`COLLECTOR_HTTP_ADDR`** 等，避免与本地其它服务冲突；管理端端口可在启动日志中确认或使用 Umi 环境变量（见 admin 文档）。

- **数据库连接失败**  
  确认容器已起：`pnpm dev:infra`；确认 `.env` 中 **`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`** 与 `docker-compose.yml` 一致（示例默认为本机 `5432`）。

- **Collector 未启动导致采集失败**  
  采集由独立进程提供；请保证 **`pnpm dev`** 或 **`pnpm dev:collector`** 已运行，且后端 **`COLLECTOR_BASE_URL`** 与 Collector 监听地址一致（见 `.env.example`）。

### 7. 重置数据库（慎用）

会删除 Compose 管理的数据卷，**清空 PostgreSQL 数据**。仅在确认无需保留本地数据时使用：

```bash
pnpm dev:reset
```

按提示输入 **`RESET`** 后才会执行 **`docker compose down -v`**。完成后请自行执行 **`pnpm dev:infra`** 或 **`pnpm dev`** 重新创建容器。

---

## 分开启动（高级）

适合需要单独调试某一服务的开发者：

```bash
docker compose up -d

cd backend
go run ./cmd/server

# 另开终端，在仓库根目录：
pnpm dev:admin
pnpm dev:collector
```

---

## 架构设计

### MVP 架构

```text
React + Ant Design Pro
        ↓
Go Gin API
        ↓
PostgreSQL（默认）/ MySQL
        ↓
Redis Queue
        ↓
Node Playwright Collector
```

### 可扩展 Provider 架构

```text
Go Gin API
   ├── Storage Provider
   │     ├── local
   │     ├── s3
   │     ├── cos
   │     ├── oss
   │     └── r2
   │
   ├── AI Provider
   │     ├── openai-compatible
   │     ├── deepseek
   │     ├── qwen
   │     ├── doubao
   │     ├── gemini
   │     ├── claude
   │     └── ollama
   │
   ├── Image Provider
   │     ├── local
   │     ├── removebg
   │     ├── openai-image
   │     ├── comfyui
   │     └── jimeng
   │
   ├── Platform Provider
   │     ├── tiktok
   │     ├── shopee
   │     ├── lazada
   │     ├── shopify
   │     └── amazon
   │
   └── Collector Provider
         ├── 1688
         ├── taobao
         ├── pdd
         ├── shein
         └── custom
```

---

## 项目结构规划

```text
trademind-ai/
├── pnpm-workspace.yaml     # pnpm 工作区（admin + collector）
├── pnpm-lock.yaml
├── package.json            # 根脚本：dev（一键启）/ dev:admin / dev:collector 等
├── scripts/                # 本地开发编排（check-dev-env / dev-all / dev-backend）
├── docker-compose.yml      # 本地 PostgreSQL + Redis
├── .env.example            # 环境变量示例（复制为 .env）
├── backend/
│   ├── cmd/server/         # main 入口
│   ├── internal/
│   │   ├── api/            # HTTP 路由注册
│   │   ├── config/
│   │   ├── database/
│   │   ├── encrypt/
│   │   ├── logger/
│   │   ├── modules/        # 业务垂直模块（auth / product / collect …）
│   │   ├── pkg/response/    # 统一 API 响应结构
│   │   ├── providers/      # Storage / AI / Image / Platform / Collector 抽象
│   │   └── queue/
│   ├── migrations/
│   ├── configs/
│   └── go.mod
├── admin/                  # React + Ant Design Pro 后台（脚手架占位）
│   └── src/
├── collector/              # Node + Playwright 采集（HTTP /v1/collect）
│   └── src/                # providers / browser / tasks / http
├── docs/                   # 项目文档
└── data/uploads/           # 本地存储挂载目录（默认）
```

### 本地开发（pnpm）

日常推荐：见上文 **「本地一键启动（小白推荐）」**，执行 **`pnpm dev`** 即可。

```bash
pnpm install          # 工作区安装依赖（admin postinstall 会执行 max setup）
pnpm dev:admin        # 启动 Ant Design Pro 管理端
pnpm build:admin      # 生产构建 admin，产物在 admin/dist
pnpm install:collector:browsers   # 首次使用 Playwright 时安装 Chromium
pnpm dev:collector    # 启动采集服务（HTTP :3100 默认）
pnpm build:collector # 编译采集服务
```

管理端开发时代理 `/api` → `http://127.0.0.1:8080`（见 `admin/.umirc.ts`），需先启动 Go 后端再调接口。**对象存储**：默认 `kind=local`；云存储在后台「存储设置」写入 `settings.storage`（详见根目录 `.env.example`：**local**、**S3/R2/minio（S3 兼容）**、**腾讯云 COS（独立 SDK）**、**阿里云 OSS（独立 SDK）**）。采集服务为独立进程，示例：`POST http://127.0.0.1:3100/v1/collect`，body `{"source":"1688","url":"https://..."}`。

---

## 功能规划

### v0.1.0 项目地基版

- Monorepo 结构
- Go Gin 后端
- React Ant Design Pro 后台
- 登录系统
- Settings 配置中心
- 本地存储
- 文件上传
- Docker Compose

### v0.2.0 AI 文本版

- AI Provider
- OpenAI-Compatible Provider
- Prompt 模板
- AI 标题优化
- AI 描述生成
- AI 任务记录

### v0.3.0 商品草稿版

- 商品草稿
- SKU 管理
- 图片管理
- AI 结果应用
- 商品编辑

### v0.4.0 采集版

- Node Playwright Collector
- 1688 单链接采集
- 采集任务
- 失败重试
- 采集结果生成商品草稿

### v0.5.0 图片能力版

- Image Provider
- 图片处理任务
- 去背景 Provider 预留
- ComfyUI Provider 预留
- 商品图处理工作台

### v0.6.0 店铺授权版

- Platform Provider
- 店铺列表
- 平台配置
- TikTok Shop 授权
- Shopee 授权

### v0.7.0 AI 客服预览版

- AI 客服建议回复
- FAQ 知识库
- Tool Calling 接口预留
- 人工确认队列

### v1.0.0 开源稳定版

- 商品采集
- AI 商品优化
- 本地 / 云存储
- 店铺授权基础能力
- AI 客服基础能力
- 完整部署文档
- 完整 Provider 扩展文档

---

## 开放平台应用配置（多平台 Schema）

各渠道的 Partner/Open API 控制台字段不尽相同，贸灵使用 **Platform Provider** 下发的 **`appConfigSchema`**（见 `GET /api/v1/platform/providers`），管理端 **`/settings/platforms`** 按字段定义 **动态渲染** 表单并把值入库到 **`settings` 分组**（如 `platform_tiktok`、`platform_shopee` …），`sensitive=true` 的项 **AES-GCM** 加密，API **统一脱敏**（占位含 `****` 的更新不会覆盖原密钥）。店铺完成 OAuth 后的 **access_token / refresh_token** 仅保存在 **`shop_auth_tokens`**，请与上述「应用级」配置区分开。

你可以在对应开放平台注册自建应用后将参数填写到贸灵 **「设置 → 平台开放配置」**，常见门户示例：

| 平台 | 文档 / 控制台入口 |
|------|-------------------|
| TikTok Shop | [partner.tiktokshop.com](https://partner.tiktokshop.com/) |
| Shopee | [open.shopee.com](https://open.shopee.com/) |
| Lazada | [open.lazada.com](https://open.lazada.com/) |
| Amazon SP-API | [developer.amazonservices.com](https://developer.amazonservices.com/) |
| AliExpress | [developers.aliexpress.com](https://developers.aliexpress.com/) |
| Shopify | [partners.shopify.com](https://partners.shopify.com/) |
| WooCommerce REST | [woocommerce.com/document/rest-api/](https://woocommerce.com/document/rest-api/) |
| eBay | [developer.ebay.com](https://developer.ebay.com/) |

保存与读取 API：`GET` / `PUT /api/v1/platform/settings/:platform`（后端按 Schema 校验字段，`platform.settings.update` 写入操作日志，不落明文密钥）。

### TikTok Shop（首发 Beta）

本项目**不内置**任何 TikTok Partner 的 App Key、App Secret，也不在代码或 `.env.example` 中写入可被误抄作生产网关的占位 URL。你在 Partner Center 获取的凭证填入 **`platform_tiktok`** 分组；随后在 **店铺管理** 中为每个店铺单独完成 OAuth。**TikTok 运行时**从 **`settings.platform_tiktok`** 读取宿主与密钥，并从 **`shop_auth_tokens`** 读取 token（缺任一必填项后端返回明确报错）。

推荐步骤：

1. 注册 / 登录 TikTok Shop Partner Center。

2. 创建 Open API 应用并获取 App Key / App Secret。

3. 在 Partner 控制台配置与你的部署域名一致的 **Redirect URI**，并勾选订单读取等相关 Scope。

4. 登录贸灵管理端 **`/settings/platforms`**，在 **TikTok Shop** Tab 填写 schema 必填项（含 **`api_version`、超时与宿主 URL**，见表单说明）。

5. 可先调用 `POST /api/v1/settings/test-platform-tiktok` **仅校验结构**（不真实请求 TikTok）；保存时后端亦会按 TikTok Schema 校验。

6. 在 **`/shops`** 创建 TikTok 店铺并 **生成授权链接** 完成 OAuth；抽屉内「覆盖 App Key / Secret」仅用于多应用场景调试。

7. 使用 **`POST /api/v1/shops/:id/sync-orders`** 或后台「同步订单」写入内部 `orders`。

---

## AI 扩展方向

### AI 商品能力

- AI 标题批量优化
- AI 多语言翻译
- AI 关键词提取
- AI 类目推荐
- AI 卖点提取
- AI 定价建议
- AI 违禁词检测
- AI 平台规则检查
- AI 商品评分

### AI 图片能力

- AI 去背景
- AI 换背景
- AI 场景图生成
- AI 模特图生成
- AI 图片翻译
- AI 海报生成
- AI 主图评分
- AI 批量生成多平台尺寸图

### AI 客服能力

- AI FAQ 回复
- AI 订单查询
- AI 物流查询
- AI 售后回复
- AI 多语言客服
- AI 情绪识别
- AI 投诉升级判断
- 人工接管机制
- 自动回复规则

### AI Agent 能力

长期目标是让用户可以通过自然语言指令完成运营任务。

示例：

```text
帮我把今天采集的女装商品全部生成英文标题，并筛选出适合 TikTok Shop 的 20 个商品。
```

系统执行：

1. 查询今日采集商品。
2. 判断商品类目。
3. 调用 AI 标题优化。
4. 调用 AI 商品评分。
5. 返回推荐商品列表。
6. 生成任务报告。

---

## ERP 扩展方向

后续会逐步扩展：

- 商品刊登
- 平台类目映射
- 平台属性映射
- 订单同步
- 库存管理
- 物流管理
- 售后管理
- 财务统计
- 自动化规则引擎
- 多店铺管理
- 多租户 SaaS

---

## 开发原则

```text
先做小闭环，再做大 ERP。
先做 AI 商品优化，再做完整供应链。
先做可配置，再做高级自动化。
先做 Provider 抽象，再接具体平台。
```

### 技术原则

```text
Go 做主业务。
React 做后台。
Node 做采集。
Redis 做队列。
Provider 做扩展。
Prompt 做 AI 技能。
本地存储保证开箱即用。
云存储保证生产可用。
```

### AI 原则

```text
AI 不写死模型。
Prompt 不写死代码。
模型可切换。
输出可追踪。
调用可计费。
工具可扩展。
客服先人工确认。
自动化必须可回滚。
```

---

## 快速开始

> 当前项目处于规划和初始化阶段，后续会提供完整 Docker Compose 启动方式。

计划支持：

```bash
docker-compose up -d
```

---

## CI / 自动检查

本仓库使用 [GitHub Actions](.github/workflows/go.yml) 在 **`push` 与针对 `main` 的 `pull_request`** 时自动检查 **Go 后端**（`backend/`）：`gofmt` 格式、`go vet` 静态检查、`go test` 与 `go build`。

---

## 贡献方向

欢迎围绕以下方向参与：

- React Ant Design Pro 后台页面
- Go Gin 后端模块
- Node Playwright 商品采集
- AI Provider 接入
- Storage Provider 接入
- Prompt 模板优化
- 图片处理 Provider
- 跨境平台 API 接入
- 文档完善

---

## License

License 待定，建议方向：

- Apache-2.0：更适合快速传播和商业友好生态
- AGPL-3.0：更适合保护开源项目不被闭源套壳

---

## 项目说明

TradeMind 目前处于早期阶段，优先完成最小可用闭环：

```text
商品采集 → 商品草稿 → AI 标题优化 → AI 描述生成 → 图片管理
```

该闭环稳定后，再继续扩展到：

```text
AI 图片处理 → 店铺授权 → 一键刊登 → 订单同步 → AI 客服 → 自动化运营 → 完整 ERP
```
