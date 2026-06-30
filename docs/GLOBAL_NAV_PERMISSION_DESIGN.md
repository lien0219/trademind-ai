# 全局菜单权限设计（Phase F6）

## 目标

F5 已有页面级 `PermissionGuard`；F6 补 **菜单级隐藏**，直接访问仍由 Guard / 后端拦截。

## 规则

| 角色 | 菜单 |
| --- | --- |
| admin | 全部 |
| operator | 可操作模块（无 settings 子项、无用户管理） |
| readonly | 只读模块（无写操作入口菜单） |

## 实现

- `admin/src/utils/menuAccess.ts` — 路由 → 权限映射 + `filterMenuByPermission`
- `admin/src/app.tsx` — `menuDataRender` 过滤侧栏
- `admin/src/utils/permission.ts` — 权限矩阵（与后端 `adminperm` 对齐）
- `admin/config/routes.ts` — 路由树（名称即菜单文案）

## 注意

- 菜单过滤 **不能替代** 后端权限校验
- 无权限 settings 子菜单（含配置状态中心、用户与权限）对非 admin 隐藏
- `access.ts` 仍为空，鉴权靠 token + layout + Guard
