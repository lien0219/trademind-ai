# 用户与权限安全检查及人工验收

更新时间：2026-10-24。权限定义和全部 79 个权限键的覆盖矩阵见 [RBAC 设计](RBAC_PERMISSION_DESIGN.md)。PASS 只表示下述代码及本地回归证据，不表示已完成远程 CI 或人工验收。

## 开发状态

| 项目 | 状态 | 实现与验证范围 |
| --- | --- | --- |
| P0-1 用户、角色、店铺、重置密码审查 | BLOCKED | 关键入口已修复并回归；原 Codex Security 快照审查未完成封存，不能宣称仓库全量深审已通过 |
| P0-2 审计静默失败 | PASS | 用户/角色/授权/密码/配置/OAuth/会话及全局密钥轮换控制的敏感成功记录在事务中写入，缺审计服务和插入失败回滚 |
| P0-3 可靠持久化与 fail closed | PASS | 采用同库事务；完整前后快照、请求追踪、v2 操作者/会话哈希链；缺失哈希、前驱篡改及审计故障注入验证 |
| P0-4 敏感 API 服务端权限与租户 | BLOCKED | 本次影响入口和 IDOR/店铺隔离测试通过；最终差异的完整安全扫描仍未完成 |
| P0-5 未知、停用、跨租户角色 | PASS | 角色白名单、同租户定义、禁用角色和禁止委派权限均有后端/前端回归 |
| P0-6 Access/Refresh/历史会话撤权 | PASS | SQL 安全版本、session 绑定、Refresh reuse、降权/禁用/角色/授权/密码变更覆盖；登出与审计原子提交，五个平台 OAuth 过期刷新不能覆盖撤销 |
| P0-7 最后有效管理员并发保护 | PASS | PostgreSQL 并发降权、禁用与降权的两个 Race 用例；没有用户 DELETE API |
| P0-8 高危门禁、错误与审计 | PASS | 影响入口拒绝返回 403/404，审计不可用明确 500；公开自助注册提权入口已关闭；全局密钥轮换限定 tenant 0 系统管理员且不可委派 |
| P1-1 用户创建、编辑、启停、密码重置 | PASS | HTTP/事务失败回归及 Admin E2E；临时密码只返回一次 |
| P1-2 首次强制改密 | PASS | 新建/重置后旧凭据不能登录；先改密再签发会话；改密审计失败回滚 |
| P1-3 自定义角色增改停启 | PASS | 禁止编辑系统角色，停用角色不可分配，重启角色不复活旧 Token |
| P1-4 成员数量与权限同步 | PASS | 租户内成员统计；修改角色增加全体成员版本并失效本地缓存 |
| P1-5 单/多店授权与撤权 | PASS | 全量替换、清空、跨租户拒绝及数据库错误整笔回滚 |
| P1-6 查询、变更记录与脱敏 | PASS | 审计前后快照不截断；日志不保存密码/Token；权限和店铺隔离回归；完整性验证按认证租户查询 |
| P1-7 异常、故障、并发、重复提交 | PASS | 参数错误、审计/元数据失败、Refresh 并发、最后管理员和前端重复点击覆盖；库存真实锁升级死锁及 AI 批次冲突/缺失归属回归 |
| P2-1 菜单、页面、操作控制 | PASS | 用户与权限页面、运维菜单/深链、未知角色/空权限及五档视口覆盖；全站业务视觉签收仍由人工执行 |
| P2-2 前端与后端双重授权 | PASS | 隐藏/禁用仅用于体验；HTTP、IDOR、店铺范围测试验证后端实际拒绝 |
| P2-3 角色树、预览与差异 | PASS | 创建/编辑、分组权限树、成员数、角色预览及差异对比 |
| P2-4 权限不足提示 | PASS | 无权页面拒绝、失败消息、角色目录失败时禁用并支持重试；无全局密钥权限时禁用控制且不发出引用查询 |
| P2-5 六类身份 E2E | PASS | admin/operator/readonly/reviewer/custom/无权限，以及未知角色伪造 grants |
| P2-6 关键流程 E2E | PASS | 用户、角色、店铺授撤权、重置、首次改密、拒绝和重复提交 |
| P2-7 深浅色、响应式、错误状态 | PASS | 1440/1280/1024/768/375 五档、两种主题、根节点无横向溢出；最终视觉验收待人工 |

## 自动化证据

本地使用任务专用 PostgreSQL 16 / Redis 测试容器，只通过 `TEST_DATABASE_URL`、`TEST_REDIS_URL` 接入；镜像创建的匿名卷不与开发或生产共享。完成回归后已显式停止、删除这两个容器及其匿名卷。未使用开发库或生产库；测试 URL 中的口令为合成 fixture，不应复制为部署配置。

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| `gofmt -l .` | PASS | 0 文件；`.gitattributes` 固定 Go LF，修复 Windows CRLF 导致的格式误报；不放宽格式检查 |
| `go vet ./...`、`go build ./...` | PASS | 最终后端行为修改后执行 |
| `go test -json -count=1 ./...` | PASS | 94 个测试 package、1187 个顶层测试，含子测试 1548 条 PASS；独立 Redis 已启用，普通全量执行无 PostgreSQL URL，19 项 PG 用例另行在隔离库执行；另 53 个 package 没有测试文件 |
| `go test -race -json -count=1 ./...` | PASS | 同上 1187 个顶层测试 / 1548 条 PASS，0 FAIL；最终源代码复跑 72.636 秒；19 项 PG 用例在此命令中因未注入 URL 跳过，单独使用隔离 PostgreSQL 执行，见下行 |
| PostgreSQL `go test -race -json -tags=inventorypostgres -count=1 ./internal/testing/integration` 及新增迁移专项 | PASS | 主集合 25 个顶层用例、35 条 PASS，474.647 秒、0 skip；再执行新增批次归属迁移及审计并发 2/2，重叠审计用例不重复累计，共 26 个不同顶层用例、36 条 PASS；涵盖重复迁移、管理员/轮换并发、审计与库存/订单/采购退货。早期 180 秒中断仍记为失败，未改 CI timeout |
| `inventorypostgres` 库存锁升级回归 | PASS | 7 个顶层标签用例已计入上行；最初订单并发运行真实触发 SQLSTATE 40P01，新增双事务共享仓库读锁用例在修复前稳定触发 55P03，修复后两个 Race 专项 2/2；完整标签主集合随后通过，没有将原失败归因于其他 suite 或删除断言 |
| Redis 与 OAuth state/callback | PASS | 全量 Race 注入独立 Redis；TikTok metadata/audit 故障回滚及 state 单次消费实际执行 |
| 权限与越权安全专项 | PASS | middleware 16、adminuser 26、adminperm 15、shopscope 21、IDOR 55、securitymod 83 条 PASS（含子测试，已计入全量 Race，不重复相加）；覆盖审计失败回滚、跨租户、禁用/未知角色、全局轮换权限、会话撤权及并发 |
| AI / 客服 / 店铺范围与刷新专项 | PASS | aioperationbatch 6、aiproductimage 13、customersync 10、imagetask 89、operationdashboard 5、shop 78、auth 28、operationlog 11 条 PASS，均包含在全量 Race；批次迁移的跨子类型冲突、未知/缺失归属和已有归属保留在真实 PostgreSQL 重复升级中验证。授权批次详情/任务分页的 JOIN 字段歧义先复现失败，再修复并验证正常/跨租户读取；最终 PG 迁移及读取专项 72.945 秒通过 |
| `pnpm test:frontend` | PASS | 47 个文件、178 个测试；jsdom/Ant Design 的既有 warning 保留，不作为失败隐藏 |
| TypeScript baseline ratchet | PASS | 当前 19 条历史诊断 / 13 签名，原基线 20 / 14；无新增签名，不能称为原始 tsc 零错误 |
| `pnpm build:admin` | PASS | 最终菜单映射修改后重新构建成功；使用现有 pnpm/Umi 生产构建 |
| 契约 / 架构 / workflow 单元 | PASS | 契约 24、架构 11、workflow 43；触发场景 14/14 |
| 权限覆盖矩阵与实现比对 | PASS | 全部 79 键、四种内置角色、6 个不可委派权限；与 Go 定义比对为 0 缺失、0 差异 |
| `pnpm architecture:check` | PASS | 15 项既有违规，0 新增、0 增加；未扩大基线 |
| 敏感差异检查 | PASS | 最终代码差异中未发现高置信凭据；不等于完整独立漏洞扫描 |
| 前端 lint / 质量检查 | PASS | 使用仓库现有 UI-copy、命名、敏感差异及 TypeScript ratchet；仓库没有独立 ESLint 脚本，不伪造命令结果 |
| Collector 单元与构建 | PASS | 6 个文件、21 个测试；`pnpm build:collector` 成功 |
| Admin 导航双构建回归 | PASS | `auth-navigation.spec.ts` 开发与生产各 28/28；生产内部库存路由为 404、零敏感请求，开发保留授权页面；五档视口、深浅色和全局密钥权限拒绝均覆盖 |
| Admin RBAC 与导航 E2E | PASS | 用户与角色 26 项、导航 28 项，共 54/54；全部包含在下行生产 P0 集合中，不重复累计 |
| Admin CI P0 集合 | PASS | 最终源代码 `a2a05800` 的生产构建及 preview、1 worker、`--retries=0`；`@smoke\|@product-draft\|@publish-safety\|@contract` 共 338/338，0 FAIL、0 skip，8.6 分钟；其中 smoke 288 项，使用既有 Write Guard、Console Guard，未扩大白名单 |
| GitHub Actions | BLOCKED | 当前修复尚未 push，无最终提交对应的 CI 结果；不能用其他 SHA 的历史运行代替 |
| 最终差异独立安全扫描 | BLOCKED | 原扫描 41d95334-0fc4-49f1-9e4c-ac1161e718ce 未封存；原代理模型不可用且后续修复已改变快照。继续读取确认仍为 discovery，0/112 review receipts；本次 preflight 为 ready，仅委派能力为 warning，但旧阶段不能回写 preflight。该状态不能作为最终代码的完整审查或零发现结论 |
| 生产备份恢复 / 旧二进制回滚 | BLOCKED | 仅验证隔离迁移与回滚边界，未访问生产或执行真实恢复 |

生产 P0 的仓库复现命令（PowerShell，仓库根目录）：

```powershell
$env:CI = 'true'
pnpm --filter @trademind/admin exec playwright test --config ../playwright.config.ts --grep '@smoke|@product-draft|@publish-safety|@contract' --retries=0
```

本地最后一轮使用已通过的当前生产构建，在独立 `127.0.0.1:18031` preview 上运行同一组标签。临时配置复用仓库的项目、浏览器、测试和 timeout，仅覆盖服务端口、单 worker、零重试、控制台报告器及系统临时目录输出；没有修改写请求/Console Guard 或白名单。完成后删除临时配置和测试输出，保留用户原有开发服务。

`quality:affected` 首次组合执行因 smoke 失败退出 1，未将该次执行改记为 PASS；上表记录后续各项实际结果。338 项覆盖现有 CI P0 四组标签的并集，不代表已运行全部夜间 E2E 或真实后端 E2E。最终 SHA 的 GitHub Actions 尚无结果，仍标记 BLOCKED。

## 剩余风险

1. **High / BLOCKED**：远程 CI 和完整独立安全扫描未闭合，当前尚不满足“所有自动化门禁全绿、只剩人工业务验收”的最终封板定义。本地提交可供代码审查及人工预验收。
2. **Medium**：原始 TypeScript 仍有上述历史诊断，架构仍有 15 项既有违规；本轮未放宽或扩大基线，也不把既有问题说成已修复。
3. **High**：数据库完全不可用时，敏感写入拒绝，无法保证失败请求同时已有数据库审计证据。恢复后需依据 HTTP/请求追踪与可观测性核对；本实现不声称具备独立于数据库的持久 Outbox。
4. **Medium**：OAuth Provider 的换码/刷新发生在本地事务之前，平台侧结果不能随本地回滚撤销。本地失败后必须重新发起授权，不能复用已消费的 state；真实 Provider、真实凭据和外部写入未执行。
5. **High / 待审查**：`LoadPrincipal` 保留历史内部调用缺少上下文时的兼容行为；生产敏感路由必须始终经 JWT/租户中间件。当前未证明存在可达绕过，但最终全量扫描应覆盖所有挂载入口。
6. **High / 运维边界**：启动配置中的 bootstrap 账号是运维恢复入口，重启可能把该账号恢复为管理员/启用。若希望永久撤销，应先由维护者更改恢复身份配置，不能仅在页面禁用后认定永久撤权。
7. **High / BLOCKED**：迁移为增量兼容，但安全版本与租户隔离不能随旧代码回退。已验证隔离 fixture 的重复迁移，生产规模的迁移耗时、锁占用及备份恢复未实测，禁止宣称生产回滚已验证。
8. **Medium**：本地完整 smoke 首次以默认 6 workers 运行时 285 PASS / 2 FAIL，分别为广告调整成功提示超时和调拨弹窗视口断言；原样单 worker 复跑两个文件为 13/13，没有删除断言或扩大基线。生产预览的后续首次完整 smoke 为 286 PASS / 2 FAIL，原因是新增导航用例错误期待生产存在开发内部库存路由，随后 404 文本定位器又错误合并两个独立元素；已按实际路由边界修正并分别验证生产与开发各 28/28。最终 CI 单 worker、构建后预览模式的集合结果见自动化证据；本地高并发稳定性不能据此标为已根治。
9. **Advisory / 已清理**：本次临时 Playwright 配置、临时计划、HTML 报告和 last-run 文件已移除，不在工作树或提交中。构建目录仍按仓库规则忽略；保留用户的开发服务及其他本地产物。

## 最终人工验收用例

仅在经批准的验收环境执行，使用虚构账号和店铺。用户于 2026-10-24 确认运维导航已有内容；以下完整流程仍待人工逐项签收，该反馈不替代其他账号、权限或业务验收。

| 编号 | 操作 | 预期 |
| --- | --- | --- |
| A01 | 用配置的 bootstrap 账号登录、刷新并打开 `/ops` | profile 完整后展示授权导航，运维默认跳到可访问页面；普通环境不重写既有密码 |
| A02 | 管理员创建运营/只读/审核账号，重复提交同一账号 | 每次有效确认只发起一次写请求；重复账号明确拒绝；密码不出现在审计中 |
| A03 | 新账号使用临时密码登录，完成首次改密，再用临时密码登录 | 改密前不能签发正常会话；改密后新密码可用，临时密码失效 |
| A04 | 修改显示名、停用再启用账号；保留旧浏览器与 Refresh Token | 旧凭据均失效；新登录展示最新身份；不能禁用自己 |
| A05 | 管理员重置密码，用旧 Access/Refresh 和历史设备继续操作 | 旧凭据拒绝；仅一次显示临时密码；下次先强制改密 |
| A06 | 新建自定义角色、选择权限树、预览并与另一角色比较 | 差异与 API 权限一致；不可选择系统专用权限；成员数准确 |
| A07 | 编辑角色权限、停用并重新启用角色 | 成员旧令牌全部失效；停用期间不可分配/使用；重新启用不复活旧会话 |
| A08 | 给同一用户依次授单店、多店和清空；尝试其他租户店铺 ID | 列表和深链同步变化；跨租户整笔拒绝，无部分授权 |
| A09 | 六类身份分别进入用户管理、运维和受限深链，并直接调用写 API | 菜单/按钮与权限一致；后端独立返回 403/404；无敏感读取或外部写入 |
| A10 | 两名管理员同时禁用或降权对方 | 至少保留一名有效管理员；失败方收到明确消息；审计与最终状态一致 |
| A11 | 验收库注入审计/元数据写失败后更改权限或完成 OAuth | 成功状态不提交，凭据/权限回滚，返回错误；恢复后可追踪失败请求 |
| A12 | 查询用户、角色、店铺和密码操作日志 | 有租户、操作者、请求追踪和前后变化；密码、Access/Refresh、AppSecret 不出现明文 |
| A13 | 五档视口切换深浅色，测试加载、空、错、无权与重试 | 布局无根节点横向溢出，操作禁用和中文提示一致 |
| A14 | 打开公开注册页并直接调用 register / send-email-code | 页面提示联系管理员，按钮禁用；服务端拒绝，不创建账号或发送验证码 |
| A15 | 同时保留多个设备会话，撤销单个/其他/全部会话 | 只影响当前用户指定范围，旧 Access/Refresh 均拒绝，审计条数与动作一致 |
| A16 | 在隔离副本升级两次、核对旧浏览器档案及 AI 批次归属，再按恢复方案演练 | 幂等迁移；文字/图片子任务归属冲突或缺失时保持隔离；不回退安全版本，不丢失审计 |
| A17 | 使用全局管理员、租户管理员、自定义角色直接查询和控制密钥轮换；模拟审计故障 | 只有 tenant 0 有效系统管理员可操作；其余拒绝并留拒绝审计；审计故障时状态回滚，日志不含密钥 |
| A18 | 两个租户分别校验审计完整性，窗口从一天中途开始；仅篡改一个租户的链或窗口前驱 | 正常窗口校验成功，篡改明确失败；不会读取另一租户的链或受其异常影响 |
| A19 | 使用受限账号访问 AI 批次、图片任务、客服同步和关联店铺摘要，替换为其他租户 ID | 读取、重试、应用及输入商品/图片/文件均拒绝跨租户；无 Provider、队列或远端写入 |
| A20 | 在隔离 Provider stub 中挂起刷新，撤销店铺，再放行旧刷新响应 | 旧结果不能提交凭据、授权状态或成功审计；后台和 HTTP 刷新均明确失败 |

## 修改文件与本地提交

本轮基线为 `67babce3b61ca02dcce538c51f750b1330ec5a11`，仅在 `dev` 创建本地提交，没有 push、PR、Tag、Release 或生产部署。下表覆盖全部 208 个不同文件及其各次修改提交；临时配置、计划和测试产物不提交。

| Commit | 内容 | 文件数 |
| --- | --- | --- |
| `1165cc40fa4748f843d140cbb10af526853a4de9` | fix(rbac): make security changes auditable and revoke stale credentials | 45 |
| `7e7c084853f702ace6ebb878f472356d37db535f` | fix(auth): persist shop authorization and audit records atomically | 24 |
| `4c31907b8c73a1209a17f0e2daee51fcf951f738` | fix(rbac): enforce tenant scope across sensitive admin APIs | 47 |
| `5a03a2b0479d2804b45e425c325ad66caad983b4` | fix(admin): complete RBAC controls and authenticated navigation | 15 |
| `1b9c2ba406df16c06c40fdf53617e41300d20dd7` | docs(rbac): record permission coverage and verification limits | 12 |
| `9bd223b86ab04cc2198728571843f554bdb9743e` | fix(security): fence OAuth refreshes and strengthen audit integrity | 33 |
| `01688cceb1680cfa7fb5be8661583b57e398671a` | fix(rbac): close tenant boundaries in AI and customer operations | 47 |
| `a2a05800d3a5c4b1293920c8ea38ab4b263fec7f` | fix(inventory): avoid warehouse lock upgrades during legacy migration | 2 |
| 本次文档提交 | docs(rbac): finalize local verification and outstanding gates | 7 |

“本次文档提交”指包含本表的最终文档提交，SHA 在交付说明中列出，也可运行 `git log -1 -- docs/SECURITY_PERMISSION_CHECKLIST.md` 查询；避免文档自引用 SHA。一个文件出现多个 Commit 时，均按修改先后列出。

| 文件 | 对应 Commit |
| --- | --- |
| [.gitattributes](../.gitattributes) | `1165cc40` |
| [.github/PULL_REQUEST_TEMPLATE.md](../.github/PULL_REQUEST_TEMPLATE.md) | `1b9c2ba4` |
| [.github/workflows/project-tests.yml](../.github/workflows/project-tests.yml) | `7e7c0848` |
| [CHANGELOG.md](../CHANGELOG.md) | `1b9c2ba4`、本次文档提交 |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | `1b9c2ba4` |
| [SECURITY.md](../SECURITY.md) | `1b9c2ba4` |
| [admin/e2e/mocks/workers.ts](../admin/e2e/mocks/workers.ts) | `5a03a2b0` |
| [admin/e2e/specs/auth-navigation.spec.ts](../admin/e2e/specs/auth-navigation.spec.ts) | `5a03a2b0` |
| [admin/e2e/specs/settings-users-rbac.spec.ts](../admin/e2e/specs/settings-users-rbac.spec.ts) | `5a03a2b0` |
| [admin/e2e/utils/routes.ts](../admin/e2e/utils/routes.ts) | `5a03a2b0` |
| [admin/src/app.tsx](../admin/src/app.tsx) | `5a03a2b0` |
| [admin/src/layouts/OpsGroupLayout.tsx](../admin/src/layouts/OpsGroupLayout.tsx) | `5a03a2b0` |
| [admin/src/pages/Settings/Security/index.tsx](../admin/src/pages/Settings/Security/index.tsx) | `5a03a2b0` |
| [admin/src/pages/Settings/Users/index.tsx](../admin/src/pages/Settings/Users/index.tsx) | `5a03a2b0` |
| [admin/src/pages/User/Login/index.tsx](../admin/src/pages/User/Login/index.tsx) | `5a03a2b0` |
| [admin/src/services/__tests__/auth.test.ts](../admin/src/services/__tests__/auth.test.ts) | `5a03a2b0` |
| [admin/src/services/auth.ts](../admin/src/services/auth.ts) | `5a03a2b0` |
| [admin/src/utils/__tests__/menuAccess.test.ts](../admin/src/utils/__tests__/menuAccess.test.ts) | `5a03a2b0` |
| [admin/src/utils/menuAccess.ts](../admin/src/utils/menuAccess.ts) | `5a03a2b0`、`01688cce` |
| [admin/src/utils/permission.test.ts](../admin/src/utils/permission.test.ts) | `5a03a2b0` |
| [admin/src/utils/permission.ts](../admin/src/utils/permission.ts) | `5a03a2b0` |
| [backend/cmd/server/main.go](../backend/cmd/server/main.go) | `1165cc40` |
| [backend/internal/api/collector_alias.go](../backend/internal/api/collector_alias.go) | `4c31907b` |
| [backend/internal/api/router.go](../backend/internal/api/router.go) | `4c31907b` |
| [backend/internal/database/collect_browser_profile_migration_test.go](../backend/internal/database/collect_browser_profile_migration_test.go) | `4c31907b` |
| [backend/internal/database/migrate.go](../backend/internal/database/migrate.go) | `4c31907b`、`01688cce` |
| [backend/internal/database/migrate_ai_operation_batch_tenant.go](../backend/internal/database/migrate_ai_operation_batch_tenant.go) | `01688cce` |
| [backend/internal/database/migrate_ai_operation_batch_tenant_test.go](../backend/internal/database/migrate_ai_operation_batch_tenant_test.go) | `01688cce` |
| [backend/internal/middleware/jwt.go](../backend/internal/middleware/jwt.go) | `1165cc40` |
| [backend/internal/middleware/jwt_test.go](../backend/internal/middleware/jwt_test.go) | `1165cc40` |
| [backend/internal/modules/admin/bootstrap.go](../backend/internal/modules/admin/bootstrap.go) | `1165cc40` |
| [backend/internal/modules/admin/bootstrap_test.go](../backend/internal/modules/admin/bootstrap_test.go) | `1165cc40` |
| [backend/internal/modules/admin/model.go](../backend/internal/modules/admin/model.go) | `1165cc40` |
| [backend/internal/modules/admin/performance_bootstrap.go](../backend/internal/modules/admin/performance_bootstrap.go) | `1165cc40` |
| [backend/internal/modules/admin/performance_bootstrap_test.go](../backend/internal/modules/admin/performance_bootstrap_test.go) | `1165cc40` |
| [backend/internal/modules/admin/role.go](../backend/internal/modules/admin/role.go) | `1165cc40` |
| [backend/internal/modules/adminuser/handler.go](../backend/internal/modules/adminuser/handler.go) | `1165cc40` |
| [backend/internal/modules/adminuser/password_reset.go](../backend/internal/modules/adminuser/password_reset.go) | `1165cc40` |
| [backend/internal/modules/adminuser/roles.go](../backend/internal/modules/adminuser/roles.go) | `1165cc40` |
| [backend/internal/modules/adminuser/router.go](../backend/internal/modules/adminuser/router.go) | `1165cc40` |
| [backend/internal/modules/adminuser/security_test.go](../backend/internal/modules/adminuser/security_test.go) | `1165cc40` |
| [backend/internal/modules/adminuser/service.go](../backend/internal/modules/adminuser/service.go) | `1165cc40` |
| [backend/internal/modules/aioperationbatch/authorization_test.go](../backend/internal/modules/aioperationbatch/authorization_test.go) | `01688cce` |
| [backend/internal/modules/aioperationbatch/handler.go](../backend/internal/modules/aioperationbatch/handler.go) | `01688cce` |
| [backend/internal/modules/aioperationbatch/model.go](../backend/internal/modules/aioperationbatch/model.go) | `01688cce` |
| [backend/internal/modules/aioperationbatch/service.go](../backend/internal/modules/aioperationbatch/service.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/handler.go](../backend/internal/modules/aiproductimage/handler.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/reconcile_apply.go](../backend/internal/modules/aiproductimage/reconcile_apply.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/router.go](../backend/internal/modules/aiproductimage/router.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/scope.go](../backend/internal/modules/aiproductimage/scope.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/scope_regression_test.go](../backend/internal/modules/aiproductimage/scope_regression_test.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/service.go](../backend/internal/modules/aiproductimage/service.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/service_apply.go](../backend/internal/modules/aiproductimage/service_apply.go) | `01688cce` |
| [backend/internal/modules/aiproductimage/service_crud.go](../backend/internal/modules/aiproductimage/service_crud.go) | `01688cce` |
| [backend/internal/modules/aiprompt/authorization_test.go](../backend/internal/modules/aiprompt/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/aiprompt/handler.go](../backend/internal/modules/aiprompt/handler.go) | `4c31907b` |
| [backend/internal/modules/aiprompt/router.go](../backend/internal/modules/aiprompt/router.go) | `4c31907b` |
| [backend/internal/modules/aiprompt/service.go](../backend/internal/modules/aiprompt/service.go) | `4c31907b` |
| [backend/internal/modules/auth/auth_regressions_test.go](../backend/internal/modules/auth/auth_regressions_test.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/auth/change_required_password.go](../backend/internal/modules/auth/change_required_password.go) | `1165cc40` |
| [backend/internal/modules/auth/email_code.go](../backend/internal/modules/auth/email_code.go) | `1165cc40` |
| [backend/internal/modules/auth/handler.go](../backend/internal/modules/auth/handler.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/auth/register.go](../backend/internal/modules/auth/register.go) | `1165cc40` |
| [backend/internal/modules/auth/service.go](../backend/internal/modules/auth/service.go) | `1165cc40` |
| [backend/internal/modules/auth/session_service.go](../backend/internal/modules/auth/session_service.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/auth/session_service_test.go](../backend/internal/modules/auth/session_service_test.go) | `1165cc40` |
| [backend/internal/modules/auth/sessions_handler.go](../backend/internal/modules/auth/sessions_handler.go) | `1165cc40` |
| [backend/internal/modules/collect/collector_alias_test.go](../backend/internal/modules/collect/collector_alias_test.go) | `4c31907b` |
| [backend/internal/modules/collect/router.go](../backend/internal/modules/collect/router.go) | `4c31907b` |
| [backend/internal/modules/collect/service.go](../backend/internal/modules/collect/service.go) | `4c31907b` |
| [backend/internal/modules/collectbrowserprofile/authorization_test.go](../backend/internal/modules/collectbrowserprofile/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/collectbrowserprofile/handler.go](../backend/internal/modules/collectbrowserprofile/handler.go) | `4c31907b` |
| [backend/internal/modules/collectbrowserprofile/model.go](../backend/internal/modules/collectbrowserprofile/model.go) | `4c31907b` |
| [backend/internal/modules/collectbrowserprofile/router.go](../backend/internal/modules/collectbrowserprofile/router.go) | `4c31907b` |
| [backend/internal/modules/collectbrowserprofile/service.go](../backend/internal/modules/collectbrowserprofile/service.go) | `4c31907b` |
| [backend/internal/modules/collectrule/authorization_test.go](../backend/internal/modules/collectrule/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/collectrule/handler.go](../backend/internal/modules/collectrule/handler.go) | `4c31907b` |
| [backend/internal/modules/collectrule/router.go](../backend/internal/modules/collectrule/router.go) | `4c31907b` |
| [backend/internal/modules/collectrule/service.go](../backend/internal/modules/collectrule/service.go) | `4c31907b` |
| [backend/internal/modules/collectruleai/authorization_test.go](../backend/internal/modules/collectruleai/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/collectruleai/router.go](../backend/internal/modules/collectruleai/router.go) | `4c31907b` |
| [backend/internal/modules/collectruleai/service.go](../backend/internal/modules/collectruleai/service.go) | `4c31907b` |
| [backend/internal/modules/collectruleai/service_test.go](../backend/internal/modules/collectruleai/service_test.go) | `4c31907b` |
| [backend/internal/modules/customersync/auto_reply_scheduler.go](../backend/internal/modules/customersync/auto_reply_scheduler.go) | `01688cce` |
| [backend/internal/modules/customersync/handler.go](../backend/internal/modules/customersync/handler.go) | `01688cce` |
| [backend/internal/modules/customersync/router.go](../backend/internal/modules/customersync/router.go) | `01688cce` |
| [backend/internal/modules/customersync/scope_regression_test.go](../backend/internal/modules/customersync/scope_regression_test.go) | `01688cce` |
| [backend/internal/modules/customersync/service.go](../backend/internal/modules/customersync/service.go) | `01688cce` |
| [backend/internal/modules/douyinpreflight/authorization.go](../backend/internal/modules/douyinpreflight/authorization.go) | `4c31907b` |
| [backend/internal/modules/douyinpreflight/authorization_test.go](../backend/internal/modules/douyinpreflight/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/douyinpreflight/handler.go](../backend/internal/modules/douyinpreflight/handler.go) | `4c31907b` |
| [backend/internal/modules/douyinpreflight/router.go](../backend/internal/modules/douyinpreflight/router.go) | `4c31907b` |
| [backend/internal/modules/douyinpreflight/service.go](../backend/internal/modules/douyinpreflight/service.go) | `4c31907b` |
| [backend/internal/modules/douyinruntime/authorization.go](../backend/internal/modules/douyinruntime/authorization.go) | `4c31907b` |
| [backend/internal/modules/douyinruntime/authorization_test.go](../backend/internal/modules/douyinruntime/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/douyinruntime/router.go](../backend/internal/modules/douyinruntime/router.go) | `4c31907b` |
| [backend/internal/modules/douyinruntime/service.go](../backend/internal/modules/douyinruntime/service.go) | `4c31907b` |
| [backend/internal/modules/files/authorization_test.go](../backend/internal/modules/files/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/files/handler.go](../backend/internal/modules/files/handler.go) | `4c31907b` |
| [backend/internal/modules/imagetask/apply.go](../backend/internal/modules/imagetask/apply.go) | `01688cce` |
| [backend/internal/modules/imagetask/execute_scoring.go](../backend/internal/modules/imagetask/execute_scoring.go) | `01688cce` |
| [backend/internal/modules/imagetask/generate_scene.go](../backend/internal/modules/imagetask/generate_scene.go) | `01688cce` |
| [backend/internal/modules/imagetask/handler.go](../backend/internal/modules/imagetask/handler.go) | `01688cce` |
| [backend/internal/modules/imagetask/handler_apply.go](../backend/internal/modules/imagetask/handler_apply.go) | `01688cce` |
| [backend/internal/modules/imagetask/handler_item_actions.go](../backend/internal/modules/imagetask/handler_item_actions.go) | `01688cce` |
| [backend/internal/modules/imagetask/handler_manual_edit.go](../backend/internal/modules/imagetask/handler_manual_edit.go) | `01688cce` |
| [backend/internal/modules/imagetask/items.go](../backend/internal/modules/imagetask/items.go) | `01688cce` |
| [backend/internal/modules/imagetask/lease.go](../backend/internal/modules/imagetask/lease.go) | `01688cce` |
| [backend/internal/modules/imagetask/list.go](../backend/internal/modules/imagetask/list.go) | `01688cce` |
| [backend/internal/modules/imagetask/monitor.go](../backend/internal/modules/imagetask/monitor.go) | `01688cce` |
| [backend/internal/modules/imagetask/persist.go](../backend/internal/modules/imagetask/persist.go) | `01688cce` |
| [backend/internal/modules/imagetask/providers_handler.go](../backend/internal/modules/imagetask/providers_handler.go) | `01688cce` |
| [backend/internal/modules/imagetask/replace_background.go](../backend/internal/modules/imagetask/replace_background.go) | `01688cce` |
| [backend/internal/modules/imagetask/service.go](../backend/internal/modules/imagetask/service.go) | `01688cce` |
| [backend/internal/modules/imagetask/service_item_actions.go](../backend/internal/modules/imagetask/service_item_actions.go) | `01688cce` |
| [backend/internal/modules/imagetask/source_resolver.go](../backend/internal/modules/imagetask/source_resolver.go) | `01688cce` |
| [backend/internal/modules/imagetask/tenant_scope.go](../backend/internal/modules/imagetask/tenant_scope.go) | `01688cce` |
| [backend/internal/modules/imagetask/tenant_scope_test.go](../backend/internal/modules/imagetask/tenant_scope_test.go) | `01688cce` |
| [backend/internal/modules/imagetask/test_image_handler.go](../backend/internal/modules/imagetask/test_image_handler.go) | `01688cce` |
| [backend/internal/modules/imagetask/translate_result_cleanup.go](../backend/internal/modules/imagetask/translate_result_cleanup.go) | `01688cce` |
| [backend/internal/modules/imagetask/worker.go](../backend/internal/modules/imagetask/worker.go) | `01688cce` |
| [backend/internal/modules/inventory/warehouse_ledger.go](../backend/internal/modules/inventory/warehouse_ledger.go) | `a2a05800` |
| [backend/internal/modules/inventory/warehouse_ledger_test.go](../backend/internal/modules/inventory/warehouse_ledger_test.go) | `1165cc40` |
| [backend/internal/modules/operationdashboard/schema_helpers.go](../backend/internal/modules/operationdashboard/schema_helpers.go) | `01688cce` |
| [backend/internal/modules/operationdashboard/service.go](../backend/internal/modules/operationdashboard/service.go) | `01688cce` |
| [backend/internal/modules/operationdashboard/tenant_scope_test.go](../backend/internal/modules/operationdashboard/tenant_scope_test.go) | `01688cce` |
| [backend/internal/modules/operationlog/bootstrap_audit.go](../backend/internal/modules/operationlog/bootstrap_audit.go) | `1165cc40` |
| [backend/internal/modules/operationlog/hash_chain.go](../backend/internal/modules/operationlog/hash_chain.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/operationlog/http_audit.go](../backend/internal/modules/operationlog/http_audit.go) | `1165cc40` |
| [backend/internal/modules/operationlog/service.go](../backend/internal/modules/operationlog/service.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/operationlog/service_test.go](../backend/internal/modules/operationlog/service_test.go) | `1165cc40`、`9bd223b8` |
| [backend/internal/modules/order/handler_permission_test.go](../backend/internal/modules/order/handler_permission_test.go) | `1165cc40` |
| [backend/internal/modules/order/shop_scope_regression_test.go](../backend/internal/modules/order/shop_scope_regression_test.go) | `9bd223b8` |
| [backend/internal/modules/order/tracking_handler_test.go](../backend/internal/modules/order/tracking_handler_test.go) | `1165cc40` |
| [backend/internal/modules/procurement/purchase_return_http_test.go](../backend/internal/modules/procurement/purchase_return_http_test.go) | `1165cc40` |
| [backend/internal/modules/profitability/handler_test.go](../backend/internal/modules/profitability/handler_test.go) | `1165cc40` |
| [backend/internal/modules/securitymod/authorization_test.go](../backend/internal/modules/securitymod/authorization_test.go) | `4c31907b`、`9bd223b8` |
| [backend/internal/modules/securitymod/handler.go](../backend/internal/modules/securitymod/handler.go) | `4c31907b`、`9bd223b8` |
| [backend/internal/modules/securitymod/rotation.go](../backend/internal/modules/securitymod/rotation.go) | `4c31907b`、`9bd223b8` |
| [backend/internal/modules/securitymod/rotation_authorization.go](../backend/internal/modules/securitymod/rotation_authorization.go) | `4c31907b` |
| [backend/internal/modules/securitymod/service.go](../backend/internal/modules/securitymod/service.go) | `4c31907b` |
| [backend/internal/modules/settings/authorization_test.go](../backend/internal/modules/settings/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/settings/handler.go](../backend/internal/modules/settings/handler.go) | `4c31907b` |
| [backend/internal/modules/settings/service.go](../backend/internal/modules/settings/service.go) | `4c31907b` |
| [backend/internal/modules/shop/amazon_bridge.go](../backend/internal/modules/shop/amazon_bridge.go) | `9bd223b8` |
| [backend/internal/modules/shop/amazon_oauth.go](../backend/internal/modules/shop/amazon_oauth.go) | `7e7c0848` |
| [backend/internal/modules/shop/authorization.go](../backend/internal/modules/shop/authorization.go) | `7e7c0848` |
| [backend/internal/modules/shop/authorization_test.go](../backend/internal/modules/shop/authorization_test.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/douyin_bridge.go](../backend/internal/modules/shop/douyin_bridge.go) | `9bd223b8` |
| [backend/internal/modules/shop/douyin_category_test.go](../backend/internal/modules/shop/douyin_category_test.go) | `7e7c0848` |
| [backend/internal/modules/shop/douyin_oauth.go](../backend/internal/modules/shop/douyin_oauth.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/douyin_oauth_state.go](../backend/internal/modules/shop/douyin_oauth_state.go) | `7e7c0848` |
| [backend/internal/modules/shop/douyin_oauth_test.go](../backend/internal/modules/shop/douyin_oauth_test.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/handler.go](../backend/internal/modules/shop/handler.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/lazada_bridge.go](../backend/internal/modules/shop/lazada_bridge.go) | `9bd223b8` |
| [backend/internal/modules/shop/lazada_oauth.go](../backend/internal/modules/shop/lazada_oauth.go) | `7e7c0848` |
| [backend/internal/modules/shop/oauth_bridge_security_test.go](../backend/internal/modules/shop/oauth_bridge_security_test.go) | `7e7c0848` |
| [backend/internal/modules/shop/oauth_persistence.go](../backend/internal/modules/shop/oauth_persistence.go) | `7e7c0848` |
| [backend/internal/modules/shop/oauth_persistence_test.go](../backend/internal/modules/shop/oauth_persistence_test.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/oauth_refresh_fencing.go](../backend/internal/modules/shop/oauth_refresh_fencing.go) | `9bd223b8` |
| [backend/internal/modules/shop/oauth_state.go](../backend/internal/modules/shop/oauth_state.go) | `7e7c0848` |
| [backend/internal/modules/shop/oauth_state_test.go](../backend/internal/modules/shop/oauth_state_test.go) | `7e7c0848` |
| [backend/internal/modules/shop/platform_app_settings.go](../backend/internal/modules/shop/platform_app_settings.go) | `7e7c0848` |
| [backend/internal/modules/shop/platform_publish_settings.go](../backend/internal/modules/shop/platform_publish_settings.go) | `7e7c0848` |
| [backend/internal/modules/shop/platform_settings_audit.go](../backend/internal/modules/shop/platform_settings_audit.go) | `7e7c0848` |
| [backend/internal/modules/shop/router.go](../backend/internal/modules/shop/router.go) | `7e7c0848` |
| [backend/internal/modules/shop/scope_regression_test.go](../backend/internal/modules/shop/scope_regression_test.go) | `9bd223b8` |
| [backend/internal/modules/shop/service.go](../backend/internal/modules/shop/service.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/service_audit_test.go](../backend/internal/modules/shop/service_audit_test.go) | `7e7c0848` |
| [backend/internal/modules/shop/shopee_bridge.go](../backend/internal/modules/shop/shopee_bridge.go) | `9bd223b8` |
| [backend/internal/modules/shop/shopee_oauth.go](../backend/internal/modules/shop/shopee_oauth.go) | `7e7c0848` |
| [backend/internal/modules/shop/tiktok_bridge.go](../backend/internal/modules/shop/tiktok_bridge.go) | `7e7c0848`、`9bd223b8` |
| [backend/internal/modules/shop/tiktok_oauth.go](../backend/internal/modules/shop/tiktok_oauth.go) | `7e7c0848` |
| [backend/internal/modules/warehousefee/handler_test.go](../backend/internal/modules/warehousefee/handler_test.go) | `1165cc40` |
| [backend/internal/modules/worker/authorization_test.go](../backend/internal/modules/worker/authorization_test.go) | `4c31907b` |
| [backend/internal/modules/worker/handler.go](../backend/internal/modules/worker/handler.go) | `4c31907b` |
| [backend/internal/modules/worker/monitor.go](../backend/internal/modules/worker/monitor.go) | `4c31907b` |
| [backend/internal/pkg/adminperm/context.go](../backend/internal/pkg/adminperm/context.go) | `1165cc40` |
| [backend/internal/pkg/adminperm/context_security_test.go](../backend/internal/pkg/adminperm/context_security_test.go) | `1165cc40` |
| [backend/internal/pkg/adminperm/custom_role_runtime_test.go](../backend/internal/pkg/adminperm/custom_role_runtime_test.go) | `1165cc40` |
| [backend/internal/pkg/adminperm/custom_roles.go](../backend/internal/pkg/adminperm/custom_roles.go) | `1165cc40` |
| [backend/internal/pkg/adminperm/custom_roles_test.go](../backend/internal/pkg/adminperm/custom_roles_test.go) | `1165cc40` |
| [backend/internal/providers/platform/amazon/provider_refresh.go](../backend/internal/providers/platform/amazon/provider_refresh.go) | `9bd223b8` |
| [backend/internal/providers/platform/douyinshop/client.go](../backend/internal/providers/platform/douyinshop/client.go) | `9bd223b8` |
| [backend/internal/providers/platform/douyinshop/oauth_test.go](../backend/internal/providers/platform/douyinshop/oauth_test.go) | `9bd223b8` |
| [backend/internal/providers/platform/douyinshop/provider_refresh.go](../backend/internal/providers/platform/douyinshop/provider_refresh.go) | `9bd223b8` |
| [backend/internal/providers/platform/douyinshop/token_singleflight.go](../backend/internal/providers/platform/douyinshop/token_singleflight.go) | `9bd223b8` |
| [backend/internal/providers/platform/lazada/provider_refresh.go](../backend/internal/providers/platform/lazada/provider_refresh.go) | `9bd223b8` |
| [backend/internal/providers/platform/shopee/provider_refresh.go](../backend/internal/providers/platform/shopee/provider_refresh.go) | `9bd223b8` |
| [backend/internal/providers/platform/tiktok/provider_refresh.go](../backend/internal/providers/platform/tiktok/provider_refresh.go) | `9bd223b8` |
| [backend/internal/providers/platform/tiktok/provider_refresh_test.go](../backend/internal/providers/platform/tiktok/provider_refresh_test.go) | `9bd223b8` |
| [backend/internal/providers/platform/types.go](../backend/internal/providers/platform/types.go) | `9bd223b8` |
| [backend/internal/securitytests/shopscope/shop_scope_test.go](../backend/internal/securitytests/shopscope/shop_scope_test.go) | `1165cc40` |
| [backend/internal/securitytests/shopscope/shopscope_extended_test.go](../backend/internal/securitytests/shopscope/shopscope_extended_test.go) | `1165cc40` |
| [backend/internal/testing/integration/adminuser_security_postgres_integration_test.go](../backend/internal/testing/integration/adminuser_security_postgres_integration_test.go) | `1165cc40` |
| [backend/internal/testing/integration/database_migration_test.go](../backend/internal/testing/integration/database_migration_test.go) | `01688cce` |
| [backend/internal/testing/integration/inventory_postgres_integration_test.go](../backend/internal/testing/integration/inventory_postgres_integration_test.go) | `a2a05800` |
| [backend/internal/testing/integration/operationlog_postgres_integration_test.go](../backend/internal/testing/integration/operationlog_postgres_integration_test.go) | `1165cc40` |
| [backend/internal/testing/integration/securitymod_postgres_integration_test.go](../backend/internal/testing/integration/securitymod_postgres_integration_test.go) | `4c31907b` |
| [docs/PROGRESS.md](../docs/PROGRESS.md) | `1b9c2ba4`、本次文档提交 |
| [docs/RBAC_PERMISSION_DESIGN.md](../docs/RBAC_PERMISSION_DESIGN.md) | `1b9c2ba4`、本次文档提交 |
| [docs/README.md](../docs/README.md) | `1b9c2ba4` |
| [docs/SECURITY_PERMISSION_CHECKLIST.md](../docs/SECURITY_PERMISSION_CHECKLIST.md) | `1b9c2ba4`、本次文档提交 |
| [docs/api.md](../docs/api.md) | `1b9c2ba4`、本次文档提交 |
| [docs/architecture.md](../docs/architecture.md) | 本次文档提交 |
| [docs/branching.md](../docs/branching.md) | `1b9c2ba4` |
| [docs/env.md](../docs/env.md) | `1b9c2ba4` |
| [docs/module-map.md](../docs/module-map.md) | `1b9c2ba4`、本次文档提交 |
