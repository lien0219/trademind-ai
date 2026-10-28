# 任务完成检查清单

## 通用

- [ ] 改动聚焦，没有混入无关重构或回滚用户修改。
- [ ] 已按 `docs/module-map.md` 检查代码、配置、Docker、CI 和文档关联。
- [ ] 没有提交 `.env`、真实密钥、Token、Cookie、平台凭据或生产数据。
- [ ] 没有新增阶段 gate、长期运行证据、截图报告或 `artifacts/`。
- [ ] 最终说明区分本地实际检查、交由 CI 的测试和人工验收。

## 代码与架构

- [ ] 新代码未扩大 TypeScript、Go、lint、安全或架构 baseline。
- [ ] 高风险改动已做深度审查；跨模块改动已检查边界和循环依赖。
- [ ] Go 改动已执行 `gofmt`。
- [ ] API、权限、状态机和业务语义没有被意外改变。

## 自动化回归

- [ ] 工作流依赖的核心测试、fixture、Mock 和配置仍然存在。
- [ ] 受影响的 frontend、collector、backend、contract、architecture、PostgreSQL、Redis 或 Admin E2E 已纳入 CI。
- [ ] CI 失败没有通过 skip、only、弱化断言或扩大 allowlist/baseline 掩盖。
- [ ] 未本地执行的测试标记为“交由 CI”，没有声称 passed。

## 人工验收

- [ ] 用户可见功能、文案和业务流程已列出人工验收步骤与结果。
- [ ] Admin UI 适用时检查五档视口、normal/loading/empty/error/readonly 等状态和根节点横向溢出。
- [ ] Admin 用户可见文案变更已运行严格 UI 文案与全站六语言审计；未执行的自动化测试明确交由 CI。
- [ ] 写操作已确认取消 0 请求、确认请求次数和 payload；未访问真实平台或生产后端。

## 数据与环境

- [ ] 本地数据库/Redis 测试只连接显式隔离资源，没有 fallback 到开发或生产环境。
- [ ] 本地 `trademind_test` 可不存在，任务没有擅自创建或重建它。
- [ ] 环境变量变更同步 `.env.example`、`docs/env.md`，Docker 使用时同步 `docker-compose.full.yml`。

## 文档与协作

- [ ] README / README.en、文档中心和当前维护状态保持一致。
- [ ] CI/PR 规则变更同步 `docs/branching.md`、`CONTRIBUTING.md` 和 PR 模板。
- [ ] 较大维护变更已更新 `docs/PROGRESS.md` 与必要的 `CHANGELOG.md`。
- [ ] 未经明确要求没有 commit、push、Tag 或 Release。
