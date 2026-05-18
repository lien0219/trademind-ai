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
- TikTok Shop 授权
- Shopee 授权
- Lazada 授权
- Amazon SP-API 授权
- 订单同步
- SKU 匹配与候选推荐
- 商品刊登任务
- 库存同步任务

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

以下能力后置，不作为当前 MVP 的第一目标：

- 多仓库存
- 采购入库
- 供应商管理
- 售后 / 退款
- 财务统计
- WMS / OMS
- 复杂 BI
- 自动化规则引擎
- 多租户 SaaS
