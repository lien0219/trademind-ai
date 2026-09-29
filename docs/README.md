# TradeMind 文档中心

项目已进入生产维护阶段。工作区只保留当前开发、部署、运维和人工验收所需文档；历史阶段报告、一次性门禁报告和运行证据不再保存在当前工作树中，必要时从 Git 历史查询。

## 使用与部署

- [本地开发](development.md)
- [Docker 部署](docker-deployment.md)
- [环境变量](env.md)
- [API 契约](api.md)
- [ERP 扩展架构](ERP_ARCHITECTURE.md)
- [AI 客服回复设计](CUSTOMER_AI_REPLY_SUGGESTION_DESIGN.md)
- [客服中心设计](CUSTOMER_SERVICE_CENTER_DESIGN.md)
- [Provider 扩展](provider.md)
- [系统架构](architecture.md)

## 生产运维

- [生产边界](PRODUCTION_CAPABILITY_BOUNDARY.md)
- [预生产架构](PREPRODUCTION_ARCHITECTURE.md)
- [人工验收清单](PRODUCTION_MANUAL_ACCEPTANCE_CHECKLIST.md)
- [风险登记](PRODUCTION_RISK_REGISTER.md)
- [可观测性架构](OBSERVABILITY_ARCHITECTURE.md)
- [数据库回滚边界](DATABASE_ROLLBACK_BOUNDARY.md)
- [灾难恢复计划](DISASTER_RECOVERY_PLAN.md)

## 工程协作

- [AI 工作流](ai-workflow.md)
- [AI 编码规则](ai-coding-rules.md)
- [模块关联索引](module-map.md)
- [任务检查清单](task-checklist.md)
- [分支与 PR](branching.md)
- [当前维护状态](PROGRESS.md)

## 验收约定

- 自动化测试只通过 `.github/workflows/` 持续执行；不得删除工作流依赖的核心测试。
- 功能、页面和业务流程的最终签收由人工按验收清单完成。
- 本地不要求创建测试数据库；CI 使用隔离的 PostgreSQL/Redis service container。
- 不为单次阶段、批次或验收创建持久化 gate、fixture 报告、截图报告或 `artifacts/` 证据。
