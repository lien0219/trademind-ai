# RBAC 用户与权限设计

更新时间：2026-10-24。后端权限定义以 `backend/internal/pkg/adminperm/matrix.go` 为准；本页记录当前契约，验证状态见 [权限安全检查清单](SECURITY_PERMISSION_CHECKLIST.md)。

## 角色

| 角色 | 标识 | 说明 |
| --- | --- | --- |
| 管理员 | `admin` | 当前租户管理；平台全局配置还要求全局管理员身份 |
| 运营 | `operator` | 授权店铺内商品/订单/库存/客服/任务操作 |
| 只读 | `readonly` | 授权范围内只读，写操作后端拦截 |
| 审核 | `reviewer` | 只具有矩阵列出的审核、执行、审计和只读能力 |
| 自定义 | `r_` 加 24 位十六进制 | 只能使用当前租户启用角色及其明确授权，不继承管理员能力 |
| 未知 / 停用角色 | 其他或已停用的角色 | 无授权；跨租户角色不能分配或恢复权限 |

控制台账号由有 `user.manage` 的系统管理员创建。公开注册及其验证码接口保留路径，但返回 403，不再创建默认管理员。注册页显示联系管理员的提示并禁用提交。新建账号及管理员重置密码后，必须先修改临时密码，再签发正常登录凭据。

`ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PHONE` 指定的启动账号在后端启动时保持 `admin`、`active`，变化同时增加安全版本并记录事务审计。普通环境中修改 `.env` 密码不会覆盖既有数据库密码。应先加载完整 profile 再显示管理导航；运维根入口按已授权能力跳转，无权限显示拒绝页。

## 权限矩阵

实现位置：`backend/internal/pkg/adminperm/matrix.go`  
Profile 导出：`GET /api/v1/auth/profile` → `permissions[]`

以下是全部权限键与内置角色的对照。“可选”表示可通过自定义角色明确授予，仍需租户、资源、店铺范围和职责分离检查；不是默认拥有。

`security.key.rotate` 仅用于 tenant 0 的有效系统管理员，不能委派给自定义角色。非零租户的管理员 profile 不返回该键；服务端仍独立核对身份与持久化角色。历史自定义角色包含此保留键时按无效授权拒绝，需要管理员修改为可委派权限后重新登录。

| 权限键 | 管理员 | 运营 | 只读 | 审核 | 自定义 |
| --- | --- | --- | --- | --- | --- |
| `product.view` | 是 | 是 | 是 | 否 | 可选 |
| `product.write` | 是 | 是 | 否 | 否 | 可选 |
| `ai_text.apply` | 是 | 是 | 否 | 否 | 可选 |
| `ai_image.apply` | 是 | 是 | 否 | 否 | 可选 |
| `publish.create_draft` | 是 | 是 | 否 | 否 | 可选 |
| `order.view` | 是 | 是 | 是 | 否 | 可选 |
| `order.operate` | 是 | 是 | 否 | 否 | 可选 |
| `sku.bind` | 是 | 是 | 否 | 否 | 可选 |
| `inventory.view` | 是 | 是 | 是 | 是 | 可选 |
| `inventory.operate` | 是 | 是 | 否 | 否 | 可选 |
| `inventory.approve` | 是 | 否 | 否 | 是 | 可选 |
| `customer.view` | 是 | 是 | 是 | 否 | 可选 |
| `customer.operate` | 是 | 是 | 否 | 否 | 可选 |
| `task.retry` | 是 | 是 | 否 | 否 | 可选 |
| `settings.manage` | 是 | 否 | 否 | 否 | 不可委派 |
| `user.manage` | 是 | 否 | 否 | 否 | 不可委派 |
| `operationlog.view` | 是 | 是 | 是 | 是 | 可选 |
| `store.view` | 是 | 是 | 是 | 否 | 可选 |
| `store.operate` | 是 | 是 | 否 | 否 | 可选 |
| `warehouse.view` | 是 | 是 | 是 | 是 | 可选 |
| `warehouse.manage` | 是 | 是 | 否 | 否 | 可选 |
| `logistics.view` | 是 | 是 | 是 | 是 | 可选 |
| `logistics.manage` | 是 | 是 | 否 | 否 | 可选 |
| `supplier.view` | 是 | 是 | 是 | 是 | 可选 |
| `supplier.manage` | 是 | 是 | 否 | 否 | 可选 |
| `procurement.view` | 是 | 是 | 是 | 是 | 可选 |
| `procurement.manage` | 是 | 是 | 否 | 否 | 可选 |
| `procurement.approve` | 是 | 否 | 否 | 是 | 可选 |
| `procurement.receive` | 是 | 是 | 否 | 否 | 可选 |
| `procurement.return` | 是 | 是 | 否 | 否 | 可选 |
| `sales_return.view` | 是 | 是 | 是 | 是 | 可选 |
| `sales_return.manage` | 是 | 是 | 否 | 否 | 可选 |
| `sales_return.approve` | 是 | 否 | 否 | 是 | 可选 |
| `sales_return.receive` | 是 | 是 | 否 | 否 | 可选 |
| `sales_return.refund` | 是 | 是 | 否 | 否 | 可选 |
| `order_profit.view` | 是 | 是 | 是 | 否 | 可选 |
| `order_profit.export` | 是 | 是 | 否 | 否 | 可选 |
| `settlement.view` | 是 | 是 | 是 | 是 | 可选 |
| `settlement.import` | 是 | 是 | 否 | 否 | 可选 |
| `settlement.export` | 是 | 是 | 否 | 是 | 可选 |
| `warehouse_fee.view` | 是 | 是 | 是 | 是 | 可选 |
| `warehouse_fee.manage` | 是 | 是 | 否 | 否 | 可选 |
| `advertising_fee.view` | 是 | 是 | 是 | 是 | 可选 |
| `advertising_fee.import` | 是 | 是 | 否 | 否 | 可选 |
| `advertising_fee.manage` | 是 | 是 | 否 | 否 | 可选 |
| `freight_fee.view` | 是 | 是 | 是 | 是 | 可选 |
| `freight_fee.import` | 是 | 是 | 否 | 否 | 可选 |
| `freight_fee.manage` | 是 | 是 | 否 | 否 | 可选 |
| `security.session.manage` | 是 | 是 | 否 | 否 | 不可委派 |
| `security.key.rotate` | 是 (tenant 0) | 否 | 否 | 否 | 不可委派 |
| `audit.read` | 是 | 是 | 是 | 是 | 可选 |
| `audit.export` | 是 | 否 | 否 | 否 | 可选 |
| `pii.read_masked` | 是 | 是 | 是 | 否 | 可选 |
| `pii.read_full` | 是 | 否 | 否 | 否 | 不可委派 |
| `pii.export` | 是 | 否 | 否 | 否 | 可选 |
| `config.read` | 是 | 是 | 是 | 否 | 可选 |
| `config.manage` | 是 | 否 | 否 | 否 | 不可委派 |
| `export.read` | 是 | 否 | 否 | 否 | 可选 |
| `export.create` | 是 | 否 | 否 | 否 | 可选 |
| `observability.read` | 是 | 是 | 是 | 否 | 可选 |
| `observability.manage` | 是 | 否 | 否 | 否 | 可选 |
| `alerts.read` | 是 | 是 | 是 | 否 | 可选 |
| `alerts.ack` | 是 | 否 | 否 | 否 | 可选 |
| `alerts.silence` | 是 | 否 | 否 | 否 | 可选 |
| `slo.read` | 是 | 是 | 是 | 否 | 可选 |
| `slo.manage` | 是 | 否 | 否 | 否 | 可选 |
| `operationtask.edit` | 是 | 是 | 否 | 否 | 可选 |
| `operationtask.review` | 是 | 否 | 否 | 是 | 可选 |
| `operationtask.execute` | 是 | 否 | 否 | 是 | 可选 |
| `operationtask.retry` | 是 | 否 | 否 | 是 | 可选 |
| `operationtask.audit.read` | 是 | 是 | 是 | 是 | 可选 |
| `inventory_sync.read` | 是 | 是 | 是 | 是 | 可选 |
| `inventory_sync.run` | 是 | 是 | 否 | 否 | 可选 |
| `inventory_sync.rerun` | 是 | 是 | 否 | 否 | 可选 |
| `inventory_snapshot.read` | 是 | 是 | 是 | 是 | 可选 |
| `sku_binding.read` | 是 | 是 | 是 | 是 | 可选 |
| `sku_binding.manage` | 是 | 是 | 否 | 否 | 可选 |
| `sku_binding.resolve_manual` | 是 | 否 | 否 | 是 | 可选 |
| `inventory_sync.audit.read` | 是 | 否 | 否 | 是 | 可选 |

## 错误码

| code | 含义 |
| --- | --- |
| 40302 | 无模块权限 |
| 40303 | 无店铺权限 |
| 40304 | 只读写操作 |
| 40305 | 需系统配置权限 |
| 40306 | 需用户管理权限 |

## 后端校验

- 统一包：`backend/internal/pkg/adminperm`
- 写操作必须 handler/service 校验，不可仅依赖前端隐藏按钮
- 店铺详情/深链：无权限返回 **404**（不泄露资源存在）

| 入口 | 权限与隔离 | 审计 / 撤权 |
| --- | --- | --- |
| `/admin/users`、`/admin/users/:id` | `user.manage`、同租户；没有用户 DELETE 接口 | 新建/编辑与成功审计同事务；禁止自禁用、自降权；最后有效管理员保护 |
| `/admin/users/:id/store-permissions` | `user.manage`；目标账号和全部店铺都必须属于当前租户 | 全量替换/清空与前后差异、版本增加同事务 |
| `/admin/users/:id/reset-password` | `user.manage`、同租户 | 临时密码只返回一次，日志不包含密码；安全版本增加与审计同事务 |
| `/admin/roles`、`/admin/roles/:code`、`/admin/permissions` | `user.manage`；系统角色不可改写；自定义角色同租户且权限键有效 | 创建/编辑/停启与审计同事务；修改角色使全部成员安全版本增加 |
| `/auth/change-required-password` | 验证当前临时密码及强制改密状态 | 新密码哈希、改密状态、安全版本与审计同事务 |
| `/auth/sessions/:id`、`revoke-others`、`logout-all`、`logout` | JWT 用户自己的会话；不信任客户端 userId；logout 的 Refresh 与当前用户/会话匹配 | 会话/Refresh 撤销与成功审计同事务，审计不可用返回失败，查询故障不伪装成成功 |
| `/shops`、店铺凭据、OAuth | `store.view` / `store.operate`、同租户店铺范围；创建/删除要求系统管理员；摘要、存在性查询及分类刷新也校验店铺范围 | 关键写入与审计同事务；OAuth state 单次使用并绑定用户、租户和安全版本；五个平台刷新以捕获的 TokenVersion 校验提交，过期回调不能恢复已撤销凭据 |
| `/platform/settings/*`、`/platform/publish-settings/*` | `settings.manage` 及全局管理员范围 | 加密配置与成功审计同事务 |
| `/settings/*` | 系统设置管理及全局范围 | 设置持久化与审计同事务；敏感值脱敏 |
| `/security/keys/*` | tenant 0、有持久化身份且启用的系统管理员及 `security.key.rotate`；缺少身份或租户上下文拒绝 | 预检、启动、暂停、恢复、验证与成功审计同事务；数据库故障回滚，拒绝请求记录失败审计；PG 事务锁串行化全局控制 |
| `/security/audit/integrity/*` | `audit.read`、当前认证租户 | 只校验本租户审计链；上下文缺失拒绝，不能固定读取 tenant 0 |
| `/collect/browser-profiles` | `product.view` / `product.write`，profile 同租户 | 列表与逐项操作校验租户；旧数据仅按可信创建人回填归属 |
| `/collect/rules`、规则 AI、采集入口 | 商品读/写权限、当前租户；店铺操作再校验店铺范围 | 未授权请求在调用采集端或 Provider 前拒绝 |
| AI Prompt、文件 | 对应读写能力；资源租户和店铺范围 | 无权限不能靠直接 URL 绕过页面控制 |
| AI 批次、商品图片批次与图片任务 | `product.view`、`ai_text.apply` / `ai_image.apply`；批次、子任务、商品、图片、文件统一校验租户 | 重试按任务类型授权；跨租户输入在 Provider/队列调用前拒绝；Worker、租约、评分和清理延续任务租户 |
| 客服消息同步、运营总览与订单关联店铺 | `customer.view` / `customer.operate` 等模块权限，租户及店铺范围 | 同步任务详情/重试、后台自动回复扫描、汇总/最近记录不跨租户；关联店铺摘要不能泄露未授权店铺 |
| Douyin preflight / runtime | 对应配置/商品能力及受控租户上下文 | 只读检查和受控动作保持既有生产能力边界 |
| `/workers/monitor` | `task.retry`、同租户；非管理员限制用户/店铺范围 | 监控不暴露其他租户任务 |
| Operation Log 查询 / 导出 | `operationlog.view` 或相应审计能力、租户/店铺范围 | 不可变日志及分区哈希链；完整保存权限变化前后快照 |

成功证据采用业务数据库事务，不依赖仅内存重试。用户、角色、授权、密码、会话及凭据写入不能在审计失败时提交。失败/拒绝的 RBAC、店铺写请求和会话撤销通过 `AuditFailures` 在回滚后单独落库，仅记录状态、动作、资源及请求追踪信息，不复制请求 body。失败审计也不能持久化时明确返回 500；数据库完全不可用期间无法声称已有持久审计，见风险清单。

PostgreSQL 审计串行化使用事务 advisory lock，不再在等待数据库锁时占用本地 mutex；同一业务事务可以追加多条审计。不能把其他数据库单元 fixture 的成功当作 PostgreSQL 并发证据。

新增审计使用哈希版本 2，将操作者、会话、角色、店铺、权限和请求元数据绑定进证据，仍兼容已有版本 1 的校验。缺失哈希明确失败；滚动窗口通过实际前驱记录校验起点，不信任窗口首行自报的前驱哈希。后台 OAuth 刷新同样使用业务事务写审计。

每次 JWT 请求读取数据库安全版本；会话还校验会话状态及版本。降权、禁用、店铺撤权、角色权限变更和密码重置会使旧 Access/Refresh/历史会话失效。重新启用角色也增加版本，旧凭据不会因此恢复。未知角色、已停用或不存在的自定义角色不能获得默认管理员权限。

## 迁移与回滚

- 生产数据库为 PostgreSQL。SQLite 仅用于进程内单元 fixture；历史兼容驱动不代表切换项目数据库。
- AutoMigrate 增加 `admin_mutation_locks`，为租户内管理员变更及账号身份竞争提供稳定锁行；增加 `collect_browser_profiles.tenant_id`、`ai_operation_batches.tenant_id` 和索引。
- 浏览器档案只根据持久化 `created_by` 指向的管理员回填租户；无法确定归属的旧行保留 tenant zero，不暴露给其他租户。
- AI 批次仅在文字和图片全部子任务都能证明同一非零租户时回填；缺失商品、未知或冲突归属保持 tenant zero。只有完全无子任务的批次允许使用创建人租户，已有非零归属不被改写；两次升级由隔离 PostgreSQL 覆盖。
- 库存历史余额迁移读取默认仓时使用 SHARE 锁，与并发订单读取兼容，避免持有 SKU 锁后将仓库 SHARE 升级为 UPDATE 的死锁；仓库元数据更新和禁用仍受行锁保护。
- 本轮没有删除业务列、表或审计数据。重复迁移、旧 schema/索引兼容由隔离 PostgreSQL 测试验证；迁移失败必须停止启动。
- 没有实现自动 down migration，也没有验证生产备份恢复。回退到缺少租户校验或版本校验的旧二进制可能重新开放安全缺口，不能仅以新增列兼容判断回滚安全。发布前必须在隔离副本验证恢复，保留审计和安全版本，不降低版本或删除锁表来恢复旧令牌。

## 前端

- `admin/src/utils/permission.ts`
- `admin/src/hooks/usePermission.ts`
- `admin/src/components/PermissionGuard.tsx`
