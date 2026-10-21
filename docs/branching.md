# 分支管理与 PR 规则

## 分支角色

| 分支 | 用途 |
| --- | --- |
| `main` | 稳定正式分支，只通过 PR 合并 |
| `dev` | 日常开发集成分支 |
| `feat/*` | 从 `dev` 创建的功能分支，PR 到 `dev` |
| `fix/*` | 常规修复 PR 到 `dev`；紧急线上修复可 PR 到 `main` 并回合 `dev` |
| `release/*` | 从 `dev` 创建的发布准备分支，PR 到 `main` |

未经明确授权，不直接 push、commit、打 Tag 或发布 Release。

## 推荐流程

```bash
git switch dev
git pull --ff-only origin dev
git switch -c feat/your-feature-name
```

PR 必须说明变更范围、风险、本地实际检查、交由 CI 的自动化回归和人工验收结果。

## 合并要求

- `main` 不接受直接 push；`dev` 也优先通过 PR。
- GitHub Actions 是自动化门禁的权威执行入口，失败不得通过 skip、降低断言或扩大 baseline 掩盖。
- 功能和业务流程由维护者人工签收。
- Go 改动执行 `gofmt`；前端/Collector 改动至少完成相关静态检查或构建，完整回归可交由 CI。
- 环境变量、Docker、Provider、API、部署或 CI 变更必须同步相关文档。
- 不提交 `.env`、真实凭据、生产数据或本地测试产物。

## CI 隔离资源

PostgreSQL 和 Redis 集成测试由 GitHub Actions service container 提供。CI 中的 `trademind_test` 是作业内临时资源，本地无需创建或维护同名数据库。任何本地集成测试都必须显式指向隔离测试资源，不得回退到开发或生产服务。

## 发布流程

从 `dev` 更新 changelog、版本号与部署文档并通过既有流程合并到 `main`。容器版本由 `deploy/IMAGE_VERSION` 管理，格式必须为不含 `+build` 元数据、最长 48 字符的 Docker tag 安全 SemVer。main 的镜像候选构建、三镜像整体门禁和人工验收完成后，Container Images 工作流才会创建匹配的 annotated `v<version>` Tag 与 GitHub Release；发布修正应回合 `dev`，避免分支漂移。

## 容器镜像发布

`Container Images` 工作流将 backend、admin、collector 三个独立镜像发布到统一的 `ghcr.io/<owner>/trademind` Package：

- 只有 `main` 分支的镜像相关 push 自动发布；`dev`、`feat/*`、`fix/*`、`release/*` 的 push 不再发布容器镜像。
- main 构建先按服务生成 `<service>-sha-<full-commit>` 候选标签；三个候选都通过平台、OCI revision 和 Trivy 门禁后才移动 `<service>-main`。
- 版本变更或明确的 main 手动发布会把候选 digest 晋级为不可覆盖的 `<service>-main-v<version>`。
- 工作流校验版本文件、已有版本顺序、Tag、main 祖先关系和三个候选 digest；正式版本回退、同名 Tag 指向其他提交或正式镜像已存在且 digest 不同会立即失败。
- 每个镜像包含 `linux/amd64` 与 `linux/arm64`，并发布 OCI 元数据、SBOM 和 provenance；main 晋级前还必须通过工作流内的 Admin/Collector 构建与 Go 单元构建测试门禁。
- 正式镜像发布不会自动部署、切流、修改数据库或启用真实平台能力；它只创建可验证的镜像、Tag 和 Release 部署输入。

版本变更 PR 应同时检查 `deploy/IMAGE_VERSION`、`CHANGELOG.md`、`docs/docker-deployment.md` 和回滚引用。Tag 只能由通过整体门禁的 main 工作流创建；完整命令见 [Docker 部署说明](docker-deployment.md#正式发布)。

## 分支保护建议

- `main` 和 `dev` 禁止直接 push并要求 PR 与 CI。
- 为 `v*` 配置 Tag 保护，禁止发布后强制移动或删除。
- 按维护者偏好使用线性历史或 squash merge。
- 合并后删除无用远程分支。
