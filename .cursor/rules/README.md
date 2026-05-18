# Cursor Rules 索引

本目录用于给 Cursor / AI Agent 提供项目级长期规则。规则按“全局规则”和“领域规则”拆分，避免一个大规则文件带来过多噪声。

## 维护原则

- 不把所有规则合并成一个大文件。
- 全局、必须长期遵守的规则使用 `alwaysApply: true`。
- 只在特定目录或技术栈生效的规则使用 `globs` + `alwaysApply: false`。
- 新增规则时优先保持简短、可执行、无冲突。
- 修改代码、配置、环境变量、Docker、CI、API、Provider 或公共契约时，必须遵守 `12-ai-coding-doc-sync.mdc` 的文档同步要求。

## 全局规则

| 文件 | 作用 |
| --- | --- |
| `00-project-overview.mdc` | 项目定位、MVP 范围、长期方向、禁止事项 |
| `01-architecture.mdc` | 总体架构、分层边界、Provider 抽象、目录结构 |
| `08-api-db-security.mdc` | API 规划、数据库约定、settings、加密、日志安全 |
| `09-dev-workflow.mdc` | 开发优先级、阶段规划、Cursor 实现行为、文档要求 |
| `10-progress-sync.mdc` | 阶段、模块或较大修改后更新 `docs/PROGRESS.md` |
| `11-local-dev-postgres.mdc` | 本地与默认数据库统一采用 PostgreSQL |
| `12-ai-coding-doc-sync.mdc` | AI 编程规则、配置与文档同步要求 |

## 领域规则

| 文件 | 适用范围 | 作用 |
| --- | --- | --- |
| `02-backend-go-gin.mdc` | `backend/**/*.go` | Go / Gin / GORM 后端编码规则 |
| `03-frontend-react-antd-pro.mdc` | `admin/src/**/*.{ts,tsx}` | React / TypeScript / Ant Design Pro 前端规则 |
| `04-ui-style.mdc` | `admin/src/**/*.{ts,tsx,less,css,scss}` | 后台 UI 风格、布局、状态、空态和交互规范 |
| `05-ai-provider.mdc` | AI / Image 相关后端与前端设置页面 | AI Provider、Prompt、AI 任务、图片 AI、客服 AI |
| `06-storage-provider.mdc` | Storage 后端与存储设置页面 | Storage Provider、本地存储、云存储扩展 |
| `07-collector-node-playwright.mdc` | `collector/**/*.{ts,tsx,js,json}` | Node / Playwright 采集服务规则 |

## 新增规则 Checklist

- [ ] 文件名按数字前缀排序。
- [ ] frontmatter 包含 `description`。
- [ ] 全局规则设置 `alwaysApply: true`。
- [ ] 领域规则设置合适的 `globs` 和 `alwaysApply: false`。
- [ ] 规则内容不与现有规则冲突。
- [ ] 如规则影响公开协作方式，同步更新 `docs/ai-coding-rules.md` 或 `docs/README.md`。
