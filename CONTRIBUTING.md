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

- GitHub Actions 执行持续自动化回归；不得删除工作流依赖的核心测试或通过 skip 掩盖失败。容器镜像变更还必须分别通过 AMD64/ARM64 manifest 的漏洞和 secret 阻断扫描，诊断产物不得包含原始 secret 内容。
- 本地默认完成相关静态检查、配置检查、格式化和必要构建，完整自动化回归可交由 CI。
- Admin 用户可见文案变更运行 `pnpm check:ui-copy --strict`；六语言覆盖使用 `node admin/scripts/i18n-audit.mjs --strict --details`，增量检查不能替代全站严格审计。
- 产品、页面和业务流程最终由人工验收；PR 中写明人工步骤和结果。
- 本地不要求创建 `trademind_test`。数据库/Redis 测试必须使用隔离资源或 CI service container。
- 常规 `go test ./...` 与 `pnpm test:backend` 不启用 PostgreSQL 集成套件；使用 `pnpm test:db` 或库存/Race 命令时，必须同时提供 `APP_ENV=test` 和隔离的 PostgreSQL/Redis URL。修改 affected-quality 编排时，也要检查其作业内的 service container 和环境继承。
- RBAC/OAuth 修改需覆盖审计回滚、撤权、租户隔离与单次 state；`project-tests.yml` 使用 Redis service 执行 OAuth callback/state Race，不能以未注入 URL 后的 skip 作为通过证据。
- 不提交 `.playwright-mcp/`、`playwright-report/`、`test-results/`、截图报告、临时日志或阶段证据。

## PR 要求

- 说明变更、影响范围、风险、本地实际检查、交由 CI 的测试和人工验收。
- Go 改动执行 `gofmt`；前端/Collector 改动完成相关构建或说明未执行原因。
- API、部署、环境变量、配置或 Provider 变更同步相关文档。
- 正式版本由 `deploy/IMAGE_VERSION` 管理；发布新版本仍需主动更新该文件，使用不含 `+build` 元数据、最长 48 字符的 Docker tag 安全 SemVer，并同步 Changelog、部署说明和回滚引用。
- 不提交 `.env`、密钥、Token、Cookie、真实平台凭据或生产数据。
- 不新增一次性阶段 gate、长期运行证据或自动扩大的 baseline。

日常开发先通过 PR 集成到 `dev`，再通过 PR 合并到 `main`。`main` 的提交命中 `Container Images` 工作流路径规则时，GitHub Actions 会分别构建 backend、admin、collector 候选镜像并发布到统一的 `trademind` GHCR Package；未命中路径规则的提交不会自动构建镜像，开发分支、PR 和非 `main` 手动运行不会写入 GHCR。三个候选都通过质量、双架构、OCI revision、Trivy、SBOM、Provenance 和 Digest 校验后，才会更新 `<service>-main` 标签。

当前版本首次通过完整镜像发布门禁且尚未正式发布时，将自动生成不可变正式镜像标签、匹配的 Git Tag 与 GitHub Release。即使此前版本构建失败，后续修复成功也会自动补发。已发布版本不会被后续提交覆盖。正式镜像标签为 `<service>-main-v<version>`，Git Tag 为 `v<version>`；自动发布不要求版本文件在本次 push 中发生变化。发布或补发仍会校验已有 Tag 的提交和正式镜像 digest，冲突时阻断，禁止覆盖。当前版本已发布后的普通 `main` 提交仅更新 SHA / main 镜像标签。

预生产和正式部署必须使用 Release 清单中的 `image@sha256:<manifest-digest>` 不可变引用。CI 通过或镜像发布不代表生产部署已完成，也不会自动切流、修改数据库或启用真实平台能力；产品和业务流程仍须人工验收。

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
