# 权限安全检查清单（Phase F5）

- [ ] admin / operator / readonly 三类账号可登录且 profile 含 permissions
- [ ] operator 仅见授权店铺订单/客服/库存/失败任务
- [ ] readonly 写 API 返回 40304
- [ ] 无店铺权限深链返回 404
- [ ] 设置 PUT/测试接口仅 admin
- [ ] 用户管理 API 仅 admin；不可禁用自己
- [ ] 操作日志不含密钥明文
- [ ] 前端 PermissionGuard 无权限页为中文
- [ ] Demo 账号见 `docs/demo-dataset.permissions.json`
