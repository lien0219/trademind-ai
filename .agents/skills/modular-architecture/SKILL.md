---
name: modular-architecture
description: TradeMind 模块化架构、模块边界、循环依赖、Baseline/Ratchet、受影响架构检查和 CI 门禁的唯一完整主规范
---

# TradeMind Modular Architecture 主规范

本 Skill 是 TradeMind 全项目唯一完整的模块化架构规范。其他 Skill、AGENTS、Cursor rules、CI 和文档只能引用本文件，不得复制出第二套完整架构规则。

本规范采用“先描述真实架构，再建立边界，再使用 Ratchet 防止恶化”。本轮不要求一次性修复全部历史架构债务，不以架构优化名义改变 API、payload、权限、状态机、数据库语义或业务行为。

## 1. 自动适用范围

以下任务必须自动读取并遵循本 Skill，用户无需显式说“模块化”“架构设计”或“检查依赖”：

- 新增一级业务模块、完整页面体系、领域模型、repository、数据库领域模型。
- 新增第三方平台 adapter、共享组件体系、共享 service、共享 DTO/type/enum。
- 新增 worker、queue consumer、scheduler/cron、后台任务体系。
- 修改 migration、跨模块 API、shared/common/package、三个或以上业务模块。
- 移动大量文件、拆分大型页面或大型 service、修改模块公共入口。
- 出现循环依赖、跨层调用、shared 反向依赖 feature/page。
- handler 承载大量领域逻辑、repository 决定业务状态、worker 重复 service 逻辑。
- 平台条件判断散落、多处重复 DTO/状态/枚举、单文件职责持续膨胀、大型重构。

## 2. 非自动适用范围

以下小修改一般不触发完整深度架构审查，只执行 code-quality 轻量边界检查：

- 按钮间距、文案、局部样式、单个空值、单个类型错误、一个表格列。
- 单个接口字段映射、单文件内部小范围重构、不改变公共 API 的测试修复、README 文档修改。

但只要小修改引入新跨模块 import、新 shared 依赖、新公共类型、新平台判断、新 repository 调用、新 worker 逻辑或新循环依赖，仍必须触发架构检查。

## 3. 轻量边界检查

所有代码修改至少确认：

- 是否新增跨应用源码依赖、生产代码依赖 tests/e2e/mock/fixture。
- 是否新增 shared/common 反向依赖页面、feature 内部实现、handler、repository 或 worker。
- 是否新增深层 import 内部实现或绕过已有公共入口。
- 是否新增明显跨层调用、运行时循环依赖、公共 DTO/enum 重复定义。
- 是否扩大 `tests/architecture/baselines/module-boundaries.json` 之外的历史违规。

## 4. 深度架构审查

触发深度审查时必须检查：模块职责、公共 API、依赖方向、循环依赖、DTO/type/enum 单一事实来源、adapter/worker/repository 边界、跨应用耦合、baseline/ratchet 变化、与测试和代码质量的联动。

深度审查不等于允许大规模重构。除非任务明确要求并经确认，不得移动大量生产文件、拆分 DraftDetail、重建 Go 分层或修改业务语义。

## 5. 架构问题严重等级

### Critical

必须阻塞完成：生产代码依赖测试 mock；前端 bundle 引入后端或脚本私密实现；跨应用依赖导致生产发布耦合；循环依赖造成初始化错误；shared 反向依赖形成核心循环；worker 绕过业务层执行危险写入；handler 绕过权限或状态机。

### High

默认阻塞新增问题：新运行时循环依赖；新非法跨层依赖；新跨应用源码依赖；新生产代码 import test/e2e/mock；新 handler → repository 越层；新 shared → feature/page 反向依赖；新第三方结构扩散到领域核心；新 worker 重复业务状态机；新模块无明确归属和公共边界。

### Medium

需要修复或解释：深层 import 内部实现；重复 DTO；平台条件判断散落；页面职责明显过多；service 与 transport 耦合；repository 返回页面展示结构；多处重复状态映射；shared 模块职责模糊。

### Advisory

可后续处理：历史大型文件；可拆分但暂不阻塞的模块；命名和目录可优化；公共入口可进一步清晰；未来模块化演进建议。

历史 DraftDetail 文件大小只能作为 Advisory，不能在本轮直接列为 High。

## 6. Baseline/Ratchet

架构 baseline 文件为 `tests/architecture/baselines/module-boundaries.json`。Baseline 只记录稳定、确定、可机器识别的历史违规，不记录本地绝对路径、行列号、时间戳、随机顺序、文件长度建议或主观评价。

规则：

1. 新违规签名阻塞。
2. 已有违规数量增加阻塞。
3. 违规减少通过，并提示可收紧 baseline。
4. baseline 不自动扩大，CI 不更新 baseline。
5. 更新必须显式执行：`pnpm architecture:baseline -- --update`。
6. baseline 只管理历史债务，不代表架构正确。

## 7. 应用级边界

当前真实应用：

- `admin/`：React + TypeScript + Ant Design Pro 管理端。
- `collector/`：Node.js + TypeScript + Playwright 采集服务。
- `backend/`：Go + Gin + GORM 后端。
- `tests/contracts/`：API 契约测试来源。
- `scripts/`：本地、CI 和质量测试编排脚本。

硬规则：Admin 运行时代码不得 import Collector 源码；Collector 运行时代码不得 import Admin 源码；Go 后端不得依赖前端源码；生产应用不得 import e2e/test/mock/fixture；测试可以 import 生产代码；contracts/shared 可以被应用依赖，但不得反向依赖具体应用；scripts 不得被浏览器运行时代码依赖；CI helper 不得进入生产 bundle。

## 8. Admin 前端边界

`admin/src/pages/**` 负责路由级编排，组合 feature/shared UI/service/hooks，不应被 shared UI、service、utils 或 types 反向依赖，不应承载全部 API、状态机、表单和视图逻辑。

`admin/src/components/ui/**`、`TmPageContainer`、`SectionCard`、`MetricCard`、`OperationToolbar`、`TmProTable`、`EmptyState`、`AppDrawer`、`layoutTokens` 应保持领域无关或低领域耦合，不 import pages，不直接调用具体业务 API，不绑定具体平台状态机或页面内部状态。

`admin/src/services/**` 负责 API 调用和传输边界，不 import pages，不 import 视觉组件，不包含页面布局逻辑，不依赖 React 组件实例，不决定复杂业务状态机，request/response 类型必须清晰。

通用 utils 不依赖页面、React 组件或具体 feature 内部实现，除非放在 feature 内；应保持确定性、可测试性，不承载隐藏全局状态。

如果已有 feature/domain 结构，领域内部可依赖 shared；领域之间通过稳定公共入口，不建立隐式双向依赖。本轮不强制将全部页面迁移到 feature-based 目录。

## 9. DraftDetail 特别规则

商品草稿详情页属于历史复杂模块。本轮不直接拆分 DraftDetail，不改变七个 Tab，不修改业务状态机、API/payload，不移动大量文件，不为架构评分重构生产代码。

未来拆分原则：页面入口只负责路由和顶层编排；七个 Tab 独立模块；通用 publish 状态和操作明确归属；Douyin 平台逻辑放入平台模块；inventory/readiness/publish 状态避免互相隐式修改；请求 payload 构造与 UI 分离；平台适配逻辑不得继续散落。

## 10. Collector 边界

Collector 以现有 `collector/src/providers/**`、`browser/**`、`normalizer/**`、`tasks/**`、`types/**`、`config/**` 为基础：

- source adapter 负责来源接入，不修改全局核心规则。
- fetch/client 负责网络，parser 负责字段解析，normalize 负责标准化，quality score 负责评分，dedup/output/persistence 负责输出，orchestration 负责编排。
- 网络请求与业务转换分离；评分尽量保持纯函数。
- 不在多个 source adapter 重复实现同一价格归一化。
- 第三方原始结构限制在 adapter 边界，外部数据在边界归一化。
- Collector 不依赖 Admin 页面代码，测试 fixture 不进入生产运行时。

## 11. Go 后端边界

当前后端以 `backend/internal/modules/**`、`backend/internal/providers/**`、`backend/internal/api/**`、`backend/internal/rdb/**`、`backend/internal/pkg/taskqueue/**` 组织。

目标方向：handler/controller 负责 HTTP 输入输出、参数解析和边界校验，调用 service/usecase，不直接写复杂 SQL、不承载完整业务状态机、不操作第三方 SDK 细节。

service/usecase 负责业务规则、状态转换和事务编排，依赖 repository 和抽象 adapter，不依赖 HTTP request/response 类型，不生成页面展示结构。

repository 负责持久化，不决定业务状态是否允许，不依赖 HTTP handler，不承载第三方平台逻辑。

model/domain 表达领域实体和值，不依赖 transport 或具体数据库连接，避免领域类型与 HTTP DTO 完全混用。

adapter/client 隔离抖店、TikTok、Shopee、Lazada、Amazon、AI Provider、图片 Provider、存储 Provider、OCR、email、collector 等第三方能力；不将第三方响应结构扩散到 service 之外，不直接决定核心业务状态。

worker/job 调用 service/usecase，不重复实现业务规则，负责任务生命周期和重试编排，不绕过 service 修改数据库状态。

如当前包没有严格分层目录，本轮不大迁移；新代码遵循目标方向，历史例外用 baseline 或 Advisory 管理。

## 12. API Contract 边界

本仓库当前以 `tests/contracts/**`、后端 route/handler/service 和 Admin service/mock 的契约测试维护 API 真实来源。API contract 变更必须检查 method、URL、query、payload、response envelope、error envelope、enum、nullable、pagination、permission、readonly、idempotency。

契约可通过 OpenAPI、JSON Schema、契约 fixture、生成类型或稳定枚举共享，但不得复制大量契约后不建立一致性测试。浏览器与 Go 不强制直接共享同一个数据库结构类型。

## 13. Shared/Common 边界

Shared 是高风险区域。任何新增 shared/common 必须回答：是否真正被多个模块复用；是否稳定通用；是否携带业务领域概念；是否应该留在 feature 内；是否形成反向依赖；是否扩大公共 API；是否有测试；是否导致跨应用耦合。

禁止把单页面专用逻辑、单平台专用判断、单接口专用 DTO、临时 workaround、只被一个模块调用的复杂 service、业务状态机、数据库 repository 放入 shared。

shared 不得依赖 pages、feature 内部文件、HTTP handler、repository、worker、E2E/test/mock。

## 14. 第三方平台 Adapter

抖店、TikTok、Shopee、Lazada、Amazon、AI Provider、图片 Provider、存储 Provider、OCR、email 等必须通过明确 adapter/client 隔离。

第三方 SDK 和响应结构限制在 adapter 层；service 使用内部归一化模型；平台错误转换为内部错误类型；平台条件判断不散落在页面、repository 和 worker；新平台通过新增 adapter 扩展；不在通用 service 中持续增加巨大 switch；平台能力差异通过 capability/config 表达；adapter 不直接决定用户权限或 UI 状态；测试使用 fake adapter。

新增平台自动触发 modular-architecture、code-quality 深度审查、backend-testing、api-contract-testing、project-testing。

## 15. Worker/Queue/Scheduler

Redis client 封装位于基础设施层；业务模块不散落原始 Redis 命令；分布式锁封装统一；queue producer 不决定 consumer 业务细节；consumer 调用 service/usecase；scheduler 只负责触发，不重复业务逻辑；job payload 使用明确版本或稳定结构；job 不依赖 HTTP handler；retry 和幂等策略由任务模块明确；shared queue 模块不得反向依赖具体页面或 handler。

新增 worker/queue/scheduler 自动触发完整架构审查。

## 16. 数据库和 Repository

migration 只负责 schema/data migration，不依赖应用运行时 service。repository 负责数据库访问；handler 不直接操作数据库；worker 不绕过 service 执行业务写入；database model 不直接成为所有 API response；状态机校验不能只依赖数据库 enum；transaction 边界由 service/usecase 明确控制；多 repository 操作必须有清晰事务归属。migration 修改必须触发深度架构审查。

## 17. 依赖方向

默认方向：应用入口/路由 → handler/page orchestration → service/usecase → repository/adapter/infrastructure → model/domain/shared primitives。shared/common/contracts 只能被上层依赖，不能反向依赖具体应用实现。测试依赖生产代码，生产代码不得依赖测试。

## 18. 循环依赖

TypeScript/JavaScript 循环依赖检查覆盖 `admin/src`、`collector/src`、`scripts` 中可复用模块和其他实际 TS workspace，忽略 node_modules、dist、build、coverage、test-results、playwright-report、generated、测试 fixture 图。

必须区分运行时依赖环和 type-only 依赖环：运行时循环默认阻塞；纯 type-only 循环根据风险标记 High 或 Medium，不与运行时循环等同。

Go import cycle 已由编译器阻塞，但仍需检查层级依赖方向。

## 19. 跨层调用

禁止新增 handler → repository 越层、worker → handler、repository → handler/service、shared → pages/feature/internal、service → HTTP request/response 私有类型、adapter → UI 状态等方向。历史例外如机器可识别且稳定，可进入 baseline；否则列为 Advisory 并在后续重构时收敛。

## 20. 模块公共 API

模块之间优先通过稳定入口访问，例如 `index.ts`、明确 exported interface、Go 对外导出 package API、contract schema。不得从其他模块深层目录直接 import 内部实现、private helper、页面内部 component、测试 fixture 或 adapter 内部 response type。

不要为了形式给每个目录创建无意义 `index.ts`；公共入口只在模块边界真实需要时建立。

## 21. DTO、类型和枚举

检查是否重复定义 API DTO、平台枚举、发布/readiness/库存状态、字符串常量映射；transport DTO 是否直接充当 domain model；第三方 DTO 是否扩散；前端展示状态是否错误反向影响后端领域状态。

优先建立单一事实来源，但不强制浏览器与 Go 直接共享无法兼容的源文件。

## 22. 新模块创建规范

新增模块前说明职责、归属应用、边界、公共入口、依赖方向、DTO/enum 来源、测试策略、受影响检查。新增模块必须能被 `tests/architecture/module-boundaries.json` 的 roots 和 rules 表达，不能依赖临时宽泛豁免。

## 23. 修改现有模块规范

小修改保持最小 diff，不强制重建目录。跨模块修改必须说明调用方向和公共入口。修改公共 API/type/enum 时同步 API contract、测试和受影响架构检查。

## 24. 大型文件和职责膨胀

大型文件不是自动错误。只有当文件继续承载新业务域、新平台分支、新状态机、新 repository 调用或新公共 API 时才升级为 Medium/High。历史大型文件作为 Advisory，后续按业务风险逐步拆分。

## 25. 模块拆分判断标准

满足以下多项才建议拆分：职责可被清晰命名；调用方向稳定；拆分后减少跨层知识；测试边界更清楚；不改变业务语义；不需要大规模无关迁移；公共入口真实存在。

## 26. 过度抽象防护

禁止为了“模块化”引入无必要接口、工厂、抽象层、空 index、万能 shared service。三个相似调用优先保持直接清晰，除非已有真实跨模块稳定复用。

## 27. 与 code-quality 联动

任何触发本 Skill 的变更必须同时遵循 `.agents/skills/code-quality/SKILL.md`。code-quality 负责类型、安全、错误处理、diff hygiene 和深度质量风险；modular-architecture 负责模块职责、依赖方向、循环依赖和 baseline ratchet。

## 28. 与 project-testing 联动

任何触发本 Skill 的变更必须同时遵循 `.agents/skills/project-testing/SKILL.md`。架构边界脚本测试通过 `pnpm architecture:test`，受影响门禁通过 `pnpm architecture:affected`。

## 29. 与 frontend-design 联动

大型页面拆分、共享 UI 边界、跨页面模块设计和 DraftDetail 演进必须同时遵循 `.agents/skills/frontend-design/SKILL.md`。UI 视觉、响应式和共享组件细节不在本 Skill 重复。

## 30. 与 backend-testing 联动

handler/service/repository/adapter/worker 边界变化必须同步 `.agents/skills/backend-testing/SKILL.md`，覆盖状态机、事务、幂等、Redis/queue、第三方 fake adapter 和 database integration。

## 31. 与 api-contract-testing 联动

API、DTO、envelope、公共类型、contract fixture 或 Admin service/mock 变化必须同步 `.agents/skills/api-contract-testing/SKILL.md`，确保前后端契约一致。

## 32. 禁止项

禁止自动 commit/push；禁止未经确认移动大量生产文件；禁止为架构评分修改业务语义；禁止自动扩大 baseline；禁止宽泛 allowlist 掩盖违规；禁止引入大型第三方架构扫描工具；禁止生产代码依赖测试、mock、e2e、fixture；禁止 Go 后端依赖前端源码；禁止 Admin/Collector 运行时代码互相 import。

## 33. 完成报告格式

最终报告列出：当前分支、开始工作区状态、应用和模块审计、Admin/Collector/Go/Shared/Adapter/Worker 当前结构、循环依赖数量、跨层违规数量、跨应用违规数量、Skill 路径、触发/非触发条件、AGENTS/Cursor/其他 Skills 更新、module-boundaries 配置、架构文档、各边界规则、TS 循环实现、Go 依赖实现、baseline 路径和数量、baseline 更新命令、新违规阻塞方式、违规减少处理、脚本和测试文件、测试数量、package scripts、quality/test affected 联动、CI 修改、是否新增依赖、是否修改生产代码、实际运行命令结果、Critical/High/Medium/Advisory、修改文件清单、diff stat、未提交文件、敏感文件/测试产物检查、是否扩大 baseline、后续触发场景、是否适合签收/commit/push。
