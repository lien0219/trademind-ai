# AGENTS.md

本文件是 TradeMind 给 AI 编程工具和协作开发者的通用入口。开始工作前先读取与任务相关的规范，不要凭空假设脚本、端口、字段或运行状态。

## 项目定位

贸灵 TradeMind 是开源 AI 跨境电商运营平台，聚焦：

1. AI 商品运营工具
2. 多平台跨境 ERP MVP

项目已进入生产维护与受控 ERP 扩展并行阶段。完整 ERP 只按明确批准的业务闭环逐步建设，不做一次性重写或无边界扩张。

## 必读入口

- `README.md`、`docs/README.md`
- `docs/ai-workflow.md`、`docs/ai-coding-rules.md`
- `.agents/skills/code-quality/SKILL.md`
- `.agents/skills/modular-architecture/SKILL.md`
- `.agents/skills/project-testing/SKILL.md`
- Admin UI 任务还需读取 `.agents/skills/frontend-design/SKILL.md` 与 `.agents/skills/admin-e2e-testing/SKILL.md`
- 前端单元/组件任务读取 `.agents/skills/frontend-unit-testing/SKILL.md`
- Go、PostgreSQL 或 Redis 任务读取 `.agents/skills/backend-testing/SKILL.md`
- API、DTO 或 envelope 任务读取 `.agents/skills/api-contract-testing/SKILL.md`
- `docs/module-map.md`、`docs/task-checklist.md`
- `docs/env.md`、`docs/api.md`、`docs/branching.md`、`CONTRIBUTING.md`

## 技术栈

- 后端：Go + Gin + GORM
- 管理端：React + TypeScript + Ant Design Pro
- 采集服务：Node.js + TypeScript + Playwright
- 数据库：PostgreSQL
- 队列 / 缓存：Redis
- 包管理：pnpm workspace
- 部署：Docker Compose

## 生产维护验收策略

- `.github/workflows/` 是自动化测试的唯一持续执行入口；工作流依赖的前端、Collector、后端、契约、架构、PostgreSQL、Redis 和 Admin E2E 测试必须保留。
- 功能、页面和业务流程的最终签收由人工完成。自动化测试用于回归保护，不替代人工产品验收。
- AI Agent 默认执行与改动直接相关的静态检查、格式检查、配置检查和必要构建；完整自动化测试交由 CI。用户明确要求本地运行时除外。
- 未在本地运行的自动化测试必须如实标记为“交由 CI”，不得声称已经通过。
- 本地不要求创建测试数据库。数据库/Redis 集成测试使用 `TEST_DATABASE_URL`、`TEST_REDIS_URL` 或 CI service container，绝不回退到开发库或生产资源。
- 不新增按阶段、批次或单次验收命名的 gate、fixture 报告、截图报告、运行证据或 `artifacts/` 目录；可复用回归直接加入现有测试套件和工作流。
- Playwright 报告、`test-results/`、截图、临时日志等本地产物完成诊断后清理，不提交 Git。

## 开发规则

- 任何代码新增、修改、重构或 Bug 修复都适用代码质量规范；高风险修改执行深度审查。
- 新模块、跨模块修改、shared/common、adapter、worker/queue/scheduler、migration/repository、公共 API/type 或大型重构都适用模块化架构规范。
- 新代码不得扩大 TypeScript、Go、lint、安全或架构 baseline；不得用 skip、ignore、宽泛 allowlist 掩盖失败。
- Bug 修复优先把回归覆盖加入现有 CI 测试层，不创建一次性门禁。
- 不得以架构优化为名修改 API、payload、权限、readonly、状态机或业务语义。
- Admin UI 变更必须保持五档视口、状态覆盖、根节点无横向溢出和写请求安全；由相关 E2E 工作流与人工验收共同签收。
- 未拦截时不得对真实平台、真实店铺或生产后端执行非 GET 请求。
- 后端遵循 handler → service → provider / repository / queue 分层；第三方能力通过 Provider 扩展。
- 耗时任务使用任务状态和队列，不在 HTTP 请求中长时间同步阻塞。
- 敏感配置加密存储、脱敏展示；日志不得输出完整密钥或 Token。
- 未经用户明确要求，不 commit、不 push、不打 Tag、不发布 Release。
- 不直接在 `main` 开发；日常变更从 `dev` 创建 `feat/*` 或 `fix/*`，通过 PR 合并。

## 文档同步

- 新增环境变量：更新 `.env.example` 和 `docs/env.md`；Docker 使用时同步 `docker-compose.full.yml`。
- 修改启动命令：更新 `README.md`、`README.en.md`、`docs/development.md`。
- 修改部署：更新 `docs/docker-deployment.md`。
- 新增 API / Provider / 队列 / 页面 / 数据表：先查 `docs/module-map.md`，再更新对应文档。
- 较大维护变更：更新 `docs/PROGRESS.md` 和必要的 `CHANGELOG.md`。
- 分支、CI、PR 流程变更：同步 `docs/branching.md`、`CONTRIBUTING.md`、PR 模板。

## 常用检查

本地按影响范围选择静态检查或构建：

```bash
pnpm check:dev
pnpm check:ui-copy --strict
pnpm build:admin
pnpm build:collector
pnpm architecture:check
pnpm workflow:verify
```

核心自动化回归命令由 GitHub Actions 编排，包括：

```bash
pnpm test:frontend
pnpm test:collector
pnpm test:contracts
pnpm test:backend
pnpm test:db:inventory
pnpm test:redis
```

修改 Go 代码时仍需执行 `gofmt`；本地未执行的测试在最终说明或 PR 中标记为交由 CI。

## 禁止事项

- 不提交 `.env`、真实密钥、Token、Cookie、平台凭据或生产数据。
- 不让前端直接调用第三方 AI、平台或存储 API。
- 不默认引入 Kubernetes、Kafka 或复杂微服务治理。
- 不在 MVP 范围默认实现 AI 客服自动外发，必须人工确认。
- 不因“进入生产维护阶段”自动启用真实凭据、真实网络、写入、Worker、重试或灰度开关。

## AI Agent 工作方式

1. 按 `docs/ai-workflow.md` 形成目标、范围、关联入口、验证和风险的最小上下文包。
2. 先确认影响范围再编辑，保持修改小而聚焦。
3. 保留用户已有修改，不擅自回滚。
4. 用 `docs/module-map.md` 与 `docs/task-checklist.md` 收尾。
5. 最终说明改了什么、本地验证了什么、哪些测试交由 CI、还有什么风险。
