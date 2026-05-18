## 变更内容

请简要说明本 PR 做了什么。

## 变更类型

- [ ] Bug 修复
- [ ] 新功能
- [ ] 文档改进
- [ ] 重构
- [ ] 测试
- [ ] 工程维护

## 测试方式

请说明你执行过的验证方式，例如：

- [ ] `pnpm check:dev`
- [ ] `pnpm dev`
- [ ] `go test ./...`
- [ ] `pnpm build:admin`
- [ ] `pnpm build:collector`
- [ ] Docker Compose 启动验证
- [ ] 未执行，原因：

## 影响范围

请说明可能受影响的模块、页面、API、环境变量或部署方式。

## 目标分支

请确认本 PR 符合 [分支管理与 PR 规则](docs/branching.md)：

- [ ] `feat/*` → `dev`
- [ ] `fix/*` → `dev`
- [ ] 紧急修复：`fix/*` → `main`，并计划回合到 `dev`
- [ ] `release/*` → `main`

## Checklist

- [ ] 我已确认没有提交 `.env`、密钥、Token、Cookie 或真实平台凭证。
- [ ] 涉及 Go 代码时已在 `backend` 目录执行 `go fmt ./...`。
- [ ] 我已阅读并遵守 `docs/ai-coding-rules.md`。
- [ ] 涉及接口、环境变量、配置文件、部署或 Provider 机制时已更新文档。
- [ ] 新增或修改环境变量时已同步 `.env.example`；Docker 部署需要时也已同步 `.env.docker.example` 与 `docker-compose.full.yml`。
- [ ] 涉及用户可见行为时已补充截图、录屏或说明。
- [ ] 我已阅读并遵守 `CONTRIBUTING.md`。
