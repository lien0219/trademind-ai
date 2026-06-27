# GitHub 仓库对外展示配置清单

这份清单用于统一 TradeMind 在 GitHub 上的对外展示方式。目标不是把仓库做成营销页，而是让第一次访问的人在 10 秒内理解：

1. 这是什么项目
2. 现在适合谁使用
3. 项目已经做到什么程度
4. 我下一步应该点哪里

适用范围：

- GitHub 仓库首页
- About 区域
- Topics
- Social Preview
- 文档入口页

## 推荐成品

### About

中文：

```text
开源 AI 跨境电商运营平台，聚焦商品采集、商品草稿、AI 内容优化、刊登与订单库存协同。
```

English:

```text
Open-source AI operations platform for cross-border commerce: product collection, drafts, AI content optimization, publishing, and order/inventory workflows.
```

### 更短版描述

中文：

```text
面向跨境电商的开源 AI 运营平台。
```

English:

```text
Open-source AI operations for cross-border commerce.
```

### Topics

建议使用以下 Topics：

```text
ai, ecommerce, cross-border-commerce, erp, self-hosted, product-operations, go, react, playwright, douyin-shop
```

### Social Preview

建议直接使用：

- [github-social-preview.png](./assets/img/github-social-preview.png)

说明：

- 当前素材由 [../scripts/generate_readme_hero.py](../scripts/generate_readme_hero.py) 生成。
- 若更新 README 头图或核心口径，应同步重新生成。

## GitHub 设置步骤

### 1. 设置 About

入口：

- 仓库首页右侧 `About` 区域
- 点击齿轮按钮进入编辑

建议填写：

- Description：使用上方中英文成品之一
- Website：如果还没有正式官网，优先留空，不建议放临时演示链接
- Topics：使用上方推荐 Topics

填写原则：

- 不写空泛词，如“best”, “next generation”, “powerful”
- 不写内部阶段术语，如 `Phase 9.2`, `A1.1`, `blocked_by_real_credentials`
- 不把过细平台细节塞进 Description

### 2. 设置 Social Preview

入口：

- 仓库 `Settings`
- 找到 `Social preview`
- 上传预览图

建议素材：

- [github-social-preview.png](./assets/img/github-social-preview.png)

建议规则：

- 画面应优先展示品牌名、项目一句话定位、1-3 个界面截图
- 不放过多小字
- 不放难以识别的代码片段
- 中英文优先使用英文版社交分享图，便于外部传播

### 3. 保持首页信息顺序稳定

建议顺序：

1. 项目名
2. 一句话定位
3. 少量关键徽章
4. 语言切换
5. 快速入口链接
6. 头图
7. 项目介绍
8. 核心能力
9. 快速开始
10. 文档导航

不建议在首屏前半部分放：

- 过长目录
- 空占位栏目
- 详细平台阶段记录
- 大段路线图
- 贡献榜 / 赞助榜占位表格

## 展示原则

### README 负责什么

- 让新访客快速理解项目定位
- 告诉用户当前最成熟的能力
- 给出最短启动路径
- 提供清晰的下一跳入口

### docs/ 负责什么

- 承载开发、部署、架构、Provider、协作等细节
- 解释规则、约束、同步要求和维护方式
- 不把这些细节全部堆回 README 首页

### 不建议出现在对外首页的内容

- “规划中 / 预留 / Coming soon” 的空表格
- 过细的内部里程碑编号
- 只对维护者有意义的技术缩写堆叠
- 长篇平台兼容矩阵
- 临时验收术语直接暴露给普通访客

## 素材维护

当前相关文件：

- [../README.md](../README.md)
- [../README.en.md](../README.en.md)
- [./README.md](README.md)
- [./assets/img/readme-hero-zh.png](./assets/img/readme-hero-zh.png)
- [./assets/img/readme-hero-en.png](./assets/img/readme-hero-en.png)
- [./assets/img/github-social-preview.png](./assets/img/github-social-preview.png)
- [../scripts/generate_readme_hero.py](../scripts/generate_readme_hero.py)

当以下内容变化时，应同步检查：

- 项目定位变化
- 首页一句话描述变化
- 头图展示模块变化
- 核心截图变化
- GitHub About / Topics 变化

## 维护建议

- 对外首页每次只强调 1 个主定位，不要同时讲 5 个故事
- 截图优先展示“已经做成的主线”，不要优先展示后台配置页
- 如果新增更成熟的页面截图，优先替换头图，再考虑扩充 README 正文
- 英文版不要逐字硬翻，优先保证自然、紧凑、可读
