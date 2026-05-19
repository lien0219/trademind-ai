/**
 * 1688 详情页多套皮肤并存，同一字段多组兜底选择器。
 * 皆为「尽量命中」，未命中则由 script JSON 与 og:image 补全。
 */
export const TITLE_SELECTORS = [
  'h1.d-title',
  '.offer-title .title-text',
  '.title-content h1',
  '[class*="offer-title"] h1',
  'h1[class*="title"]',
];

/** 主图预览区（非详情正文）；避免过宽的 [class*="gallery"] 以免扫到服务图标 */
export const MAIN_GALLERY_SELECTORS = [
  '.vertical-img img',
  '.dot-img-footer-list img',
  '.tab-content-wrapper img',
  '.detail-gallery-preview img',
  '.detail-gallery-turn img',
  '.detail-gallery img',
  '[class*="offer-gallery"] img',
  '[class*="main-image"] img',
  '.swiper-slide img',
  '.obj-sku-img-item img',
  '.obj-header-image img',
];

/** 详情描述区（常为富文本容器） */
export const DETAIL_SELECTORS = [
  '#offer-template-0 img',
  '.offer-description img',
  '.offer-detail img',
  '.detail-desc-module img',
  '[class*="detail-description"] img',
  '[class*="offerDesc"] img',
  '[module-title="商品详情"] img',
  '.wireless-description img',
];

/** 参数 / 属性表 */
export const ATTRIBUTE_ROW_SELECTORS = [
  '.offer-attrprogram .de-feature-item',
  '.offer-attr-item',
  '[class*="param-table"] tr',
  '.obj-content-table tr',
  '.offer-params tr',
  '#productAttributes .obj-content-table tr',
  '[module-title="商品属性"] tr',
];

/** SKU 规格区（颜色/尺码等） */
export const SKU_SECTION_SELECTORS = [
  '[class*="sku-item-wrapper"]',
  '[class*="sku-selector"]',
  '[class*="obj-sku"]',
  '[class*="sale-prop"]',
  '[class*="spec-item"]',
  '[class*="prop-item"]',
  '.module-od-sku-selection',
];

/** SKU 尺码/库存表格行 */
export const SKU_TABLE_ROW_SELECTORS = [
  '[class*="sku-table"] tr',
  '[class*="sku-item-list"] [class*="item"]',
  '[class*="table-sku"] tr',
  '.obj-sku .table tr',
];
