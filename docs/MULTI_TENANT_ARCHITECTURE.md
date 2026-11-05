# TradeMind 多租户架构

更新时间：2026-10-25。本文按当前 `dev` 分支实现描述，生产就绪状态见[验收状态](MULTI_TENANT_ACCEPTANCE.md)。

## 部署与数据模型

当前采用共享应用、共享 PostgreSQL schema 的逻辑租户隔离模式。没有为每个租户创建独立数据库，也没有依赖 PostgreSQL RLS 作为唯一隔离层。业务表通过 `tenant_id` 归属租户，并由认证上下文、RBAC、repository/service 查询条件和 Worker 执行前校验共同隔离。`tenant_id=0` 表示系统/未能可靠判定归属的数据，不代表任一客户租户。

核心模型位于 `backend/internal/modules/admin/tenant.go`、`tenant_invitation.go`：

| 实体 | 作用与约束 |
| --- | --- |
| `tenants` | 租户注册、名称/联系方式、设置 JSON、生命周期状态和乐观版本；`code` 全局唯一。Provision 幂等键和请求摘要不向 API 返回。 |
| `admin_users` | 全局登录身份。兼容保留既有 `tenant_id`；一个身份可通过多条 membership 加入多个租户。`last_tenant_id` 是登录恢复偏好，不是授权凭据。 |
| `tenant_memberships` | `(tenant_id,user_id)` 唯一；身份在每个租户有独立角色、状态。membership 是租户访问授权来源。 |
| `tenant_invitations` | 租户范围内的邀请；数据库只存 Token 摘要，另存邮箱、角色、店铺授权、到期时间、状态和操作者。 |
| `platform_operators` | 与租户 RBAC 分离的平台控制面授权；`admin` 角色或 tenant 0 本身不产生平台权限。当前没有应用内授权维护 API。 |
| `role_definitions`、`user_store_permissions` | 继续复用现有 RBAC 与店铺授权；自定义角色定义按租户归属，店铺授权在写入和读取时验证店铺归属当前租户。 |

租户状态为 `pending`、`active`、`suspended`、`archived`、`closed`。Provision 在一个数据库事务中创建/复用 owner 身份、建立管理员 membership、激活租户并写成功审计；相同幂等键与相同请求摘要返回既有租户，不同请求摘要冲突。`active` 可转为暂停、归档或关闭；暂停可恢复为 active；pending 可激活或归档；archived/closed 是终态。状态更新要求 `version`，成功后递增版本并审计。暂停、归档或关闭会撤销相关成员会话/Refresh Token，不会删除业务数据。

## 身份、成员与授权

JWT 和持久化 Session 带有服务端签发的 tenant 绑定。客户端提交的 `tenantId` 仅用于请求切换目标；服务端重新校验目标租户为 active、membership 为 active，再签发新租户 Session，并撤销当前 Session/旧凭据。登录会恢复最近一个有效租户；无有效 membership 的旧身份进入 tenant 0 恢复上下文，不会从旧 `admin_users.tenant_id` 越权回退。

同一用户在不同租户分别使用 membership 角色及该租户的自定义角色。既有系统角色、权限白名单和店铺授权继续生效。最后一个有效租户管理员不能退出、禁用、移除或降权；membership 变更使用租户级事务锁，并撤销对应身份的旧 Session/Refresh Token。邀请接受需已认证身份且邮箱匹配；邀请 Token 单次使用并有过期时间。创建邀请不会发邮件，明文 Token 只在创建响应返回一次。

平台运营者检查 `platform_operators.enabled`，与业务 tenant ID、用户名角色分开授权。它只开放 `/platform/tenants` 控制面，不隐式成为任何租户成员，也不因此能读取全量业务数据。运维人员添加或撤销平台运营者须遵循 [运维流程](MULTI_TENANT_OPERATIONS.md#平台运营者管理)。

## 关键入口

- 生命周期和控制面：`backend/internal/modules/tenantmgmt`。
- 身份、Session、租户切换：`backend/internal/modules/auth`、`backend/internal/middleware/jwt.go`。
- 权限上下文：`backend/internal/pkg/adminperm`；后台任务租户上下文与门禁：`backend/internal/pkg/tasktenant`。
- Schema 与兼容回填：`backend/internal/database/migrate.go`、`migrate_tenant_memberships.go`、`migrate_tenant_security.go`、`migrate_ai_operation_batch_tenant.go`。
- 管理台：`admin/src/pages/Platform/Tenants`、`admin/src/pages/Tenant`、`admin/src/components/layout/AppTopNav.tsx`、`admin/src/services/tenants.ts`。
- 公开 API 清单：[`docs/api.md`](api.md#多租户)。

## 业务隔离范围与边界

主要业务记录（商品、店铺、订单、库存、采购、售后、物流、结算/费用、客服、图片/采集任务、导出与文件记录等）使用既有租户字段或以租户归属的父资源为依据；所有新旧入口都必须在读写、批量操作、导入导出、聚合和异步执行时绑定同一租户及适用的店铺/角色权限。Worker 使用持久化任务 tenant 或经验证的 shop/product 父资源来构造上下文；缺少上下文或关联资源租户不一致时关闭执行。Webhook 由持久化事件绑定租户后再处理。

当前实现提供应用层隔离，不等于全量跨租户威胁建模完成。现有大型业务域仍须按模块地图和 CI 覆盖检查具体查询、缓存键、文件/对象路径、统计聚合和错误分支；本轮没有证明每个 endpoint、每种 Redis key 或每种存储后端均完成完整审计。未确认归属的旧记录保留在 tenant 0 并对非零租户不可见。

## 非目标

本模型不启用公开自助注册、不发送邀请邮件、不创建租户独立数据库、不自动迁移无法判定的历史数据，也不把租户暂停等同于物理删除。平台运营者的授权/撤销 API 和完整租户级运维看板尚未提供。
