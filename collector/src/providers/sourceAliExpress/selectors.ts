/** AliExpress PDP — DOM hints（站点改版时需调整）；主解析仍偏向 script JSON */

export const AE_TITLE_SELECTORS = [
  'h1[data-pl="product-title"]',
  'h1[class*="product-title"]',
  'h1[class*="wrap-product-title"]',
  '[data-pl="product-title"]',
  '.product-title-text',
  'h1',
];

/** 列表 / 放大镜区 */
export const AE_MAIN_GALLERY_SELECTORS = [
  '[slider-product-image] img',
  '[class*="gallery"] img',
  '[class*="images-view"] img',
  '[class*="image-view"] img',
  '[itemprop="image"]',
  '[class*="slider--pagination"] img',
  '[class*="slider-pagination"] img',
  '[class*="magnifier--image-wrap"] img',
  '.images-view-item img',
  'div[class*="Sku"] img[class*="Sku"]',
];

export const AE_DETAIL_AREA_SELECTORS = [
  '[id="product-description"] img',
  '[class*="detail-desc"] img',
  '[class*="detailcontent"] img',
  '[class*="detail-desc-modules"] img',
  '[module-id="detailProductDesc"] img',
];

/** dd/dt rows、属性表 */
export const AE_ATTRIBUTE_CONTAINER_SELECTORS = [
  '[class*="specification--line"]',
  '[class*="specifications--spec"]',
  '[class*="property-item"]',
  '[class*="ProductSpecification"] dd',
];
