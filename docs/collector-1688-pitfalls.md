# 1688 采集已知问题与防复发清单

> **用途**：记录 1688 Collector 已踩过的坑、修复方式与编码约束，供后续开发与 AI 编程时对照，避免同类 bug 回归。  
> **关联代码**：`collector/src/providers/source1688/`、`collector/src/browser/evaluate-in-page.ts`  
> **维护规则**：修复 1688 解析/注入/价格/SKU 类 bug 后，须在本文件追加条目，并同步 `docs/PROGRESS.md` 变更记录。

---

## 1. `page.evaluate` 脚本注入（`__name is not defined`）

### 现象

采集失败，Playwright 报错：

```text
page.evaluate: ReferenceError: __name is not defined
```

失败中心可能显示 **采集脚本执行错误**（`collector_evaluate_script`）。

### 根因

- 使用 `fn.toString()` + `eval()` 把 Node 侧函数注入页面。
- tsx / esbuild 的 `keepNames` 会在函数体插入 `__name(...)` helper。
- 浏览器页面上下文没有该 helper，执行即失败。

### 禁止做法

```typescript
// ❌ 禁止
await page.evaluate(`(${extractFn.toString()})()`);
await page.evaluate(eval(`(${fn.toString()})(arg)`));

// ❌ 禁止：函数体引用外部 helper / Node 变量 / class method
await page.evaluate(externalHelperFn, arg);
```

### 正确做法

1. **原生 `page.evaluate(fn, arg)`**，函数内所有 helper 自包含定义。
2. 复杂逻辑分层：
   - **Browser 侧**：只读 DOM + script 文本片段。
   - **Node 侧**：JSON 解析、SKU 合并、价格兜底。
3. 1688 DOM 抽取集中在 **`browser-extract-1688.ts`** 的 `extract1688DomInPage`。
4. 浏览器上下文创建时注入 **`PAGE_EVALUATE_POLYFILL`**（`session-manager.ts` / `manager.ts`）。

### 回归检查

- [ ] 全局搜索 `toString()`、`evaluateHandle` 注入页面逻辑，确认无新增。
- [ ] `pnpm build:collector` 通过。
- [ ] `pnpm collect:test -- --url "https://detail.1688.com/offer/....html"` 无 `__name` 错误。

---

## 2. 价格误取 `unitWeight`（如 39 元）

### 现象

- `raw.productPrice` 为 **39**，与页面批发价（如 ¥720 起）不符。
- 所有 SKU 统一显示 **39.00**，库存为空。
- `scriptDigest` 可见 `"unitWeight": 39.000`（商品件重尺）。

### 根因

`extractPriceFromJsonRoots` / JSON 遍历曾把 **任意 0~100 万数值** 当价格，先命中 `productPackInfo.fields.unitWeight`。

### 修复要点（`price-extract.ts` / `context-parse.ts`）

- 维护 **`NON_PRICE_KEYS`** / **`OFFER_NON_PRICE`**：`unitWeight`、`weight`、`volume`、`canBookCount`、`skuId` 等。
- 仅在 **价格相关 key**（`price`、`salePrice`、`priceDisplay`…）下读取数值。
- `extractDefaultOfferPrice` **优先** `tradeModel` / `price` / `mainPrice` 模块。
- DOM 价格取多个 `¥` 候选中的 **合理最小值**（批发起价）。

### 回归检查

- [ ] 含 `unitWeight: 39` 且无真实 price 字段时，`productPrice` 为 `undefined`，而非 39。
- [ ] 含 `tradeModel.fields.price: 720` 时返回 720。
- [ ] 工业类多 SKU 商品（如更衣柜）主价与页面「新人价 / 批发价」量级一致。

---

## 3. SKU 维度噪声（价格/库存被拼进属性值）

### 现象

`skuCandidates` 出现：

```json
{ "properties": { "颜色": "尺寸1.2mm¥790库存299件1.4mm¥860库存299件" } }
{ "properties": { "颜色": "库存299件" } }
{ "properties": { "颜色": "颜色" } }
```

管理端 SKU/库存表价格全为同一错误值，库存为 `-`。

### 根因

- DOM 选择器过宽，对 SKU 容器直接取 `textContent`，把 **尺寸 + ¥ + 库存** 整段文本当作一个规格值。
- 未过滤维度名自身（`颜色`、`尺寸`）及含 `¥` / `库存` 的字符串。
- 在无表格行时做 **笛卡尔积**，生成大量无效 SKU 并套用错误 `productPrice`。

### 修复要点

| 文件 | 要点 |
|------|------|
| `browser-extract-1688.ts` | `isJunkSkuValue` 过滤；颜色选项用 leaf 节点 + `title`/`alt`；尺寸区单独解析表格 |
| `sku-helpers.ts` | `isValidSkuDimensionValue` 供 Node 侧二次过滤 |
| `parser.ts` | `skusFromDomPayload` 先过滤再组合；有 **表格行 + 颜色维** 时生成 **颜色×尺寸** 矩阵并带价/库存 |

### 回归检查

- [ ] 属性值中不含 `¥`、`库存\d+`、与维度名相同的字符串。
- [ ] 多颜色 × 多尺寸（mm 厚度表）商品：`domSkuTableRowCount > 0`，SKU 带独立 price/stock。
- [ ] `mergeSkuLists` 优先保留 **带真实价格** 的列表。

---

## 4. 尺寸价格表只认 `内长\d+`，漏掉 `1.2mm` 类规格

### 现象

页面有「1.2mm ¥790 库存299件 / 1.4mm ¥860 库存299件」，但 `domSkuTableRowCount: 0`。

### 根因

`pushTableRow` 仅匹配 `/内长\d+/`，工业类商品常用 **厚度 mm** 而非「内长」。

### 修复要点

- 表格行正则扩展：`[\d.]+\s*mm`、`厚度[\d.]+\s*mm`。
- 扫描 `1.2mm ¥790 ... 库存299` 紧凑格式。
- `enrichSkusFromDomTable` 支持 mm 标签模糊匹配。

### 回归检查

- [ ] 测试链接：`https://detail.1688.com/offer/1048021652334.html`（不锈钢更衣柜，多颜色 + mm 价表）。
- [ ] `raw.domSkuTableRowCount >= 2`，SKU 价格约 790/860 而非 39。

---

## 5. 1688 登录态与风控（非解析 bug，但易混淆）

### 现象

- 后台显示已登录，采集仍失败或落到首页/验证码页。
- `finalUrl` 非 `/offer/...`，标题为「1688首页」。

### 要点

- Profile 路径统一：`collector/data/browser-profiles/1688`（见 `browser-paths.ts`）。
- 登录检测 **已登录信号优先**（`auth-detect.ts`）。
- 失败分类：`collector_platform_login` vs `collector_evaluate_script` vs `collector_missing_price`。

---

## 6. 推荐本地验证命令

```bash
# 通用管道疏通类（context + skuMap）
pnpm collect:test -- --url "https://detail.1688.com/offer/819929707153.html"

# 多颜色 + mm 价表工业类
pnpm collect:test -- --url "https://detail.1688.com/offer/1048021652334.html"
```

检查 `raw.extractDebug.productPrice`、`raw.domSkuTableRowCount`、`skus[].price` / `skus[].stock`。

---

## 7. 变更索引（按日期）

| 日期 | Bug / 主题 | 关键文件 |
|------|------------|----------|
| 2026-05-20 | `__name is not defined` | `evaluate-in-page.ts`、`browser-extract-1688.ts`、`session-manager.ts` |
| 2026-05-20 | 价格误取 `unitWeight` | `price-extract.ts`、`context-parse.ts` |
| 2026-05-20 | SKU 维度噪声 + mm 价表未解析 | `browser-extract-1688.ts`、`sku-helpers.ts`、`parser.ts` |
| 2026-05-20 | 失败分类「采集脚本执行错误」 | `collect-task.ts`、`failureclassifier.go`、`taskCenter.ts` |

---

## 8. AI / 开发者修改 1688 解析前必读

1. 先读 **`docs/collector-1688-pitfalls.md`**（本文件）与 **`docs/module-map.md`** 采集关联项。
2. **禁止**恢复 `toString` 注入；**禁止**在 `page.evaluate` 中引用 Node 模块或外部 closure。
3. 改价格逻辑时确认 **不会**把 weight/stock/id 当 price。
4. 改 SKU DOM 逻辑时用 **§3、§4** 两类商品各测一条。
5. 收尾：`pnpm build:collector` + 更新 **`docs/PROGRESS.md`** 变更记录。
