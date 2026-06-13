# 分支管理与 PR 规则

本文定义 TradeMind 的标准开源分支管理方式。日常开发请遵守以下规则，避免直接在稳定分支上开发。

## 分支角色

| 分支 | 角色 | 规则 |
| --- | --- | --- |
| `main` | 正式展示分支、GitHub 默认分支 | 保持稳定、可运行；不直接开发；所有变更通过 Pull Request 合并 |
| `dev` | 日常开发集成分支 | 功能先合并到 `dev`；测试稳定后再 PR 到 `main` |
| `feat/*` | 功能开发分支 | 从 `dev` 创建，完成后 PR 到 `dev` |
| `fix/*` | Bug 修复分支 | 一般从 `dev` 创建；线上紧急问题可从 `main` 创建 |
| `release/*` | 发版准备分支 | 从 `dev` 创建，用于版本测试、版本号、changelog 与发布检查 |

## 命名规范

功能分支：

```text
feat/ai-product-tools
feat/multi-platform-erp
feat/docker-deployment
feat/docs-open-source
feat/sku-recommendation
```

修复分支：

```text
fix/docker-start-error
fix/readme-link
fix/collector-timeout
```

发版分支：

```text
release/v0.1.0
release/v0.2.0
```

## 推荐开发流程

从 `dev` 更新本地代码：

```bash
git switch dev
git pull --ff-only origin dev
```

创建功能分支：

```bash
git switch -c feat/your-feature-name dev
```

推送功能分支：

```bash
git push -u origin feat/your-feature-name
```

功能完成后：

1. 提交 Pull Request 到 `dev`。
2. 等待 CI 通过。
3. 完成代码审查。
4. 合并到 `dev`。

## PR 合并规则

| PR 类型 | 来源分支 | 目标分支 | 说明 |
| --- | --- | --- | --- |
| 功能开发 | `feat/*` | `dev` | 所有新功能先进入开发集成分支 |
| 普通修复 | `fix/*` | `dev` | 常规 Bug 修复先进入 `dev` |
| 紧急修复 | `fix/*` | `main` | 仅用于线上紧急问题；合并后必须回合到 `dev` |
| 发版准备 | `release/*` | `main` | 从 `dev` 创建，测试稳定后合并到 `main` |
| 发版回合 | `release/*` 或 `main` | `dev` | 发版修正合并回 `dev`，避免分支漂移 |

## 合并要求

- `main` 不接受直接 push。
- `dev` 建议不直接 push，优先通过 PR 合并。
- PR 必须描述变更内容、测试方式和影响范围。
- 涉及后端 Go 代码时，必须执行 `go fmt ./...`。
- 涉及前端或采集服务时，至少执行对应构建命令：

```bash
pnpm check:ui-copy --strict
pnpm build:admin
pnpm build:collector
```

- 涉及 Docker、环境变量、Provider、接口或部署流程时，需要同步更新文档。
- 不允许提交 `.env`、真实密钥、Token、Cookie、平台凭证或生产数据。

## 发版流程

从 `dev` 创建发版分支：

```bash
git switch dev
git pull --ff-only origin dev
git switch -c release/v0.1.0
git push -u origin release/v0.1.0
```

在 `release/*` 中完成：

- 版本测试
- changelog 整理
- 版本号调整
- 部署文档核对
- 关键路径验证

稳定后：

1. PR：`release/*` → `main`
2. 打 tag（如适用）
3. PR：`main` 或 `release/*` → `dev`

## GitHub 分支保护建议

建议在 GitHub 仓库设置中启用：

- `main`：禁止直接 push，要求 Pull Request，要求 CI 通过。
- `dev`：建议禁止直接 push，要求 Pull Request，要求 CI 通过。
- 要求线性历史或 squash merge（按维护者偏好选择）。
- 删除已合并分支，保持远程分支列表清晰。
