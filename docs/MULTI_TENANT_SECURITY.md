# TradeMind 多租户安全控制

更新时间：2026-10-25。生产安全结论及未完成项见[多租户验收](MULTI_TENANT_ACCEPTANCE.md)。

## 授权边界

1. 认证中间件校验 JWT、用户状态、TokenVersion、持久化 Session 和 Session 的租户绑定；请求租户从已验证凭据导出，而不是信任用户可控的 `tenant_id` 请求字段或 header。
2. 租户请求的角色来自 `(tenant_id,user_id)` 的有效 membership。membership disabled/removed、租户 suspended/archived/closed 或会话已撤销时，不得继续访问业务 API。
3. 平台控制面单独验证 `platform_operators.enabled`。全局 `admin` 角色、tenant 0、客户端路由隐藏均不能替代服务端授权。
4. 每家店铺、商品、任务和其他外部副作用目标均须重新确认归属当前租户；敏感跨域资源按现有 API 约定拒绝或返回不可见的 404。
5. 自定义角色在当前租户下读取；权限仍由既有允许委派权限白名单约束。店铺授权在写入时逐项确认 `shop.tenant_id`，切换租户不能沿用旧角色或店铺权限。

## 会话撤权和生命周期

租户切换签发新 Session 并撤销旧 Session/Refresh Token。membership 的角色、状态、店铺授权发生变化时撤销该身份的相关会话凭据；禁用、移除、离开租户以及租户暂停也必须立即使旧凭据失效。最后一位 active tenant admin 的降权、移除、禁用或退出在事务内拒绝。平台管理员状态、租户状态、成员状态和 Session 状态分别维护。

邀请 Token 以摘要保存，设过期时间，接受时在事务中锁定并验证状态、租户可用性、认证邮箱和目标角色，再创建/激活 membership 和店铺权限；重复接受、取消后接受、过期接受或邮箱不匹配必须失败。邮件不会由此 API 发送。

## 后台任务与文件

异步任务必须在入队时冻结 tenant 归属，并在 Worker 执行前验证租户状态、任务与父资源/店铺关联，以及目标对象所属租户。任务重试、租约恢复、回调和补偿不得从当前全局身份或可变租户状态推导历史任务的归属。Webhook 在签名、幂等和持久化后，只能按已绑定的店铺/租户上下文执行业务处理。

文件记录、导出任务和下载路径需把租户归属纳入服务端所有权判断；客户端给出的文件 ID、URL、对象 key 或任务 ID 不能作为授权依据。当前代码在主要路径加入 tenant 字段与检查，但完整文件/对象存储、缓存和所有后台入口的独立安全审计尚未完成，见[剩余风险](#剩余风险)。

## 安全审计

平台创建/更新/状态变化、租户配置、membership、邀请和成员退出等关键成功写入与审计记录在事务中提交。日志可关联操作者、租户、资源、动作和结果；不得记录密码、邀请明文 Token、完整 Session/Refresh Token、平台密钥或平台凭据。邀请明文 Token 仅在成功创建响应中返回一次。

应用没有平台运营者管理 API。对 `platform_operators` 的数据库授权和撤销必须在受控变更记录与数据库审计中双人复核；不能把租户内 `operation_logs` 当作该数据库管理行为的完整审计。

## 回归覆盖

已有回归位于 `backend/internal/modules/auth/tenant_test.go`、`tenantmgmt/*_test.go`、`adminperm/*_test.go`、`middleware/jwt_test.go`、`tasktenant/*_test.go`、业务 worker 的租户测试、`backend/internal/testing/integration/database_migration_test.go`、Admin 租户服务与 E2E、`tests/contracts`。本次增加可切换租户列表测试，验证只显示 active membership 且 tenant active。

不得用前端隐藏代替服务端授权，不得以放宽现有 RBAC、增加测试 skip 或全局 tenant fallback 来修复失败。完整风险覆盖状态见[验收矩阵](MULTI_TENANT_ACCEPTANCE.md)。

## 剩余风险

- 没有本轮深度安全扫描的有效 scan ID、报告或覆盖证明；扫描结论是 **BLOCKED**，不能表述为零发现。
- 没有完成逐 endpoint、repository、Redis namespace、文件对象 key、Webhook、批量/导出、异步重试的独立全量威胁审计。
- 没有生产规模迁移锁时长、备份恢复、跨区域/多实例并发或真实对象存储权限演练。
- 没有最终远程 CI 结果和完整多租户人工验收。

因此本文描述的是已实现的控制和验证范围，不是安全认证或生产就绪声明。
