# Dashboard 运营总览设计（Phase F6）

## 定位

`/dashboard/product-operations`（菜单名 **运营总览**）是全项目运营入口，不是复杂 BI。

- 只读 DB 聚合，不调用外部平台 API
- 不自动同步订单 / 库存 / 发送客服
- 不加载 raw / Prompt / 平台 raw 大字段
- 单模块失败时降级，不导致整页 500

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/dashboard/product-operations` | 完整看板（KPI / 待办 / 漏斗 / 异常 / 最近动态） |
| GET | `/api/v1/dashboard/overview` | 模块化 overview + 10 张顶部卡片 |
| GET | `/api/v1/dashboard/todos` | 统一待办流（P0/P1/P2） |
| GET | `/api/v1/dashboard/health` | 子系统健康摘要 + 配置风险 |

## 顶部卡片（overview.cards）

1. 今日采集任务
2. 商品草稿
3. AI 待复核
4. 发布检查问题
5. 刊登任务异常
6. 订单异常
7. 库存异常
8. 客服待回复
9. 失败任务
10. 配置风险

每张卡片：`count` / `status` / `priority` / `link` / `emptyHint`。

## RBAC

- `admin`：全租户聚合
- `operator` / `readonly`：按 `user_store_permissions` 过滤订单、客服、库存、刊登、商品（通过 platform config / publication 关联）
- 查询参数 `shopId` 可与 scope 叠加

## 模块

`backend/internal/modules/operationdashboard`

- `service.go` — 聚合逻辑
- `overview.go` — overview / todos / health
- `scope.go` — 店铺 scope 辅助

## 前端

- 页面：`admin/src/pages/Dashboard/ProductOperations/index.tsx`
- 服务：`admin/src/services/dashboard.ts`
- 兜底：`admin/src/constants/dashboardDefaults.ts`
