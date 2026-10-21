# Contributing to TradeMind

欢迎提交 Bug 修复、功能、文档、Provider、采集规则和部署改进。

## 开发流程

1. Fork 仓库并从 `dev` 创建 `feat/*` 或 `fix/*`。
2. 阅读 `AGENTS.md`、`docs/ai-coding-rules.md`、`docs/module-map.md` 和相关 Skill。
3. 保持改动聚焦，同步代码、配置和文档。
4. 提交 PR 到 `dev`；紧急线上修复按 `docs/branching.md` 处理。

```bash
git switch dev
git pull --ff-only origin dev
git switch -c feat/your-feature-name
```

## 验证与验收

- GitHub Actions 执行持续自动化回归；不得删除工作流依赖的核心测试或通过 skip 掩盖失败。
- 本地默认完成相关静态检查、配置检查、格式化和必要构建，完整自动化回归可交由 CI。
- 产品、页面和业务流程最终由人工验收；PR 中写明人工步骤和结果。
- 本地不要求创建 `trademind_test`。数据库/Redis 测试必须使用隔离资源或 CI service container。
- 不提交 `.playwright-mcp/`、`playwright-report/`、`test-results/`、截图报告、临时日志或阶段证据。

## PR 要求

- 说明变更、影响范围、风险、本地实际检查、交由 CI 的测试和人工验收。
- Go 改动执行 `gofmt`；前端/Collector 改动完成相关构建或说明未执行原因。
- API、部署、环境变量、配置或 Provider 变更同步相关文档。
- 容器版本由 `deploy/IMAGE_VERSION` 管理；版本变更使用不含 `+build` 元数据、最长 48 字符的 Docker tag 安全 SemVer，并同步 Changelog、部署说明和回滚引用。
- 不提交 `.env`、密钥、Token、Cookie、真实平台凭据或生产数据。
- 不新增一次性阶段 gate、长期运行证据或自动扩大的 baseline。

只有镜像相关变更合并到 `main` 后，GitHub Actions 才会把 backend、admin、collector 候选镜像发布到统一的 `trademind` GHCR Package；开发分支、PR 和非 main 手动运行不会写入 GHCR。三个候选都通过平台、OCI revision 和 Trivy 门禁后，普通 main 构建才移动服务 `main` 标签。版本文件变更或明确的 main 手动发布会使用同一提交的 digest 创建不可覆盖的 `<service>-main-v<version>` 正式标签、匹配的 `v<version>` Git Tag 和 GitHub Release。预生产和正式部署必须使用 Release 清单中的 `image@sha256:<manifest-digest>` 引用。镜像发布不代表已经部署或启用真实平台能力。

## Commit Message

建议使用简洁 Conventional Commit：

```text
feat: add storage provider docs
fix: handle collector timeout error
docs: improve deployment guide
chore: simplify maintenance tooling
```

## 代码风格

- 后端：Go + Gin + GORM，业务通过 service 编排，第三方能力走 Provider。
- Admin：React + TypeScript + Ant Design Pro。
- Collector：Node.js + TypeScript + Playwright，不直接操作主业务数据库。
- 数据库：PostgreSQL；安全日志不得输出完整密钥或凭据。

项目使用 Apache-2.0 许可证。衍生作品应保留许可证并注明原项目：<https://github.com/lien0219/trademind-ai>
