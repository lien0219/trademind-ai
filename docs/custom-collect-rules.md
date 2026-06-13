# 自定义链接采集规则（collect_rules）

## 面向用户的说明

- **采集规则**：告诉系统从网页哪里读取商品标题、图片、价格等内容。管理端推荐用「AI 帮我生成规则」，不必手写。
- **页面位置（selector）**：开发者术语，表示标题/价格等在页面上的位置；保存为 JSON 时字段名为 `selector` / `selectors`。
- **登录状态（浏览器 Profile）**：商品页需登录时，在采集浏览器中手动登录；系统不保存账号密码。

> **实现说明**：解析在 Node Collector（`collector/src/providers/sourceCustom`）。Go 后端只做规则 CRUD、域名匹配、`request_options` 快照与任务编排，**不写 DOM 解析**。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/collect/rules` | 列表 |
| POST | `/api/v1/collect/rules` | 新建 |
| PUT | `/api/v1/collect/rules/:id` | 更新 |
| DELETE | `/api/v1/collect/rules/:id` | 删除 |
| POST | `/api/v1/collect/rules/:id/enable` | 启用 |
| POST | `/api/v1/collect/rules/:id/disable` | 停用 |
| POST | `/api/v1/collect/rules/:id/test` | 通用访问状态 + 规则提取试跑，**不**创建 `collect_tasks` / `products` |

创建采集任务：`POST /api/v1/collect/tasks`，`source=custom`，可选 `ruleId`；未传则按 **域名 + match_pattern + priority** 自动匹配启用规则。

## 规则 JSON（≤64KB）

### 简写格式（推荐入门）

```json
{
  "title": { "selector": "h1", "type": "text" },
  "price": { "selector": ".price", "type": "text" },
  "mainImage": { "selector": ".gallery img", "type": "attr", "attr": "src" },
  "detailImages": { "selector": ".detail img", "type": "attr_all", "attr": "src" },
  "attributes": { "mode": "disabled" },
  "fallbacks": { "jsonLd": true, "openGraph": true, "meta": true }
}
```

### 京东（jd.com）主图提示

京东商品页主图常为**懒加载**，真实地址在 `data-origin` / `data-lazy-img`，不要只写 `src` 占位图。

推荐 `mainImage` 规则示例：

```json
"mainImage": {
  "selector": "#spec-img, img#spec-img, img[data-origin], img[data-lazy-img], meta[property='og:image']",
  "type": "attr",
  "attr": "src"
}
```

Collector 在规则未命中时会自动尝试：内置京东选择器、JSON-LD、OpenGraph、meta、页面最大可见 `img`（需重启 collector 使代码生效）。

### `type` 允许值

| type | 含义 |
|------|------|
| `text` | 单元素文本 |
| `text_all` | 多元素文本 |
| `attr` | 单元素属性（须 `attr`，默认 `src`） |
| `attr_all` | 多元素属性 |
| `html` | 单元素 innerHTML |
| `html_all` | 多元素 innerHTML |

### 完整格式（多选择器）

```json
{
  "title": {
    "selectors": ["h1", "[property='og:title']"],
    "attr": "text"
  },
  "mainImages": {
    "selectors": [".gallery img"],
    "attr": "src",
    "multiple": true,
    "limit": 10
  },
  "fallbacks": { "jsonLd": true, "openGraph": true, "meta": true }
}
```

保存时后端会将简写规范化为完整格式再校验。

## 域名

- 只填主机名：`1688.com`、`jd.com`，**不要**写 `https://`。
- `1688.com` 可匹配 `detail.1688.com`；`www.1688.com` **不能**匹配 `detail.1688.com`。
- `*.1688.com` 自定义采集会复用 **1688 登录 Profile**；其他站点为未登录浏览器，登录墙会导致失败。

## 通用访问状态检测（非 1688 专属）

自定义链接采集器在 Collector 内做**通用**检测（URL 路径、页面文案、HTTP 401/403、超时、标题 selector 等），**不像 1688** 那样写死平台登录流程。

`accessStatus` 枚举：

| 值 | 含义 |
|----|------|
| `public` | 页面可打开，未命中登录/风控信号 |
| `login_required` | 疑似需登录 |
| `verify_required` | 疑似验证码/风控 |
| `blocked` | 访问被拦截 |
| `timeout` | 加载超时 |
| `navigation_failed` | 导航失败 |
| `unknown` | 可打开但核心字段未提取等 |

规则测试 `POST /api/v1/collect/rules/:id/test` 返回：

- `accessStatus`、`finalUrl`、`extractedFields`、`missingFields`、`warnings`、`errorCode`、`suggestion`
- 可选 `product`（提取成功时）

`extractedFields` 示例：`{ "title": true, "price": true, "mainImage": true, "detailImagesCount": 8, "attributesCount": 12 }`

### 采集浏览器 Profile（登录态）

表 **`collect_browser_profiles`**（元数据在 PostgreSQL，Cookie 在 Collector 目录 `browser-profiles/custom/{profileKey}`）。

| API | 说明 |
|-----|------|
| `GET /api/v1/collect/browser-profiles` | 列表（可按 domain / status 筛选） |
| `POST /api/v1/collect/browser-profiles` | 新建 Profile |
| `POST …/:id/open-login` | 打开 headed 采集浏览器（body: `{ url }`） |
| `POST …/:id/check` | 用 Profile 检测 URL 访问状态 |
| `POST …/:id/disable` | 停用 Profile |
| `POST …/:id/enable` | 重新启用 Profile |
| `DELETE …/:id` | 删除 Profile 元数据（Collector 本地 Profile 目录不自动清理） |

规则测试与采集任务 body 可选：

```json
{
  "url": "https://item.jd.com/…",
  "profileId": "uuid",
  "useBrowserProfile": true
}
```

无头 Collector 无法 `open-login`，需 **`COLLECTOR_HEADLESS=0`**。

## 约束

- 不执行用户 JS；不保存完整 HTML；**不破解验证码**；**不自动绕过登录**；**不保存用户账号密码**。
- Go 后端**不解析 DOM**；前端**不直连**目标商品站。
- `custom` **batchSupported=false**，批量采集未开放。
- **1688 域名**仍走专属 `with1688Page` + 登录态检测；其他域名走通用检测。

## 常见错误码

| 码 | 说明 |
|----|------|
| `LOGIN_REQUIRED` | 疑似登录页 |
| `PAGE_BLOCKED_OR_VERIFY_REQUIRED` | 验证码/风控 |
| `CUSTOM_RULE_MISSING` | 无匹配启用规则 |
| `CUSTOM_RULE_INVALID` | 规则 JSON 无效 |
| `PARSE_FAILED_TITLE_MISSING` | 未抽到标题 |
| `PARSE_FAILED_IMAGE_MISSING` | 未抽到图片 |
| `NAVIGATION_FAILED` | 页面打不开 |
| `TIMEOUT` | 页面超时 |

## 管理端

- **采集中心** → 自定义链接采集器 → **立即采集**（Modal：URL + 规则 + **测试访问与规则**）
- **采集规则** → 新建 / **测试**（展示访问状态与字段提取摘要）
