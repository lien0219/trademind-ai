# 架构设计

TradeMind 采用 monorepo 组织，核心由 Go backend、React admin、Node collector、PostgreSQL 与 Redis 组成。

## 总体架构

```text
React + Ant Design Pro Admin
        ↓
Go Gin API
        ↓
PostgreSQL + Redis
        ↓
Node Playwright Collector
```

## 目录职责

```text
backend/    Go Gin API、业务模块、Provider、数据库迁移、队列 Worker
admin/      React + TypeScript + Ant Design Pro 管理后台
collector/  Node.js + TypeScript + Playwright 商品采集服务
docs/       项目文档
scripts/    本地开发编排脚本
```

## 后端分层

- `handler`：处理请求参数、鉴权上下文与响应。
- `service`：编排业务流程与事务。
- `providers`：封装 AI、存储、图片、平台、采集等外部能力。
- `modules`：按业务域组织认证、设置、商品、采集、AI、图片、店铺、订单、客服等模块。
- `queue` / worker：处理采集、图片、订单同步、客服同步、刊登、库存同步等异步任务。

## 管理端分层

- 页面使用 React + TypeScript。
- 表格优先使用 ProTable。
- 表单优先使用 ProForm。
- API 请求统一放在 services。
- 敏感配置展示必须脱敏。
- 前端不直接调用第三方 AI、平台或存储 API。

## 采集服务

Collector 是独立 Node.js 服务：

- 使用 Playwright 打开页面并解析商品数据。
- 不直接操作主业务数据库。
- 通过 HTTP 与 Go backend 通信。
- 每个采集源以 Collector Provider 形式接入。

## 数据与队列

- PostgreSQL 是默认数据库。
- Redis 用于异步任务队列、租约与部分状态协调。
- 敏感配置通过后端 AES-GCM 加密存储。

## 安全原则

- 不在代码中写死 API Key、Token、Secret、平台凭证。
- 日志禁止输出完整密钥、密码、Cookie、Token。
- AI、存储、平台能力必须由后端 Provider 调用。
- AI 客服默认只生成建议，外发必须人工确认。

## 扩展方向

TradeMind 的主要扩展点包括：

- AI Provider
- Storage Provider
- Image Provider
- Platform Provider
- Collector Provider
- Prompt 模板
- 异步任务 Worker
