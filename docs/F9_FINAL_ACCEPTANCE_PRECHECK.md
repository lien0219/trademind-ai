# F9 最终总体验收准备清单

> **用途**：F9 启动前环境与人肉准备；**F8 只维护清单，不执行验收**。

## 基础设施

- [ ] 运行中 **backend**（`go build ./cmd/server/...` 或 Docker）
- [ ] 运行中 **admin**（`pnpm build:admin` + 静态服务或 dev）
- [ ] **PostgreSQL** 可连接（默认 5432）
- [ ] **Redis** 可连接
- [ ] **Collector** 服务（1688/PDD/淘宝采集走查时需要）

## Demo 与账号

- [ ] 执行 `scripts/seed-demo-data.ps1`
- [ ] 执行 `scripts/seed-demo-permissions.ps1`
- [ ] （可选）`POST /api/v1/dev/demo-seed/full-project-edge-cases`（dev/demo）
- [ ] **demo_admin** / **demo_operator** / **demo_readonly** 可登录

## 配置

- [ ] **Storage** 已配置（本地或云；抖店 E2E 需 **公网 URL**）
- [ ] **AI Provider** 已配置（文案/图片/客服 AI 建议走查）
- [ ] **OCR Provider**（图片文字翻译走查时需要）
- [ ] **Collector** 连接与浏览器 Profile（1688/PDD/淘宝）
- [ ] **配置状态中心** 无 blocking 项（或已知风险已记录）

## 抖店 / 平台

- [ ] 抖店 **App Key / Secret / Service ID** 是否准备（`manual_required`）
- [ ] 抖店 **OAuth 授权店铺** 是否准备
- [ ] Storage **公网访问** 是否验证（`POST /api/v1/storage/test-public-access`）
- [ ] 非抖店平台预期 **`local_draft_only`**（不阻塞 Demo）

## 预发 / 网络

- [ ] 预发 **服务器** 是否准备
- [ ] **HTTPS 域名** 是否准备
- [ ] Nginx / 反向代理配置草案
- [ ] **回滚方案** 文档与联系人

## 人工测试

- [ ] 测试 **人员** 与角色分工
- [ ] 测试 **浏览器**（Chrome 推荐）
- [ ] 测试 **分辨率** 1366×768 与 1024×768
- [ ] [`DEMO_CHECKLIST.md`](../DEMO_CHECKLIST.md) 16 步主链路 printed / shared
- [ ] [`docs/FULL_PROJECT_MVP_MAIN_FLOW.md`](FULL_PROJECT_MVP_MAIN_FLOW.md) 对照表

## 自动化（F9 首日复跑）

```bash
pnpm demo:auto-acceptance
```

期望：go test / build / 静态扫描通过；backend 在线时 API smoke 通过。

## 不在 F9 之前强制

- 生产灰度
- `v0.1.0-demo` tag
- Production Ready 标记

## 相关文档

- [`DEMO_AUTO_ACCEPTANCE_GUIDE.md`](DEMO_AUTO_ACCEPTANCE_GUIDE.md)
- [`DEMO_SEEDING_GUIDE.md`](DEMO_SEEDING_GUIDE.md)
- [`FUNCTION_FREEZE_P0_P1_AUDIT.md`](FUNCTION_FREEZE_P0_P1_AUDIT.md)
