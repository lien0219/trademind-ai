# RBAC 权限设计（Phase F5）

## 角色

| 角色 | 标识 | 说明 |
| --- | --- | --- |
| 管理员 | `admin` | 全系统配置、用户管理、全部店铺数据 |
| 运营 | `operator` | 授权店铺内商品/订单/库存/客服/任务操作 |
| 只读 | `readonly` | 授权范围内只读，写操作后端拦截 |

## 权限矩阵

实现位置：`backend/internal/pkg/adminperm/matrix.go`  
Profile 导出：`GET /api/v1/auth/profile` → `permissions[]`

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

## 前端

- `admin/src/utils/permission.ts`
- `admin/src/hooks/usePermission.ts`
- `admin/src/components/PermissionGuard.tsx`
