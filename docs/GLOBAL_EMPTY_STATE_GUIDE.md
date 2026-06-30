# 全局空状态引导（Phase F6）

## 组件

`admin/src/components/ui/EmptyState.tsx`

- `title` — 为什么为空
- `description` — 下一步说明
- `actionLabel` + `actionPath` — 推荐入口

## 适用页面

| 页面 | 空态要点 |
| --- | --- |
| Dashboard | 无数据时卡片仍展示 0 + emptyHint；最近动态区引导采集/配置 |
| 采集中心 | 输入链接或配置采集服务 |
| 商品草稿 | 采集或手动创建 |
| AI 运营工作台 | 先有待处理商品或批次 |
| 订单列表 | 配置店铺授权并同步；Demo 可跑种子脚本 |
| 订单异常 | 同步订单后自动产生 |
| 库存中心 | 刊登 SKU 绑定后才有库存 |
| 客服中心 | 授权店铺并同步消息 |
| 失败任务中心 | 无失败为正常；有失败显示重试入口 |
| 配置状态中心 | 逐项完成 settings |
| 用户与权限 | admin 创建用户并分配店铺 |

## 文案来源

优先 `admin/src/constants/copywriting.ts` 的 `PAGE_COPY` 与 `EMPTY_GUIDE`。
