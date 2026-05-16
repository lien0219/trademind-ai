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
├── package.json            # 根脚本：dev:admin / build:admin 等
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
