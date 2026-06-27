# Demo 演示脚本（Phase R1）

> **Release 状态**：`MVP Demo Ready`  
> 预计时长：15–25 分钟（含 AI 复核与批量刊登）

## 前置条件

1. 本地或预发环境已启动：`docker compose up -d` + 后端 + 管理端
2. 已执行 `.\scripts\seed-demo-data.ps1`（可选，推荐）
3. AI Provider 已配置（文案试跑必需；图片白底图需 Image Provider Key）
4. 管理员账号可登录

## 标准演示路径

| 步骤 | 操作 | 入口 | 预期 |
| --- | --- | --- | --- |
| 1 | 打开 AI 商品运营工作台 | AI 工具 → 商品运营工作台 `/ai/operation-workbench` | 统计卡片与待办列表加载 |
| 2 | 查看待复核 AI 文案 | 筛选「AI 文案待复核」 | 列表含中文类型/优先级 |
| 3 | 进入文案复核并应用一条 | 点击待办 → 复核页 `/product/ai-text-batches/:id` | 对比原文/建议，应用成功 |
| 4 | 返回工作台刷新 | 点击「刷新待办」 | 对应待办减少 |
| 5 | 查看待复核 AI 图片 | 筛选「AI 图片待复核」 | 缩略图与占位正常 |
| 6 | 应用一张图片到图库 | 复核页应用 | 商品详情图片 Tab 可见新图 |
| 7 | 查看商品详情运营进度 | 商品 → 草稿 → 详情顶部进度条 | 步骤与阻断中文展示 |
| 8 | 进入发布检查 | 详情 → 发布检查 / 工作台跳转 | passed / warning / failed 三态 |
| 9 | 选择多平台多店铺刊登目标 | 刊登 Tab 或批量刊登向导 | TikTok / Shopee 等为「仅生成本地草稿」 |
| 10 | 创建本地刊登草稿 | 批量刊登向导第 5 步创建 | 批次 success 或 partial_success |
| 11 | 查看批量刊登批次 | `/product/publish-batches/:id` | 子任务状态中文，技术详情可折叠 |
| 12 | 查看失败任务中心 | 运维 → 失败任务中心 `/ops/task-center/failures` | 可深链到文案/图片/刊登批次 |

## 抖店说明（演示边界）

- 可展示抖店配置、OAuth 入口、刊登 Tab 与「创建抖店商品草稿」按钮
- **无真实 App Key / 店铺凭证** 时，create-draft 预期 `blocked_by_real_credentials`，**不**标记为 Production Ready
- **不**演示直接上架

## 快捷脚本

```powershell
.\scripts\demo-route-smoke.ps1          # 路由 smoke
.\scripts\ai-text-trial-run.ps1         # 文案小样本
.\scripts\ai-image-trial-run.ps1        # 图片小样本
```

## Phase R1.2 预发输出文件

真实预发部署后，将 smoke / Demo 数据写入：

```powershell
.\scripts\demo-route-smoke.ps1 -ApiBase https://<pre-api-domain> -OutFile docs/demo-route-smoke.preprod.json
.\scripts\seed-demo-data.ps1 -ApiBase https://<pre-api-domain> -OutFile docs/demo-dataset.preprod.json
```

当前 `docs/demo-route-smoke.preprod.json` / `docs/demo-dataset.preprod.json` 为 **本地 dev 等价复跑**（apiBase=`http://127.0.0.1:8080`），待真实预发 HTTPS 域名替换后重新生成。

## 变更记录

| 日期 | 说明 |
| --- | --- |
| 2026-06-27 | Phase R1.2：预发输出文件说明；真实 HTTPS 待运维接入 |
| 2026-06-27 | Phase R1.1：失败任务中心路由更正为 `/ops/task-center/failures` |
| 2026-06-27 | Phase R1 标准演示路径 |
