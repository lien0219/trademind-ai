## 变更内容

请简要说明本 PR 做了什么、影响哪些模块以及主要风险。

## 变更类型

- [ ] Bug 修复
- [ ] 新功能
- [ ] 文档
- [ ] 重构
- [ ] 测试 / CI
- [ ] 工程维护

## 本地检查

只勾选实际执行成功的项目：

- [ ] 静态检查 / 格式检查
- [ ] `pnpm check:ui-copy --strict`
- [ ] `pnpm build:admin`
- [ ] `pnpm build:collector`
- [ ] `gofmt`
- [ ] Docker Compose 配置检查
- [ ] 未执行本地检查，原因：

## GitHub Actions

- [ ] 受影响的核心自动化回归仍由工作流覆盖
- [ ] PostgreSQL / Redis 测试使用 CI service container
- [ ] 调用数据库/Worker 回归的作业（含 affected quality）已配置 `APP_ENV=test` 和两个隔离 URL；单元与显式集成套件的覆盖均保留
- [ ] RBAC / OAuth 改动保留 callback/state、审计失败回滚、旧 Token 撤权和租户隔离回归；最终 SHA CI 无证据时标记 BLOCKED
- [ ] 未删除工作流依赖的测试、fixture、Mock 或配置
- [ ] 没有用 skip、弱化断言或扩大 baseline 掩盖失败
- [ ] Docker 变更已记录 AMD64/ARM64 构建、revision/digest、Trivy 漏洞/secret、SBOM/provenance 和发布门禁结果；未附原始扫描 JSON

## 人工验收

请写明验收步骤和结果：

- [ ] 功能 / 业务流程
- [ ] UI 五档视口、状态与 overflow（如适用）
- [ ] 写请求安全与请求次数（如适用）
- [ ] 不适用，原因：

## 文档与安全

- [ ] 已按 `docs/module-map.md` 同步关联内容
- [ ] 环境变量、API、Provider、Docker、部署或 CI 变更已同步文档
- [ ] 如需发布新容器版本，已更新 `deploy/IMAGE_VERSION`、Changelog 和不可变镜像/回滚引用；合并、CI 与人工验收完成前不创建正式 Tag
- [ ] 没有提交 `.env`、密钥、Token、Cookie、真实凭据或生产数据
- [ ] 没有提交 Playwright 报告、测试结果、截图报告、临时日志或阶段证据
- [ ] 已按 `docs/task-checklist.md` 完成自查

## 目标分支

- [ ] `feat/*` → `dev`
- [ ] `fix/*` → `dev`
- [ ] 紧急修复 `fix/*` → `main`，并计划回合 `dev`
- [ ] `release/*` → `main`
