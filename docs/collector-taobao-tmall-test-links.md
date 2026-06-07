# 淘宝/天猫采集器测试链接验收表

> 状态：**测试中（Beta）**。请在独立 `taobao_tmall` 登录浏览器 Profile 下逐条验收，填写实测结果。

## 验收说明

| 字段 | 说明 |
| --- | --- |
| 需要登录 | 未登录时是否出现 `LOGIN_REQUIRED` |
| 标题 | 是否采到非空标题 |
| 价格 | 是否识别价格（缺失应有 `PRICE_NOT_FOUND` warning） |
| 主图数 | 主图数量（0 则任务应失败 `MAIN_IMAGES_EMPTY`） |
| 详情图数 | 详情图数量（0 可有 `DETAIL_IMAGES_INCOMPLETE` warning） |
| SKU 数 | 规格行数（不完整可有 `SKU_INCOMPLETE` warning） |
| warning | 采集 warning 码 |
| error | 失败 error 码 |
| 草稿 | 是否成功创建商品草稿 |

## 普通淘宝商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 2 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 3 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 4 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 5 | `https://item.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 普通天猫商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 6 | `https://detail.tmall.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 7 | `https://detail.tmall.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 8 | `https://detail.tmall.hk/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 9 | `https://chaoshi.tmall.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 10 | `https://ju.taobao.com/item.htm?id=【填写】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 有 SKU 商品（5 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 11 | `https://item.taobao.com/item.htm?id=【多规格淘宝】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 12 | `https://detail.tmall.com/item.htm?id=【多规格天猫】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 13 | `https://item.taobao.com/item.htm?id=【颜色+尺码】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 14 | `https://detail.tmall.com/item.htm?id=【颜色+尺码】` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |
| 15 | `https://world.taobao.com/item/【填写】.htm` | 待测 | 待测 | 待测 | 待测 | 待测 | 待测 | — | — | 待测 |

## 多主图商品（2 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 16 | `https://item.taobao.com/item.htm?id=【多主图】` | 待测 | 待测 | 待测 | ≥2 | 待测 | 待测 | — | — | 待测 |
| 17 | `https://detail.tmall.com/item.htm?id=【多主图】` | 待测 | 待测 | 待测 | ≥2 | 待测 | 待测 | — | — | 待测 |

## 多详情图商品（2 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 18 | `https://item.taobao.com/item.htm?id=【长详情】` | 待测 | 待测 | 待测 | 待测 | ≥3 | 待测 | — | — | 待测 |
| 19 | `https://detail.tmall.com/item.htm?id=【长详情】` | 待测 | 待测 | 待测 | 待测 | ≥3 | 待测 | — | — | 待测 |

## 已下架 / 异常商品（1 条）

| # | 链接 | 需要登录 | 标题 | 价格 | 主图数 | 详情图数 | SKU 数 | warning | error | 草稿 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 20 | `https://item.taobao.com/item.htm?id=【已下架】` | — | — | — | 0 | 0 | 0 | — | `ITEM_NOT_FOUND` | 否 |

## 不支持链接（回归）

| 场景 | 链接示例 | 期望 error |
| --- | --- | --- |
| 淘宝首页 | `https://www.taobao.com/` | `UNSUPPORTED_TAOBAO_URL` |
| 店铺页 | `https://shop.taobao.com/shop/view_shop.htm?shop_id=xxx` | `UNSUPPORTED_TAOBAO_URL` |
| 搜索页 | `https://s.taobao.com/search?q=xxx` | `UNSUPPORTED_TAOBAO_URL` |

## 升级「已可用」门槛

满足以下全部条件后，可将采集中心状态从 **测试中** 改为 **已可用**：

1. 上表 20 条真实链接验收完成，成功率稳定（建议 ≥80% 普通商品可创建草稿）。
2. 登录 / 验证 / 下架三类异常提示准确可读。
3. 主图为空必失败、价格缺失 warning + 发布前拦截生效。
4. 失败任务中心可重试、可打开采集浏览器。
5. 无绕过验证码相关逻辑。
