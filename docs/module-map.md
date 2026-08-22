# 模块关联索引

本文件用于帮助开发者和 AI Agent 判断“改一个点时还要检查哪些关联内容”。遇到不确定的改动，先查本表，再读取对应代码和文档。

## 通用原则

- 改代码时同步检查配置、示例、Docker、CI、文档和前端展示。
- 改公共契约时同步检查调用方，不只改定义处。
- 涉及敏感字段时同步检查加密、脱敏、日志和 `SECURITY.md`。
- 涉及较大模块或阶段性能力时同步更新 `docs/PROGRESS.md`。

## 关联检查表

| 改动类型 | 必须检查 / 同步 |
| --- | --- |
| 环境变量 | `.env.example`、`docker-compose.yml`、`docker-compose.full.yml`、`docs/env.md`、`docs/development.md`、`docs/docker-deployment.md` |
| 启动命令 / pnpm 脚本 | `package.json`、`README.md`、`README.en.md`、`docs/development.md`、`.github/workflows/*.yml` |
| Docker 部署 / 镜像发布 | `deploy/IMAGE_VERSION`、`docker-compose.full.yml`、服务 Dockerfile、`.env.example`、`docs/env.md`、`docs/docker-deployment.md`、`.github/workflows/docker.yml`、`.github/workflows/container-images.yml` |
| 后端 API | `backend/internal/api`、对应 handler/service/dto、`docs/api.md`、`admin/src/services`、`admin/src/types`、相关页面 |
| 统一返回 / 错误码 | `backend/internal/pkg/response`、所有调用方、`docs/api.md`、前端错误处理 |
| 管理端页面 | `admin/config/routes.ts`、`admin/src/pages`、`admin/src/services`、`admin/src/types`、README 能力描述、相关 docs |
| 数据库模型 / 自动迁移 | `backend/internal/modules/**/model`、`backend/internal/database`、`docs/architecture.md`、`docs/PROGRESS.md` |
| ERP 仓库 / 库存账 / 订单库存 / 供应商 / 采购入库 | `backend/internal/modules/warehouse`、`supplier`、`procurement`、`inventory/warehouse_stock.go`、`inventory/warehouse_ledger.go`、`inventory/order_inventory.go`、`inventory/order_inventory_effect.go`、`inventory/warehouse_transfer.go`、`inventory/stocktake.go`、`order`、`ordersync`、`admin/src/pages/Procurement`、`admin/src/pages/Inventory/WarehouseLedger`、`admin/src/pages/Inventory/WarehouseTransfers`、`admin/src/pages/Inventory/Stocktakes`、`admin/src/pages/Orders`、`admin/src/services/procurement.ts`、`admin/src/services/inventory.ts`、`admin/src/services/orders.ts`、`admin/config/routes.ts`、数据库迁移、`adminperm`、API 契约、Admin E2E、`docs/ERP_ARCHITECTURE.md`；必须检查 tenant scope、仓库必选/平台默认仓、单订单单仓、金额最小单位、状态机 revision、收货/调库/订单库存幂等、盘点快照与差异过账、预占/释放/出库/回补事务、库存 effect 和 movement 不可变性、已有 effect 后订单行/仓库锁定、平台同步行 ID/数量保留、历史库存默认仓或待分配仓迁移、对账、权限/只读写请求安全和旧 `product_skus.stock` 兼容边界。 |
| 异步任务 / 队列 | 任务 model/service/worker、Redis 配置、健康检查、`.env.example`、`docs/env.md`、任务中心页面 |
| AI Provider | `backend/internal/providers`、AI settings、Prompt 模板、调用记录、`docs/provider.md`、`docs/provider-template.md` |
| Storage Provider | Provider 接口、文件上传 API、settings.storage、本地/对象存储文档、`docs/provider.md` |
| Image Provider | 图片任务、队列、settings.image、任务页面、`docs/provider.md` |
| Platform Provider | 店铺授权、Token 加密、平台配置、订单/库存/客服调用方、`docs/provider.md`、`SECURITY.md` |
| AI 客服自动回复 | `customerchat` tenant setting/policy/run model、`customersync` 动态轮询与入站触发、独立 Redis Worker、平台客服 Provider、AI Prompt/Gateway、失败事件、操作日志、Admin 运行设置与店铺策略页、`.env.example`、`docs/env.md`、`docs/CUSTOMER_AI_REPLY_SUGGESTION_DESIGN.md` |
| 告警中心 / 外部通知 | 业务告警 `backend/internal/modules/taskcenter`、系统告警 `backend/internal/modules/alerting`、settings `alert_notify`、Admin `TaskCenter/Alerts`、可观测性概览、`adminperm`、`docs/api.md` |
| 可观测性 / 指标入口 / SLO | `backend/internal/pkg/metrics` 结构化快照、`backend/internal/modules/alerting` 窗口告警、`backend/internal/modules/observabilitymod` 概览与 SLO、可信代理/CIDR 配置、`deploy/nginx/*.conf`、Admin `Ops/Observability`、API 契约、`.env.example`、`docs/env.md`、`docs/docker-deployment.md` |
| 数据库备份 / 灾难恢复 | 云数据库自动备份、加密、保留、PITR 与告警，运维平台隔离恢复演练、RPO/RTO 证据和审批；TradeMind 仅保留 `docs/DISASTER_RECOVERY_PLAN.md`、数据库回滚边界、部署级预生产脚本与人工验收门槛，不提供 Admin/API 管理能力 |
| 多平台 / 批量刊登 | `backend/internal/modules/productpublish`、`backend/internal/database/migrate_product_publish_tenant.go`、`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`、`docs/PUBLISH_BATCH_MIGRATION.md`、`docs/api.md`（batch-targets / batches）、`admin/src/pages/Product/PublishBatch*`、`admin/src/pages/Product/PublishTasks`、`admin/src/constants/publishLabels.ts`、`admin/src/constants/publishLimits.ts`、`adminperm`、相关 CI 测试；修改时检查 tenant/store scope、权限、事务与 staging/production fail-closed。 |
| 运营任务真实抖店草稿写 | `backend/internal/modules/operationtask` 冻结草稿/审批/尝试/outbox、`backend/internal/modules/productpublish` 唯一 Worker 写链与恢复、`backend/internal/modules/productioncontrol` allowlist/gray/kill switch、`backend/internal/config` L3 fail-closed 校验、`backend/internal/providers/platform/douyinshop`、Redis 与任务回收器、Admin 运营任务页、API 契约、PostgreSQL/Redis/Admin E2E CI、`docs/env.md`、`docs/api.md`、两份人工验收清单 |
| Collector Provider | `collector/`、`backend/internal/modules/collect`、采集任务 API、队列、raw 原始数据、`COLLECTOR_SERVICE_TOKEN`、HTTP 请求限制、URL/DNS/WebSocket 出站保护、Docker 内部网络、`docs/provider.md`、**1688 改解析时必读 [`docs/collector-1688-pitfalls.md`](collector-1688-pitfalls.md)** |
| 安全 / 密钥 / Token | 加密、脱敏、日志、环境模板、`SECURITY.md`、相关 settings 文档 |
| CI / 分支 / PR 流程 | `.github/workflows`、`docs/branching.md`、`CONTRIBUTING.md`、`.github/PULL_REQUEST_TEMPLATE.md` |
| 开源治理 | `README.md`、`README.en.md`、`docs/README.md`、`CHANGELOG.md`、`.github/*` |
| AI 工作流 / Agent 规则 | `AGENTS.md`、`docs/ai-workflow.md`、`docs/ai-coding-rules.md`、`docs/README.md`、`docs/cursor-rules-usage.md`、`.cursorrules`、`.cursor/rules/README.md`、必要时新增或更新 `.cursor/rules/*.mdc`、`CONTRIBUTING.md`、PR 模板 |

## 前后端联动

后端 API 或 DTO 变化时，必须检查：

- `admin/src/services/**` 的请求路径、方法、参数、响应字段。
- `admin/src/types/**` 或页面内类型定义。
- ProTable / ProForm 字段名、枚举、状态文案和空状态。
- `docs/api.md` 的接口契约。

## 配置联动

新增配置时优先判断配置归属：

- 部署级固定配置：进入唯一模板 `.env.example`，Docker 需要时同步 Compose。
- 可变业务配置：优先进入 settings 表和后台设置页。
- 敏感业务配置：存库时必须 AES-GCM 加密，前端展示必须脱敏。

## 文档联动

- README 只保留首页重点和入口。
- 细节放入 `docs/`，并在 `docs/README.md` 增加入口。
- 新增 AI 规则或关联说明时，同步 `AGENTS.md`、`docs/ai-workflow.md` 和 `.cursor/rules/README.md`。
- 重复出现的坑、质量门槛或工具协作经验，应写回对应 pitfalls、模块文档、`docs/PROGRESS.md` 或 AI 规则，避免只停留在单次对话。
# 预生产基础设施

Changes to `.env.example`, `deploy/preproduction/**`, or `deploy/scripts/*preproduction*` must be checked together with `docs/PREPRODUCTION_ARCHITECTURE.md`, `docs/env.md`, `docs/docker-deployment.md`, workflow configuration, sensitive-diff checks, and the manual acceptance checklist. Production resources and credentials are outside the default writable scope.

## Production Credential / Read-only / Control Modules

Changes under `backend/internal/modules/platformcredential`, `inventoryread`, or `productioncontrol` must be checked with backend routing, migration, `adminperm`, metrics/redaction, config validation, API/provider/security docs, environment templates, CI regression, and `PRODUCTION_MANUAL_ACCEPTANCE_CHECKLIST.md`. `inventoryread` may depend on exported inventory Provider/calibration/audit contracts. The only approved production-write architecture is operation task -> immutable reviewed draft -> transactional outbox -> product-publish Worker -> Douyin platform draft. Direct Douyin create, traditional publish, multi-target/batch paths and legacy retry APIs must fail before writes. Online publish, inventory mutation, automatic business retry and multi-shop expansion require separate product approval and are not implied by L3.
