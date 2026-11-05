---
name: admin-e2e-testing
description: TradeMind 生产维护阶段的 Admin Playwright CI 回归、写请求安全、响应式覆盖与人工验收规范
---

# TradeMind Admin 自动化测试与 E2E 测试规范

本 Skill 是 Admin 自动化测试、E2E 测试、Playwright MCP 动态验收、Playwright Test 持久化回归、CI 触发和后续页面测试补充规则的唯一完整来源。其他入口只引用本文件，不复制完整规范。

## 生产维护策略（优先级最高）

- Playwright Test 继续作为 `admin/e2e/**` 与 `.github/workflows/admin-e2e.yml` 中的持久 CI 回归；这些测试及其 Mock/fixture 必须保留。
- 功能、视觉、文案、响应式和业务流程最终由人工验收；完整 E2E 默认交由 GitHub Actions。
- Playwright MCP 只在用户明确要求本地浏览器验收或任务确需交互诊断时使用，不生成长期保留的证据。
- 本 Skill 后续“必须运行”的要求，在未明确要求本地执行时表示相应用例必须由 CI 覆盖。未本地运行不得声称 passed。
- 非 GET 请求拦截、五档视口、状态覆盖、根节点 overflow 和真实平台隔离要求继续强制执行。
- 不创建阶段/批次 gate 或一次性验收包装器；Playwright 报告、截图和临时结果完成诊断后清理，不提交 Git。

## 1. 自动适用范围

任何涉及 Admin 页面的新增、修改、UI 修复、组件、页面布局、响应式、表格、表单、Modal、Drawer、Popconfirm、Tabs、路由、URL 参数、深链、loading、empty、error、readonly、disabled、写操作、请求 payload、防重复提交、Console warning、可访问性、共享 UI 组件、`global.less`、`TmPageContainer` 或 `layoutTokens` 的任务，均自动适用本 Skill。

用户无需明确说“运行 E2E”“使用 Playwright”或“使用测试 Skill”。AI 必须自动识别并执行相关测试。纯 service、类型或工具函数修改只有在确认 DOM、className、页面状态、用户交互、路由、请求触发时机、loading/error/empty、响应式和写请求 payload 全部不受影响时，才允许跳过浏览器测试。

## 2. 测试分层

- 探索验收：Playwright MCP，用于开发过程动态确认真实 UI、Console、网络和交互。
- 持久回归：Playwright Test，用于仓库内可重复运行、CI 和定时回归。
- 静态检查：`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin` 和必要的类型/构建检查。

## 3. Playwright MCP 模式

开发时用 Playwright MCP 访问用户提供的 Admin 预览地址；常见本地地址是 `http://localhost:8000`，不要把端口写死为默认值。若用户已经启动 Admin dev server，复用现有服务，不启动、不停止、不杀进程。所有非 GET API 必须先用 `browser_route` 拦截，不得执行真实平台写操作。

## 4. Playwright Test 模式

持久 E2E 使用 Playwright Test，首期浏览器为 Chromium。默认 `baseURL` 为 `http://127.0.0.1:8001`，本地复用已有 server；CI 由 `webServer` 执行 `pnpm build:admin` 后启动 Admin preview，端口 8001。需要复用用户已启动的本机预览时，显式设置 `ADMIN_E2E_BASE_URL`（例如 `http://localhost:8000`）；该变量仅接受 `http` / `https` 回环 origin，设置后配置不会启动或管理 `webServer`。测试不得依赖后端服务，所有 `/api/v1/**` 业务接口和页面所需的旧 `/api/collector/**` 读取接口由 Mock 提供；共享写守卫覆盖 `/api/**`，未逐测试声明的写请求必须阻断。

## 5. 测试目录规范

Admin E2E 位于 `admin/e2e/`：

- `fixtures/`：测试 fixture。
- `mocks/`：API envelope 与业务 Mock。
- `pages/`：稳定 Page Object。
- `specs/`：按功能拆分测试。
- `utils/`：network guard、console guard、断言和路由工具。

不得把全部测试塞进一个文件，不得把业务 Mock 写进 Playwright config，不创建巨大万能 helper。

## 6. 测试数据规范

测试数据使用明确 `e2e` 或 `mock` 前缀，例如 `e2e-user`、`e2e-product-draft`、`e2e-shop-douyin`、`e2e-publication-old`、`e2e-publication-new`。不得引用生产 ID、真实店铺、真实用户、真实 token。每个测试独立建立 Mock，不依赖执行顺序。

## 7. API Mock 规范

Mock 必须基于真实前端 request helper 和 service 类型。统一 envelope 为：

```ts
{ code: number; message: string; data: T; traceId?: string }
```

`code !== 0` 为业务错误。`GET /api/v1/image/providers` 的 `data` 必须是 `ImageProviderCapability[]`，不得返回 `{ data: { list: [] } }`。

## 8. 写请求安全边界

所有 API 非 GET 请求默认禁止，除非当前测试显式允许。至少识别 `POST`、`PUT`、`PATCH`、`DELETE`。Write Guard 必须捕获 method、URL、path、query、payload、次数和顺序；未声明写请求必须阻断并导致测试失败。

允许的写接口必须逐测试声明，并返回 Mock success/failure。必须支持断言取消 0 请求、确认 1 请求、快速重复点击仍 1 请求、没有额外写请求。

## 9. 选择器规范

优先使用 `getByRole`、`getByLabel`、`getByPlaceholder`、明确 `getByText` 和稳定业务标识。禁止优先依赖 Ant Design 内部 class、深层 CSS selector、`nth-child`、坐标点击、随机 ID 或偶然 DOM 层级。只有无可靠语义选择器时才最小添加 `data-testid`，且最终报告列出生产代码修改。

## 10. Console 和运行时错误规范

Console Guard 必须捕获 `pageerror`、`console.error`、unhandled rejection、React fatal error、Ant Design fatal warning 和 HMR overlay。默认 pageerror、console.error、新增 React warning、新增 AntD warning 失败。白名单必须精确且说明原因，不允许 `/warning/`、忽略所有 React warning 或忽略所有 AntD warning。

当前候选 warning（如仍稳定存在才可精确白名单）：`useForm is not connected to any Form element`、`Each child in a list should have a unique "key" prop`。

## 11. 响应式规范

强制视口：1440×900、1280×800、1024×768、768×900、375×812。页面根节点无横向 overflow，Header 与 Content 左右边缘误差不超过 4px，Tabs 可用，主要操作区不超出视口，表格只在自身容器内部滚动。

## 12. 页面根节点 overflow

标准断言：

```js
document.documentElement.scrollWidth <= document.documentElement.clientWidth;
document.body.scrollWidth <= document.body.clientWidth;
```

断言失败必须输出实际值和预期值。

## 13. 路由、深链、刷新和 history

路由、URL 状态、Tab、section、深链和刷新恢复必须测试。商品详情普通 Tab 当前使用 `replaceState`，测试不得错误要求 `pushState`。非法 tab 必须安全 fallback。

## 14. Modal / Drawer / Popconfirm

必须验证标题、上下文、默认值、loading、confirmLoading、取消、关闭、移动端宽度、单次确认只发一次请求。取消不得发请求；危险操作保留确认。

## 15. loading / empty / error / readonly

新增或修改页面必须覆盖 normal、loading、empty、error、readonly、disabled、submitting。错误不能伪装为空数据，空数据不能伪装为错误，readonly 不得扩大或改变原业务语义。

## 16. 测试优先级 P0 / P1 / P2

P0：每个 Admin PR 必跑，覆盖 Admin smoke、核心路由、商品详情七个 Tab、精简响应式/overflow、发布请求安全、核心 API contract、Console fatal error。

P1：合并前或每日运行，覆盖完整五档响应式、Basic 保存、AI 任务、图片任务、SKU 编辑、库存调整、预警线、库存同步、readiness、抖店配置、映射、上传、绑定/解绑、readonly、Modal/Drawer。

P2：夜间或人工触发完整回归，覆盖全部 Admin 路由、长文本、全部状态、history、权限组合、Console warning 审计、可访问性基础扫描和性能基础检查。

## 17. 变更类型与测试选择

- 页面局部 TSX/LESS：P0 smoke、目标页面 spec、相关响应式。
- `TmPageContainer`、`layoutTokens`、`global.less`：P0 smoke、全页面基础路由、Header/Content 对齐、五档 overflow。
- DraftDetail：product-draft、publish-safety、responsive，涉及 envelope 时加 contract。
- MultiPlatformPublishCenter：publish-safety、DraftDetail publish smoke、Console guard。
- 路由、tab、section、history：navigation、deep-link、refresh restore、history。
- 表单、Modal、Drawer：对应交互 spec、取消 0 请求、确认 1 请求、375px。
- service 或 response envelope：contract tests、受影响页面 smoke。
- 纯文案：`check:ui-copy --strict` 和目标页面 smoke。
- 纯后端且不影响 Admin：不强制完整 Admin E2E，但运行对应后端测试。

## 18. 自动触发规则

AI 不得因测试较慢而跳过相关测试。未运行必要测试不得声明完成；测试阻塞时必须说明原因、阻塞命令和首个根因。

## 19. CI 规则

PR、dev push、workflow_dispatch 和定时回归应触发 Admin E2E。CI 使用真实 Node/pnpm 版本，执行 `pnpm install --frozen-lockfile`、`pnpm exec playwright install --with-deps chromium`、静态检查和 P0 E2E。CI 不连接生产数据库、真实 Redis、真实平台、真实店铺或真实 API。

## 20. 新增页面测试要求

新增 Admin 页面必须同时完成路由 smoke、auth Mock、normal/loading/empty/error/readonly、桌面和 375px、根节点 overflow、Console guard、所有写请求 Mock、取消 0 请求、单次提交、关键 payload、URL 状态刷新恢复。不得新增页面但完全不补自动化测试。

## 21. 修改页面测试要求

修改已有页面必须识别受影响 spec，优先更新已有测试，不重复新建同类文件；补充回归场景，验证旧行为、新行为、写请求无变化、响应式、Console 和无额外写请求。Bug 修复优先补一个修复前失败、修复后通过的稳定回归测试。

## 22. 请求 payload 契约测试

写请求必须断言 method、URL、path params、query、payload、请求次数和顺序。关键场景包括创建多平台草稿、创建抖店商品草稿、传统 publishProduct、库存同步、SKU 绑定、表单保存。不得修改 API/payload 来迁就错误测试。

## 23. 测试失败处理

失败时先定位首个真实根因。Mock 结构错误则修 Mock；真实生产缺陷则记录，只有属于基础设施阻塞且影响测试稳定性时才允许最小修复。不得删除、skip 或弱化失败测试来掩盖问题。

## 24. 禁止项

禁止 Cypress、Selenium、Puppeteer 测试框架、第二套浏览器测试框架、生产账号、真实店铺、真实 API Token、真实平台写接口、CI 连接生产后端、未声明写请求放行、忽略全部 console.error、忽略全部 React/AntD warning、大范围 `waitForTimeout`、坐标点击、`nth-child`、随机测试数据、测试依赖顺序、为测试大范围修改生产代码、批量添加 `data-testid`、自动更新快照掩盖回归、失败测试直接 skip、未执行测试声明完成、未经用户要求 commit/push。

## 25. 完成报告格式

最终报告至少列出：当前分支、开始工作区、审计结果、是否已有 Playwright、依赖、config 路径、E2E 目录、Skill 路径、入口更新、Cursor rule、frontend-design 引用、CLAUDE.md 状态、package scripts、Network Write Guard、Console Guard、API envelope helper、Page Object、assertions、P0 测试、覆盖页面/Tab/视口/publish/API contract、GitHub Actions、触发条件、是否访问真实后端、是否执行真实写操作、运行命令、通过/失败数量、warning 白名单、是否修改生产代码、修改原因、修改文件、diff check、check:dev、check:ui-copy、build:admin、E2E smoke/contracts、diff stat、当前未提交文件、剩余风险、后续 UI 新增/修改是否自动触发、是否适合签收。
