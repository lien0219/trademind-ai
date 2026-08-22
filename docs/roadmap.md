# Roadmap

TradeMind 当前路线坚持先完成 AI 商品运营闭环，再扩展多平台跨境 ERP MVP，最后逐步增强完整 ERP 能力。

## 产品优先级

1. **AI 商品运营工具**
2. **多平台跨境 ERP MVP**
3. **完整 ERP 增强**

## v0.1.0 项目地基版

目标：项目能启动，后台能登录，系统设置能保存。

- Monorepo 项目结构
- Go Gin 后端
- React + Ant Design Pro 后台
- PostgreSQL + Redis
- 管理员登录
- 统一 API 返回
- settings 配置中心
- 敏感配置加密
- 本地存储与文件上传
- Docker Compose

## v0.2.0 AI 文本版

目标：可以配置 AI Provider，并完成标题与描述优化。

- AI Provider 接口
- OpenAI-compatible Provider
- Prompt 模板
- AI 设置页面
- Prompt 编辑页面
- AI 标题优化
- AI 描述生成
- AI 调用记录

## v0.3.0 商品草稿版

目标：商品数据可以被创建、编辑、保存。

- products
- product_skus
- product_images
- 商品草稿列表
- 商品详情编辑
- SKU 编辑
- 商品图片管理
- 商品归档

## v0.4.0 采集版

目标：可以采集商品链接，并保存为商品草稿。

- Node.js + Playwright 采集服务
- 1688 Provider
- AliExpress beta
- 自定义规则采集 beta
- 采集任务队列
- 采集任务状态
- 失败重试
- 采集结果生成商品草稿

## v0.5.0 图片能力版

目标：形成更完整的 AI 商品图处理能力。

- Image Provider 接口
- remove.bg
- OpenAI Image
- ComfyUI
- 图片任务表
- 图片处理任务页面
- 自动重试与任务监控

## v0.6.0 多平台 ERP MVP

目标：完成店铺授权、订单、刊登和库存同步的 MVP 闭环。

- Platform Provider 接口
- 店铺列表
- 平台配置页面
- 抖店 / Douyin Shop 优先授权
- 抖店类目属性
- 抖店图片上传
- 抖店商品草稿创建
- 订单同步
- SKU 匹配与候选推荐
- 商品刊登任务
- 库存同步任务
- TikTok Shop / Shopee / Lazada / Amazon SP-API 后续接入，不在当前阶段多平台并行铺开

## v0.7.0 AI 客服预览版

目标：AI 能根据客户消息和订单上下文生成建议回复。

- 客服消息同步
- 会话列表
- AI 建议回复
- FAQ / Prompt 模板
- 人工确认发送
- Tool Calling 接口预留

## v1.0.0 开源稳定版

目标：形成稳定、可部署、可扩展的开源版本。

- 完整部署文档
- Provider 扩展文档
- 更完善的测试与 CI
- 更清晰的 Demo 与截图
- 更稳定的升级路径

## 后续完整 ERP 增强

完整 ERP 按可验收闭环渐进建设，详细边界见 [ERP 扩展架构](ERP_ARCHITECTURE.md)。当前已完成仓库/供应商主数据、采购单与分批收货、Admin 采购工作台、人工调库和历史库存迁移/对账、订单库存生命周期、仓库调拨、库存盘点、采购退货及只读安全库存/补货建议 V1 的代码闭环；新实现仍需现有 CI 与人工业务验收后才能签收。

后续能力顺序：

- 持续验收订单库存、调拨、盘点与采购退货的并发和对账边界
- 收敛剩余旧库存写入，将 `product_skus.stock` 明确降级为仓库余额兼容投影
- 持续验收只读安全库存与补货建议的库存账阻断、供应商选择和导出边界
- 复杂售后 / 退款
- 复杂财务结算
- WMS / OMS
- 复杂 BI
- 自动化规则引擎
- 自动补货
- 自动直接上架
- 多租户 SaaS
