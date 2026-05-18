# 任务完成检查清单

每次完成开发任务前，按改动范围检查本文件。AI Agent 最终说明中应写明执行过的检查，以及未执行的原因。

## 通用检查

- [ ] 修改范围聚焦，没有混入无关重构。
- [ ] 没有提交 `.env`、真实密钥、Token、Cookie、平台凭证或生产数据。
- [ ] 代码、配置、示例、Docker、CI、文档已按 `docs/module-map.md` 同步。
- [ ] 涉及较大模块、阶段性能力或公共契约时已更新 `docs/PROGRESS.md`。

## 后端 Go

- [ ] 在 `backend` 目录执行 `go fmt ./...`。
- [ ] 执行或说明 `go test ./...`。
- [ ] 检查 handler → service → provider / repository / queue 分层。
- [ ] 检查统一响应、错误处理、鉴权上下文和日志脱敏。
- [ ] 新增模型或字段时检查 AutoMigrate、索引、默认值和 `docs/PROGRESS.md`。

## 管理端 Admin

- [ ] 执行或说明 `pnpm build:admin`。
- [ ] 检查 `admin/config/routes.ts` 与页面文件是否一致。
- [ ] 检查 `admin/src/services`、类型定义、表格列、表单校验和状态映射。
- [ ] 用户可见行为变化时补截图、录屏或说明。

## Collector

- [ ] 执行或说明 `pnpm build:collector`。
- [ ] Playwright 行为变化时检查超时、UA、headless 和错误归因。
- [ ] 采集结果结构变化时同步后端 DTO、商品草稿映射和 `docs/api.md`。

## 环境变量 / 配置

- [ ] 更新 `.env.example`。
- [ ] Docker 需要时更新 `.env.docker.example`、`docker-compose.full.yml`。
- [ ] 更新 `docs/env.md`、`docs/development.md`、`docs/docker-deployment.md`。
- [ ] 敏感配置确认加密存储、脱敏展示、日志禁止输出。

## API / 前后端契约

- [ ] 更新 `docs/api.md`。
- [ ] 后端 DTO 改动已同步前端 `services` 和 `types`。
- [ ] 路由、请求方法、分页、排序、状态枚举保持一致。
- [ ] 受保护接口已接入 Bearer 鉴权。

## Provider

- [ ] 按 `docs/provider-template.md` 检查接口、配置、超时、错误处理。
- [ ] 更新 `docs/provider.md`。
- [ ] 连接测试、设置页和脱敏展示已同步。

## Docker / CI

- [ ] Docker 变化执行或说明 `docker compose -f docker-compose.full.yml config`。
- [ ] CI 变化检查分支触发、PR 触发和 `workflow_dispatch`。
- [ ] 依赖变化检查 lockfile、Dependabot 和构建缓存。

## 文档 / 开源治理

- [ ] README 与 README.en 链接和结构保持一致。
- [ ] 新文档已加入 `docs/README.md`。
- [ ] 分支或协作规则变化已同步 `CONTRIBUTING.md` 和 PR 模板。
- [ ] 用户可见版本变化已更新 `CHANGELOG.md`。
